package httpserver

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"mikrotik-vpn-manager/internal/app"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/provisioning"
	"mikrotik-vpn-manager/internal/routers"
	"mikrotik-vpn-manager/internal/security"
	assets "mikrotik-vpn-manager/web"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type attempt struct {
	Count int
	Until time.Time
}
type Server struct {
	App       *app.Service
	Log       *slog.Logger
	templates *template.Template
	locales   map[string]map[string]string
	mu        sync.Mutex
	attempts  map[string]attempt
	authGate  chan struct{}
}

func New(s *app.Service, logger *slog.Logger) (*Server, error) {
	h := &Server{App: s, Log: logger, attempts: map[string]attempt{}, locales: map[string]map[string]string{}, authGate: make(chan struct{}, 2)}
	for _, lang := range []string{"en", "fa"} {
		b, e := assets.Files.ReadFile("locales/" + lang + ".json")
		if e != nil {
			return nil, e
		}
		var d map[string]string
		if e = json.Unmarshal(b, &d); e != nil {
			return nil, e
		}
		h.locales[lang] = d
	}
	t, e := template.New("").Funcs(template.FuncMap{"t": func(lang, key string) string {
		if v := h.locales[lang][key]; v != "" {
			return v
		}
		if v := h.locales["en"][key]; v != "" {
			return v
		}
		return key
	}, "json": func(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }, "timefmt": func(t time.Time) string {
		if t.IsZero() {
			return "—"
		}
		return t.UTC().Format("2006-01-02 15:04 UTC")
	}, "join": strings.Join}).ParseFS(assets.Files, "templates/*.html")
	if e != nil {
		return nil, e
	}
	h.templates = t
	return h, nil
}
func (h *Server) Routes() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	_ = r.SetTrustedProxies(h.App.Config.TrustedProxies)
	r.Use(func(c *gin.Context) {
		defer func() {
			if recover() != nil {
				h.Log.Error("request panic suppressed")
				c.AbortWithStatusJSON(500, gin.H{"error": gin.H{"message": "internal error"}})
			}
		}()
		c.Next()
	}, h.headers)
	static, _ := fs.Sub(assets.Files, "static")
	r.StaticFS("/static", http.FS(static))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) {
		var v string
		if e := h.App.Repo.Get("settings", "key_check", &v); e != nil {
			c.JSON(503, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})
	r.GET("/setup", h.setup)
	r.POST("/setup", h.setup)
	r.GET("/", h.page)
	r.POST("/login", h.login)
	api := r.Group("/api/v1", h.auth, h.csrf)
	api.POST("/logout", h.logout)
	api.GET("/routers", h.routerList)
	api.POST("/routers", h.addRouter)
	api.POST("/routers/:id/sync", h.syncRouter)
	api.DELETE("/routers/:id", h.removeRouter)
	api.GET("/routers/:id/capabilities", h.capabilities)
	api.POST("/provisioning/servers/preview", h.prepareServer)
	api.POST("/provisioning/users/preview", h.prepareUser)
	api.POST("/provisioning/jobs/:id/apply", h.apply)
	api.GET("/provisioning/jobs/:id", h.job)
	api.POST("/vpn/users/:id/:action", h.userAction)
	api.POST("/client-configs/:id/export", h.export)
	api.POST("/client-configs/download", h.download)
	api.POST("/client-configs/revoke", h.revokeDelivery)
	api.POST("/accounts", h.addAccount)
	api.GET("/audit", h.audit)
	api.GET("/routers/:id/sessions", h.sessions)
	r.GET("/metrics", h.auth, func(c *gin.Context) {
		a := account(c)
		if a.Role != domain.Super && a.Role != domain.Auditor {
			h.fail(c, 403, errors.New("permission denied"))
			return
		}
		var all []domain.Router
		if e := h.App.Repo.List("routers", &all); e != nil {
			h.fail(c, 500, e)
			return
		}
		n := 0
		for _, r := range all {
			if a.Can("read", r.ID) {
				n++
			}
		}
		c.Data(200, "text/plain; version=0.0.4", []byte(fmt.Sprintf("# HELP mvm_registered_routers Inventory size visible to current role.\n# TYPE mvm_registered_routers gauge\nmvm_registered_routers %d\n", n)))
	})
	return r
}
func (h *Server) headers(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128*1024)
	c.Header("X-Request-ID", app.ID())
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "same-origin")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'; object-src 'none'")
	if !h.App.Config.Dev {
		c.Header("Strict-Transport-Security", "max-age=31536000")
	}
	start := time.Now()
	c.Next()
	h.Log.Info("http", "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
}
func account(c *gin.Context) domain.Account {
	v, _ := c.Get("account")
	a, _ := v.(domain.Account)
	return a
}
func (h *Server) session(c *gin.Context) (domain.Session, domain.Account, error) {
	token, e := c.Cookie("mvm_session")
	if e != nil {
		return domain.Session{}, domain.Account{}, e
	}
	var s domain.Session
	if e = h.App.Repo.Get("sessions", security.Digest(token), &s); e != nil {
		return s, domain.Account{}, e
	}
	if time.Now().After(s.Expires) {
		_ = h.App.Repo.Delete("sessions", security.Digest(token))
		return s, domain.Account{}, errors.New("session expired")
	}
	var a domain.Account
	e = h.App.Repo.Get("accounts", s.AccountID, &a)
	return s, a, e
}
func (h *Server) auth(c *gin.Context) {
	s, a, e := h.session(c)
	if e != nil {
		h.fail(c, 401, errors.New("sign in required"))
		c.Abort()
		return
	}
	c.Set("account", a)
	c.Set("session", s)
	c.Next()
}
func (h *Server) csrf(c *gin.Context) {
	if c.Request.Method == "GET" || c.Request.Method == "HEAD" {
		c.Next()
		return
	}
	s := c.MustGet("session").(domain.Session)
	provided := c.GetHeader("X-CSRF-Token")
	if provided == "" {
		provided = c.PostForm("csrf")
	}
	if c.GetHeader("Origin") != h.App.Config.Origin || subtle.ConstantTimeCompare([]byte(provided), []byte(s.CSRF)) != 1 {
		h.fail(c, 403, errors.New("CSRF validation failed"))
		c.Abort()
		return
	}
	c.Next()
}
func (h *Server) fail(c *gin.Context, status int, e error) {
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") || c.GetHeader("Accept") == "application/json" {
		c.JSON(status, gin.H{"error": gin.H{"message": e.Error(), "request_id": c.Writer.Header().Get("X-Request-ID")}})
		return
	}
	h.render(c, status, gin.H{"Page": "error", "Error": e.Error(), "Account": account(c)})
}
func (h *Server) render(c *gin.Context, status int, data gin.H) {
	lang := "en"
	if cookie, e := c.Cookie("mvm_language"); e == nil && cookie == "fa" {
		lang = "fa"
	}
	if q := c.Query("lang"); q == "en" || q == "fa" {
		lang = q
		c.SetCookie("mvm_language", q, 31536000, "/", "", !h.App.Config.Dev, true)
	}
	data["Lang"] = lang
	data["Dir"] = "ltr"
	if lang == "fa" {
		data["Dir"] = "rtl"
	}
	data["Mutations"] = h.App.Config.Mutations
	data["Version"] = h.App.Config.Version
	if v, ok := c.Get("session"); ok {
		data["CSRF"] = v.(domain.Session).CSRF
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	if e := h.templates.ExecuteTemplate(c.Writer, "layout", data); e != nil {
		h.Log.Error("template execution failed", "page", data["Page"])
	}
}
func (h *Server) success(c *gin.Context, data any, page string) {
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		c.JSON(200, data)
		return
	}
	c.Redirect(303, page)
}
func (h *Server) throttled(key string) bool {
	return h.throttledLimit(key, 5)
}
func (h *Server) throttledLimit(key string, limit int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for k, v := range h.attempts {
		if now.After(v.Until) {
			delete(h.attempts, k)
		}
	}
	v := h.attempts[key]
	if v.Count >= limit && now.Before(v.Until) {
		return true
	}
	if len(h.attempts) > 10000 {
		return true
	}
	v.Count++
	v.Until = now.Add(10 * time.Minute)
	h.attempts[key] = v
	return false
}
func (h *Server) login(c *gin.Context) {
	if c.GetHeader("Origin") != h.App.Config.Origin {
		h.Log.Warn("login origin rejected", "origin", c.GetHeader("Origin"), "expected_origin", h.App.Config.Origin, "fetch_site", c.GetHeader("Sec-Fetch-Site"))
		h.fail(c, 403, errors.New("invalid login origin"))
		return
	}
	if h.throttledLimit("ip:"+c.ClientIP(), 50) {
		h.fail(c, 429, errors.New("too many sign-in attempts; wait ten minutes"))
		return
	}
	select {
	case h.authGate <- struct{}{}:
		defer func() { <-h.authGate }()
	default:
		h.fail(c, 429, errors.New("authentication capacity reached; retry later"))
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		if e := decode(c, &input); e != nil {
			h.fail(c, 400, e)
			return
		}
	} else {
		input.Username = c.PostForm("username")
		input.Password = c.PostForm("password")
	}
	accountKey := "account:" + security.Digest(strings.ToLower(input.Username))
	if h.throttled(accountKey) {
		h.fail(c, 429, errors.New("too many sign-in attempts; wait ten minutes"))
		return
	}
	var accounts []domain.Account
	if e := h.App.Repo.List("accounts", &accounts); e != nil {
		h.fail(c, 500, e)
		return
	}
	var found domain.Account
	for _, a := range accounts {
		if a.Username == input.Username {
			found = a
		}
	}
	hash := found.Hash
	if hash == "" {
		hash = "argon2id$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if !security.CheckPassword(hash, input.Password) || found.ID == "" {
		_ = h.App.Audit("anonymous", "auth.login", "", "denied")
		h.fail(c, 401, errors.New("invalid username or password"))
		return
	}
	token := security.Token()
	s := domain.Session{AccountID: found.ID, CSRF: security.Token(), Expires: time.Now().Add(8 * time.Hour)}
	if e := h.App.Repo.Put("sessions", security.Digest(token), s); e != nil {
		h.fail(c, 500, e)
		return
	}
	h.mu.Lock()
	delete(h.attempts, "ip:"+c.ClientIP())
	delete(h.attempts, accountKey)
	h.mu.Unlock()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("mvm_session", token, 8*3600, "/", "", !h.App.Config.Dev, true)
	_ = h.App.Audit(found.ID, "auth.login", "", "success")
	next := "/"
	if c.PostForm("next") == "delivery" {
		next = "/?page=delivery"
	}
	h.success(c, gin.H{"csrf": s.CSRF}, next)
}
func (h *Server) logout(c *gin.Context) {
	token, _ := c.Cookie("mvm_session")
	if e := h.App.Repo.Delete("sessions", security.Digest(token)); e != nil {
		h.fail(c, 500, e)
		return
	}
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("mvm_session", "", -1, "/", "", !h.App.Config.Dev, true)
	h.success(c, gin.H{"ok": true}, "/")
}
func decode(c *gin.Context, out any) error {
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return errors.New("invalid request JSON")
	}
	var extra any
	if e := d.Decode(&extra); !errors.Is(e, io.EOF) {
		return errors.New("only one JSON object is allowed")
	}
	return nil
}
func (h *Server) page(c *gin.Context) {
	s, a, e := h.session(c)
	if e != nil {
		var accounts []domain.Account
		_ = h.App.Repo.List("accounts", &accounts)
		h.render(c, 200, gin.H{"Page": "login", "Uninitialized": len(accounts) == 0})
		return
	}
	c.Set("session", s)
	c.Set("account", a)
	page := c.DefaultQuery("page", "dashboard")
	if a.Role == domain.EndUser && page != "delivery" {
		page = "portal"
	}
	data := gin.H{"Page": page, "Account": a}
	var allRouters []domain.Router
	var servers []domain.Server
	var users []domain.VPNUser
	var allJobs []domain.Job
	for _, read := range []struct {
		bucket string
		out    any
	}{{"routers", &allRouters}, {"servers", &servers}, {"users", &users}, {"jobs", &allJobs}} {
		if e = h.App.Repo.List(read.bucket, read.out); e != nil {
			h.fail(c, 500, e)
			return
		}
	}
	visibleRouters := []domain.Router{}
	for _, r := range allRouters {
		if a.Role != domain.EndUser && a.Can("read", r.ID) {
			r.Password = ""
			r.CA = ""
			if time.Since(r.LastSuccess) > 2*time.Minute && r.Status != "unknown" {
				r.Status = "stale"
			}
			visibleRouters = append(visibleRouters, r)
		}
	}
	visibleServers := []domain.Server{}
	for _, v := range servers {
		if a.Role != domain.EndUser && a.Can("read", v.Spec.RouterID) {
			visibleServers = append(visibleServers, v)
		}
	}
	visibleUsers := []domain.VPNUser{}
	for _, v := range users {
		if a.Role == domain.EndUser && v.AccountID == a.ID || a.Role != domain.EndUser && a.Can("read", v.RouterID) {
			visibleUsers = append(visibleUsers, v)
		}
	}
	visibleJobs := []domain.Job{}
	for _, j := range allJobs {
		if a.Role != domain.EndUser && a.Can("read", j.RouterID) {
			visibleJobs = append(visibleJobs, redactJob(j))
		}
	}
	sort.Slice(visibleJobs, func(i, j int) bool { return visibleJobs[i].Created.After(visibleJobs[j].Created) })
	data["Routers"] = visibleRouters
	data["Servers"] = visibleServers
	data["Users"] = visibleUsers
	data["Jobs"] = visibleJobs
	data["RequestKey"] = app.ID()
	data["SelectedRouterID"] = ""
	data["InputAnchor"] = ""
	data["ForwardAnchor"] = ""
	if page == "new-server" && c.Query("router_id") != "" {
		r, e := h.App.Router(a, c.Query("router_id"), "vpn")
		if e != nil {
			h.fail(c, 403, e)
			return
		}
		client, e := h.App.Connect(r)
		if e != nil {
			h.fail(c, 400, e)
			return
		}
		rules, e := routers.List(c.Request.Context(), client, "ip/firewall/filter")
		if e != nil {
			h.fail(c, 502, e)
			return
		}
		anchors := map[string]string{"input": "__empty__", "forward": "__empty__"}
		for _, chain := range []string{"input", "forward"} {
			for _, rule := range rules {
				if rule["chain"] == chain {
					if rule["dynamic"] == "yes" || rule["dynamic"] == "true" {
						h.fail(c, 409, errors.New("dynamic first-chain firewall rules require expert review"))
						return
					}
					anchors[chain] = rule[".id"]
					break
				}
			}
		}
		data["SelectedRouterID"] = r.ID
		data["InputAnchor"] = anchors["input"]
		data["ForwardAnchor"] = anchors["forward"]
		data["FirewallRows"] = rules
	}
	switch page {
	case "job":
		var j domain.Job
		if e = h.App.Repo.Get("jobs", c.Query("id"), &j); e != nil || !a.Can("read", j.RouterID) || a.Role == domain.EndUser {
			h.fail(c, 404, errors.New("job unavailable"))
			return
		}
		data["Job"] = redactJob(j)
	case "router", "sessions", "certificates":
		r, e := h.App.Router(a, c.Query("id"), "read")
		if e != nil || a.Role == domain.EndUser {
			h.fail(c, 403, errors.New("router access denied"))
			return
		}
		r.Password = ""
		r.CA = ""
		data["Router"] = r
		if page != "router" {
			original, e := h.App.Router(a, r.ID, "read")
			if e != nil {
				h.fail(c, 500, e)
				return
			}
			client, e := h.App.Connect(original)
			if e != nil {
				h.fail(c, 400, e)
				return
			}
			path := "ppp/active"
			if page == "certificates" {
				path = "certificate"
			}
			rows, e := routers.List(c.Request.Context(), client, path)
			if e != nil {
				h.fail(c, 502, e)
				return
			}
			data["Rows"] = rows
			data["ObservedAt"] = time.Now().UTC()
		}
	case "audit":
		if a.Role != domain.Super && a.Role != domain.Auditor {
			h.fail(c, 403, errors.New("permission denied"))
			return
		}
		var events []domain.Audit
		if e = h.App.Repo.List("audit", &events); e != nil {
			h.fail(c, 500, e)
			return
		}
		sort.Slice(events, func(i, j int) bool { return events[i].At.After(events[j].At) })
		visible := h.visibleAudit(a, events)
		data["Audit"] = visible[:min(200, len(visible))]
	case "accounts":
		if a.Role != domain.Super {
			h.fail(c, 403, errors.New("permission denied"))
			return
		}
		var accounts []domain.Account
		if e = h.App.Repo.List("accounts", &accounts); e != nil {
			h.fail(c, 500, e)
			return
		}
		for i := range accounts {
			accounts[i].Hash = ""
		}
		data["Accounts"] = accounts
	case "dashboard", "routers", "new-router", "servers", "new-server", "users", "new-user", "jobs", "portal", "delivery", "settings":
	default:
		h.fail(c, 404, errors.New("page not found"))
		return
	}
	h.render(c, 200, data)
}
func redactJob(j domain.Job) domain.Job {
	steps := make([]domain.Step, len(j.Steps))
	for i, s := range j.Steps {
		steps[i] = s
		steps[i].Values = map[string]string{}
		for k, v := range s.Values {
			if k == "password" {
				v = "[REDACTED]"
			}
			steps[i].Values[k] = v
		}
	}
	j.Steps = steps
	j.Fingerprint = ""
	return j
}
func (h *Server) routerList(c *gin.Context) {
	var rows []domain.Router
	if e := h.App.Repo.List("routers", &rows); e != nil {
		h.fail(c, 500, e)
		return
	}
	out := []domain.Router{}
	for _, r := range rows {
		if account(c).Role != domain.EndUser && account(c).Can("read", r.ID) {
			r.Password = ""
			r.CA = ""
			out = append(out, r)
		}
	}
	c.JSON(200, gin.H{"items": out})
}
func (h *Server) addRouter(c *gin.Context) {
	var r domain.Router
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		var in struct {
			Name     string `json:"name"`
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
			CA       string `json:"ca_pem"`
			Group    string `json:"group"`
		}
		if e := decode(c, &in); e != nil {
			h.fail(c, 400, e)
			return
		}
		r = domain.Router{Name: in.Name, Host: in.Host, Port: in.Port, Username: in.Username, Password: in.Password, CA: in.CA, Group: in.Group}
	} else {
		r = domain.Router{Name: c.PostForm("name"), Host: c.PostForm("host"), Username: c.PostForm("username"), Password: c.PostForm("password"), CA: c.PostForm("ca_pem"), Group: c.PostForm("group")}
		r.Port, _ = strconv.Atoi(c.PostForm("port"))
	}
	r, e := h.App.AddRouter(account(c), r)
	if e != nil {
		h.fail(c, 400, e)
		return
	}
	r.Password = ""
	r.CA = ""
	h.success(c, r, "/?page=routers")
}
func (h *Server) syncRouter(c *gin.Context) {
	r, e := h.App.Sync(c.Request.Context(), account(c), c.Param("id"))
	if e != nil {
		h.fail(c, 502, e)
		return
	}
	r.Password = ""
	r.CA = ""
	h.success(c, r, "/?page=router&id="+r.ID)
}
func (h *Server) removeRouter(c *gin.Context) {
	a := account(c)
	if a.Role != domain.Super {
		h.fail(c, 403, errors.New("permission denied"))
		return
	}
	id := c.Param("id")
	unlock := h.App.Engine.Lock(id)
	defer unlock()
	var servers []domain.Server
	if e := h.App.Repo.List("servers", &servers); e != nil {
		h.fail(c, 500, errors.New("cannot verify router dependencies"))
		return
	}
	for _, s := range servers {
		if s.Spec.RouterID == id {
			h.fail(c, 409, errors.New("router has managed servers; inventory removal is blocked"))
			return
		}
	}
	if h.App.Engine.Blocked(id, "") {
		h.fail(c, 409, errors.New("router has unresolved jobs"))
		return
	}
	if e := h.App.Audit(a.ID, "router.remove", id, "inventory_only"); e != nil {
		h.fail(c, 500, e)
		return
	}
	if e := h.App.Repo.Delete("routers", id); e != nil {
		h.fail(c, 500, e)
		return
	}
	c.Status(204)
}
func (h *Server) capabilities(c *gin.Context) {
	r, e := h.App.Router(account(c), c.Param("id"), "read")
	if e != nil || account(c).Role == domain.EndUser {
		h.fail(c, 403, errors.New("permission denied"))
		return
	}
	c.JSON(200, gin.H{"observed": r.Observed, "last_success": r.LastSuccess})
}
func split(raw string) []string {
	out := []string{}
	for _, v := range strings.Split(raw, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
func (h *Server) prepareServer(c *gin.Context) {
	var in struct {
		Spec domain.ServerSpec `json:"spec"`
		Key  string            `json:"idempotency_key"`
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		if e := decode(c, &in); e != nil {
			h.fail(c, 400, e)
			return
		}
	} else {
		in.Spec = domain.ServerSpec{RouterID: c.PostForm("router_id"), Name: c.PostForm("name"), Endpoint: c.PostForm("endpoint"), Transport: c.PostForm("transport"), Subnet: c.PostForm("subnet"), Start: c.PostForm("pool_start"), End: c.PostForm("pool_end"), Gateway: c.PostForm("gateway"), DNS: split(c.PostForm("dns")), Allowed: split(c.PostForm("allowed")), InputAnchor: c.PostForm("input_anchor"), ForwardAnchor: c.PostForm("forward_anchor")}
		in.Spec.Port, _ = strconv.Atoi(c.PostForm("port"))
		in.Key = c.PostForm("idempotency_key")
	}
	j, e := h.App.PrepareServer(c.Request.Context(), account(c), in.Spec, in.Key)
	if e != nil {
		h.fail(c, 400, e)
		return
	}
	h.success(c, redactJob(*j), "/?page=job&id="+j.ID)
}
func (h *Server) prepareUser(c *gin.Context) {
	var in app.UserRequest
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		if e := decode(c, &in); e != nil {
			h.fail(c, 400, e)
			return
		}
	} else {
		in = app.UserRequest{ServerID: c.PostForm("server_id"), Username: c.PostForm("username"), Password: c.PostForm("password"), Address: c.PostForm("address"), AccountID: c.PostForm("account_id"), Key: c.PostForm("idempotency_key")}
	}
	j, e := h.App.PrepareUser(c.Request.Context(), account(c), in)
	if e != nil {
		h.fail(c, 400, e)
		return
	}
	h.success(c, redactJob(*j), "/?page=job&id="+j.ID)
}
func (h *Server) apply(c *gin.Context) {
	var in struct {
		Confirm bool `json:"confirm"`
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		if e := decode(c, &in); e != nil {
			h.fail(c, 400, e)
			return
		}
	} else {
		in.Confirm = c.PostForm("confirm") == "yes"
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Minute)
	defer cancel()
	if e := h.App.Apply(ctx, account(c), c.Param("id"), in.Confirm); e != nil {
		h.fail(c, 409, e)
		return
	}
	h.success(c, gin.H{"ok": true}, "/?page=job&id="+c.Param("id"))
}
func (h *Server) job(c *gin.Context) {
	var j domain.Job
	if e := h.App.Repo.Get("jobs", c.Param("id"), &j); e != nil || account(c).Role == domain.EndUser || !account(c).Can("read", j.RouterID) {
		h.fail(c, 404, errors.New("job unavailable"))
		return
	}
	c.JSON(200, redactJob(j))
}
func (h *Server) userAction(c *gin.Context) {
	if c.PostForm("confirm") != "yes" && c.GetHeader("X-Confirm-Operation") != "yes" {
		h.fail(c, 400, errors.New("explicit confirmation required"))
		return
	}
	if e := h.App.UserAction(c.Request.Context(), account(c), c.Param("id"), c.Param("action")); e != nil {
		h.fail(c, 409, e)
		return
	}
	h.success(c, gin.H{"ok": true}, "/?page=users")
}
func (h *Server) export(c *gin.Context) {
	var in struct {
		Passphrase string `json:"passphrase"`
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		if e := decode(c, &in); e != nil {
			h.fail(c, 400, e)
			return
		}
	} else {
		in.Passphrase = c.PostForm("passphrase")
	}
	data, e := h.App.Export(c.Request.Context(), account(c), c.Param("id"), in.Passphrase)
	if e != nil {
		h.fail(c, 409, e)
		return
	}
	token, e := h.App.IssueDelivery(account(c), c.Param("id"), data)
	if e != nil {
		h.fail(c, 500, e)
		return
	}
	link, png, e := provisioning.DownloadQR(h.App.Config.Origin, token)
	if e != nil {
		h.fail(c, 500, e)
		return
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		c.JSON(200, gin.H{"url": link, "expires_in_seconds": 300, "mode": "authenticated_download_link"})
		return
	}
	h.render(c, 200, gin.H{"Page": "issued", "Account": account(c), "Token": token, "Link": link, "QR": template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))})
}
func (h *Server) download(c *gin.Context) {
	var in struct {
		Token string `json:"token"`
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		if e := decode(c, &in); e != nil {
			h.fail(c, 400, e)
			return
		}
	} else {
		in.Token = c.PostForm("token")
	}
	if e := h.App.Audit(account(c).ID, "profile.download", "delivery", "requested"); e != nil {
		h.fail(c, 500, e)
		return
	}
	data, e := h.App.Consume(account(c), in.Token)
	if e != nil {
		h.fail(c, 403, e)
		return
	}
	if e = h.App.Audit(account(c).ID, "profile.download", "delivery", "consumed"); e != nil {
		h.Log.Error("download consumed audit failed", "account", account(c).ID, "request_id", c.Writer.Header().Get("X-Request-ID"))
	}
	c.Header("Content-Disposition", `attachment; filename="client.ovpn"`)
	c.Data(200, "application/x-openvpn-profile", data)
}
func (h *Server) revokeDelivery(c *gin.Context) {
	token := c.PostForm("token")
	a := account(c)
	e := h.App.RevokeDelivery(a, token)
	if e != nil {
		h.fail(c, 403, e)
		return
	}
	h.success(c, gin.H{"ok": true}, "/?page=users")
}
func (h *Server) addAccount(c *gin.Context) {
	var in struct {
		Username string      `json:"username"`
		Password string      `json:"password"`
		Role     domain.Role `json:"role"`
		Routers  []string    `json:"routers"`
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		if e := decode(c, &in); e != nil {
			h.fail(c, 400, e)
			return
		}
	} else {
		in.Username = c.PostForm("username")
		in.Password = c.PostForm("password")
		in.Role = domain.Role(c.PostForm("role"))
		in.Routers = split(c.PostForm("routers"))
	}
	if e := h.App.AddAccount(account(c), in.Username, in.Password, in.Role, in.Routers); e != nil {
		h.fail(c, 400, e)
		return
	}
	h.success(c, gin.H{"ok": true}, "/?page=accounts")
}
func (h *Server) audit(c *gin.Context) {
	a := account(c)
	if a.Role != domain.Super && a.Role != domain.Auditor {
		h.fail(c, 403, errors.New("permission denied"))
		return
	}
	var events []domain.Audit
	if e := h.App.Repo.List("audit", &events); e != nil {
		h.fail(c, 500, e)
		return
	}
	out := h.visibleAudit(a, events)
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}
	offset = min(offset, len(out))
	c.JSON(200, gin.H{"items": out[offset:min(offset+100, len(out))], "total": len(out)})
}

func (h *Server) visibleAudit(a domain.Account, events []domain.Audit) []domain.Audit {
	out := []domain.Audit{}
	for _, event := range events {
		if a.Role == domain.Super || event.Actor == a.ID {
			out = append(out, event)
			continue
		}
		var router domain.Router
		if h.App.Repo.Get("routers", event.Target, &router) == nil && a.Can("read", router.ID) {
			out = append(out, event)
			continue
		}
		var job domain.Job
		if h.App.Repo.Get("jobs", event.Target, &job) == nil && a.Can("read", job.RouterID) {
			out = append(out, event)
			continue
		}
		var user domain.VPNUser
		if h.App.Repo.Get("users", event.Target, &user) == nil && a.Can("read", user.RouterID) {
			out = append(out, event)
		}
	}
	return out
}
func (h *Server) sessions(c *gin.Context) {
	a := account(c)
	if a.Role == domain.EndUser {
		h.fail(c, 403, errors.New("permission denied"))
		return
	}
	r, e := h.App.Router(a, c.Param("id"), "read")
	if e != nil {
		h.fail(c, 403, e)
		return
	}
	client, e := h.App.Connect(r)
	if e != nil {
		h.fail(c, 400, e)
		return
	}
	rows, e := routers.List(c.Request.Context(), client, "ppp/active")
	if e != nil {
		h.fail(c, 502, e)
		return
	}
	c.JSON(200, gin.H{"items": rows, "observed_at": time.Now().UTC()})
}
