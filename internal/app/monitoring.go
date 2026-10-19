package app

import (
	"context"
	"errors"
	"math"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"mikrotik-vpn-manager/internal/storage"
	"strconv"
	"strings"
	"time"
)

type MonitorSample struct {
	At        time.Time `json:"at"`
	Online    bool      `json:"online"`
	CPU       *float64  `json:"cpu"`
	Memory    *float64  `json:"memory"`
	Disk      *float64  `json:"disk"`
	RX        *float64  `json:"rx_bytes"`
	TX        *float64  `json:"tx_bytes"`
	PPP       *float64  `json:"ppp"`
	OpenVPN   *float64  `json:"openvpn"`
	SSTP      *float64  `json:"sstp"`
	L2TP      *float64  `json:"l2tp"`
	WireGuard *float64  `json:"wireguard"`
	IPsec     *float64  `json:"ipsec"`
	Warnings  []string  `json:"warnings"`
}
type MonitorHistory struct {
	RouterID string          `json:"router_id"`
	Samples  []MonitorSample `json:"samples"`
}
type MonitorRouter struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Status  string          `json:"status"`
	CanSync bool            `json:"can_sync"`
	History []MonitorSample `json:"history"`
}

func number(s string) *float64 {
	v, e := strconv.ParseFloat(s, 64)
	if e != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}
func percent(free, total string) *float64 {
	f, t := number(free), number(total)
	if f == nil || t == nil || *t == 0 || *f > *t {
		return nil
	}
	v := 100 * (1 - *f / *t)
	return &v
}
func sampleFromRouter(r domain.Router) MonitorSample {
	resources := r.Observed.Resources
	return MonitorSample{At: r.LastSuccess, Online: r.Status == "online" || r.Status == "degraded", CPU: number(resources["cpu-load"]), Memory: percent(resources["free-memory"], resources["total-memory"]), Disk: percent(resources["free-hdd-space"], resources["total-hdd-space"])}
}
func (s *Service) recordMonitor(ctx context.Context, c routers.API, r domain.Router) error {
	sample := sampleFromRouter(r)
	sample.At = time.Now().UTC()
	if !sample.Online {
		sample.CPU = nil
		sample.Memory = nil
		sample.Disk = nil
	}
	if sample.Online && c != nil {
		for _, metric := range []struct {
			path   string
			target **float64
			wg     bool
		}{{"ppp/active", &sample.PPP, false}, {"interface/wireguard/peers", &sample.WireGuard, true}, {"ip/ipsec/active-peers", &sample.IPsec, false}} {
			rows, e := routers.List(ctx, c, metric.path)
			if e != nil {
				sample.Warnings = append(sample.Warnings, metric.path)
				continue
			}
			count := float64(len(rows))
			if metric.path == "ppp/active" {
				ovpn, sstp, l2tp := 0.0, 0.0, 0.0
				for _, row := range rows {
					switch row["service"] {
					case "ovpn":
						ovpn++
					case "sstp":
						sstp++
					case "l2tp":
						l2tp++
					}
				}
				sample.OpenVPN = &ovpn
				sample.SSTP = &sstp
				sample.L2TP = &l2tp
			}
			if metric.wg {
				count = 0
				for _, row := range rows {
					if row["disabled"] == "yes" || row["disabled"] == "true" {
						continue
					}
					age, e := parseRouterDuration(row["last-handshake"])
					if e == nil && age >= 0 && age < 3*time.Minute && (age > 0 || row["rx"] != "0" && row["rx"] != "" || row["tx"] != "0" && row["tx"] != "") {
						count++
					}
				}
			}
			*metric.target = &count
		}
		rows, e := c.Call(ctx, "POST", "interface/print", routers.Record{"stats": "", ".proplist": "name,type,rx-byte,tx-byte"})
		if e != nil {
			sample.Warnings = append(sample.Warnings, "interface")
		} else {
			rx, tx := 0.0, 0.0
			found := false
			for _, row := range rows {
				if row["type"] != "ether" && row["type"] != "wlan" && row["type"] != "wifi" {
					continue
				}
				a, b := number(row["rx-byte"]), number(row["tx-byte"])
				if a != nil && b != nil {
					rx += *a
					tx += *b
					found = true
				}
			}
			if found {
				sample.RX = &rx
				sample.TX = &tx
			}
		}
	}
	var history MonitorHistory
	if e := s.Repo.Get("monitoring", r.ID, &history); e != nil {
		if !errors.Is(e, storage.ErrNotFound) {
			return e
		}
		history = MonitorHistory{RouterID: r.ID}
	}
	history.Samples = append(history.Samples, sample)
	if len(history.Samples) > 120 {
		history.Samples = history.Samples[len(history.Samples)-120:]
	}
	return s.Repo.Put("monitoring", r.ID, history)
}
func parseRouterDuration(raw string) (time.Duration, error) {
	if strings.Contains(raw, ":") {
		parts := strings.Split(raw, ":")
		if len(parts) == 3 {
			raw = parts[0] + "h" + parts[1] + "m" + parts[2] + "s"
		}
	}
	if strings.Contains(raw, "w") {
		parts := strings.SplitN(raw, "w", 2)
		v, e := strconv.Atoi(parts[0])
		if e != nil {
			return 0, e
		}
		d, e := parseRouterDuration(parts[1])
		return time.Duration(v)*7*24*time.Hour + d, e
	}
	if strings.Contains(raw, "d") {
		parts := strings.SplitN(raw, "d", 2)
		v, e := strconv.Atoi(parts[0])
		if e != nil {
			return 0, e
		}
		if parts[1] == "" {
			return time.Duration(v) * 24 * time.Hour, nil
		}
		d, e := parseRouterDuration(parts[1])
		return time.Duration(v)*24*time.Hour + d, e
	}
	return time.ParseDuration(raw)
}
func (s *Service) Monitoring(a domain.Account) ([]MonitorRouter, error) {
	var all []domain.Router
	if e := s.Repo.List("routers", &all); e != nil {
		return nil, e
	}
	result := []MonitorRouter{}
	for _, r := range all {
		if a.Role == domain.EndUser || !a.Can("read", r.ID) {
			continue
		}
		var history MonitorHistory
		if e := s.Repo.Get("monitoring", r.ID, &history); e != nil {
			if !errors.Is(e, storage.ErrNotFound) {
				return nil, e
			}
			if !r.LastSuccess.IsZero() {
				history.Samples = []MonitorSample{sampleFromRouter(r)}
			}
		}
		status := r.Status
		if (status == "online" || status == "degraded") && time.Since(r.LastSuccess) > 2*time.Minute {
			status = "stale"
		}
		result = append(result, MonitorRouter{ID: r.ID, Name: r.Name, Status: status, CanSync: a.Can("router", r.ID), History: history.Samples})
	}
	return result, nil
}
