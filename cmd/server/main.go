package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"mikrotik-vpn-manager/internal/app"
	"mikrotik-vpn-manager/internal/config"
	httpserver "mikrotik-vpn-manager/internal/http"
	"net"
	"net/url"

	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var version = "1.1.0-dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if e := run(logger); e != nil {
		logger.Error("application stopped", "error", e.Error())
		os.Exit(1)
	}
}
func run(logger *slog.Logger) error {
	c, e := config.Load()
	if e != nil {
		return e
	}
	c.Version = version
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		origin, e := url.Parse(c.Origin)
		if e != nil {
			return e
		}
		host, port, e := net.SplitHostPort(c.Address)
		if e != nil {
			return e
		}
		scheme := "http"
		if c.Cert != "" {
			scheme = "https"
		}
		var roots *x509.CertPool
		if c.TLSCAFile != "" {
			roots, _ = x509.SystemCertPool()
			if roots == nil {
				roots = x509.NewCertPool()
			}
			pem, err := os.ReadFile(c.TLSCAFile)
			if err != nil {
				return err
			}
			if !roots.AppendCertsFromPEM(pem) {
				return errors.New("invalid panel TLS CA file")
			}
		}
		client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: origin.Hostname(), RootCAs: roots}}}
		if host == "" || host == "0.0.0.0" {
			host = "127.0.0.1"
		} else if host == "::" {
			host = "::1"
		}
		resp, e := client.Get(scheme + "://" + net.JoinHostPort(host, port) + "/readyz")
		if e != nil {
			return e
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return errors.New("unhealthy")
		}
		return nil
	}
	if len(os.Args) > 1 && os.Args[1] == "restore" {
		if len(os.Args) != 3 {
			return errors.New("usage: server restore <validated-backup-path>; destination must not exist")
		}
		return restoreRepository(c.Database, os.Args[2], c.Master)
	}
	repo, e := openRepository(c.Database, c.Master)
	if e != nil {
		return e
	}
	defer repo.Close()
	service := app.New(repo, c)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bootstrap":
			if e = service.Bootstrap(os.Getenv("MVM_ADMIN_USERNAME"), os.Getenv("MVM_ADMIN_PASSWORD")); e != nil {
				return e
			}
			logger.Info("initial administrator created")
			return nil
		case "check":
			return repo.Check()
		case "backup":
			if len(os.Args) != 3 {
				return errors.New("usage: server backup <new-backup-path>")
			}
			f, e := os.OpenFile(os.Args[2], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if e != nil {
				return e
			}
			e = repo.Backup(f)
			if e == nil {
				e = f.Sync()
			}
			closeErr := f.Close()
			return errors.Join(e, closeErr)
		default:
			return errors.New("unknown command")
		}
	}
	if e = service.Engine.Recover(); e != nil {
		return e
	}
	web, e := httpserver.New(service, logger)
	if e != nil {
		return e
	}
	server := &http.Server{Addr: c.Address, Handler: web.Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 5 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 * 1024}
	stop, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()
	go service.Maintenance(stop)
	errs := make(chan error, 1)
	go func() {
		logger.Info("listening", "address", c.Address, "version", version, "router_writes", c.Mutations)
		if c.Cert != "" && c.Key != "" {
			errs <- server.ListenAndServeTLS(c.Cert, c.Key)
		} else {
			if !c.Dev && !c.ProxyTLS && !c.AllowHTTP {
				errs <- errors.New("configure native TLS, development loopback HTTP, or documented container reverse proxy")
				return
			}
			errs <- server.ListenAndServe()
		}
	}()
	select {
	case e := <-errs:
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	case <-stop.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if e = server.Shutdown(ctx); e != nil {
			return e
		}
		logger.Info("graceful shutdown complete")
		return nil
	}
}
