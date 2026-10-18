package httpserver

import (
	"crypto/subtle"
	"errors"
	"github.com/gin-gonic/gin"
	"mikrotik-vpn-manager/internal/domain"
	"os"
	"strings"
	"time"
	_ "time/tzdata"
)

func (h *Server) setup(c *gin.Context) {
	var accounts []domain.Account
	if e := h.App.Repo.List("accounts", &accounts); e != nil {
		h.fail(c, 503, e)
		return
	}
	if len(accounts) > 0 {
		c.AbortWithStatus(404)
		return
	}
	tokenPath := os.Getenv("MVM_BOOTSTRAP_TOKEN_FILE")
	raw, e := os.ReadFile(tokenPath)
	if e != nil || len(strings.TrimSpace(string(raw))) < 32 {
		c.AbortWithStatus(404)
		return
	}
	if c.Request.Method == "GET" {
		lang := c.DefaultQuery("lang", "en")
		if lang != "fa" {
			lang = "en"
		}
		dir := "ltr"
		if lang == "fa" {
			dir = "rtl"
		}
		if e = h.templates.ExecuteTemplate(c.Writer, "setup", gin.H{"Lang": lang, "Dir": dir, "Origin": h.App.Config.Origin, "TLS": h.App.Config.Cert != "" || h.App.Config.ProxyTLS}); e != nil {
			h.Log.Error("setup render failed")
		}
		return
	}
	if !h.allowedOrigin(c.GetHeader("Origin")) {
		h.fail(c, 403, errors.New("invalid setup origin"))
		return
	}
	if h.throttled("setup:" + c.ClientIP()) {
		c.AbortWithStatus(429)
		return
	}
	if subtle.ConstantTimeCompare([]byte(c.PostForm("token")), []byte(strings.TrimSpace(string(raw)))) != 1 {
		c.AbortWithStatus(403)
		return
	}
	select {
	case h.authGate <- struct{}{}:
		defer func() { <-h.authGate }()
	default:
		c.AbortWithStatus(429)
		return
	}
	zone := c.PostForm("timezone")
	if _, e = time.LoadLocation(zone); e != nil {
		h.fail(c, 400, errors.New("invalid timezone"))
		return
	}
	lang := c.PostForm("language")
	if lang != "en" && lang != "fa" {
		c.AbortWithStatus(400)
		return
	}
	if e = h.App.Bootstrap(c.PostForm("username"), c.PostForm("password")); e != nil {
		h.fail(c, 400, e)
		return
	}
	// Initialization itself closes this endpoint even if optional preferences cannot persist.
	if e = h.App.Repo.Put("settings", "panel", map[string]string{"timezone": zone, "language": lang, "origin": h.App.Config.Origin}); e != nil {
		h.fail(c, 500, e)
		return
	}
	c.Redirect(303, "/?lang="+lang)
}
