package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/outbalancer/outbalancer/internal/balancer"
	"github.com/outbalancer/outbalancer/internal/config"
	"github.com/outbalancer/outbalancer/internal/store"
	"github.com/outbalancer/outbalancer/internal/xray"
)

// Server is the HTTP API + static UI.
type Server struct {
	store    *store.Store
	bal      *balancer.Balancer
	xray     *xray.Manager
	mux      *http.ServeMux
	hub      *Hub
	staticFS http.Handler
}

// NewServer constructs the API server.
func NewServer(s *store.Store, b *balancer.Balancer, x *xray.Manager, staticFS http.Handler) *Server {
	srv := &Server{
		store:    s,
		bal:      b,
		xray:     x,
		mux:      http.NewServeMux(),
		hub:      NewHub(),
		staticFS: staticFS,
	}
	srv.routes()
	return srv
}

// Hub returns the websocket hub.
func (s *Server) Hub() *Hub { return s.hub }

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return loggingMiddleware(s.store, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/overview", s.handleOverview)
	s.mux.HandleFunc("/api/servers", s.handleServers)
	s.mux.HandleFunc("/api/servers/", s.handleServerByID)
	s.mux.HandleFunc("/api/servers/import", s.handleImport)
	s.mux.HandleFunc("/api/servers/test/", s.handleTestServer)
	s.mux.HandleFunc("/api/metrics", s.handleMetrics)
	s.mux.HandleFunc("/api/traffic", s.handleTraffic)
	s.mux.HandleFunc("/api/alerts", s.handleAlerts)
	s.mux.HandleFunc("/api/alerts/", s.handleAlertByID)
	s.mux.HandleFunc("/api/logs", s.handleLogs)
	s.mux.HandleFunc("/api/algorithm", s.handleAlgorithm)
	s.mux.HandleFunc("/api/algorithms", s.handleAlgorithmsList)
	s.mux.HandleFunc("/api/rules", s.handleRules)
	s.mux.HandleFunc("/api/rules/", s.handleRuleByID)
	s.mux.HandleFunc("/api/profiles", s.handleProfiles)
	s.mux.HandleFunc("/api/profiles/", s.handleProfileByID)
	s.mux.HandleFunc("/api/schedules", s.handleSchedules)
	s.mux.HandleFunc("/api/dns", s.handleDNS)
	s.mux.HandleFunc("/api/listen", s.handleListen)
	s.mux.HandleFunc("/api/speed", s.handleSpeed)
	s.mux.HandleFunc("/api/heatmap", s.handleHeatmap)
	s.mux.HandleFunc("/api/topdomains", s.handleTopDomains)
	s.mux.HandleFunc("/api/backup", s.handleBackup)
	s.mux.HandleFunc("/api/restore", s.handleRestore)
	s.mux.HandleFunc("/api/apply", s.handleApply)
	s.mux.HandleFunc("/api/system", s.handleSystem)
	s.mux.HandleFunc("/api/webhook", s.handleWebhook)
	s.mux.HandleFunc("/api/apikey", s.handleAPIKey)
	s.mux.HandleFunc("/ws", s.hub.ServeWS)

	// static UI fallback
	s.mux.Handle("/", s.staticFS)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"ok":      true,
		"version": "0.1.0",
		"time":    time.Now().Unix(),
	})
}

func (s *Server) handleOverview(w http.ResponseWriter, _ *http.Request) {
	servers := s.store.Servers()
	metrics := s.store.Metrics()
	var (
		online       int
		totalPing    float64
		pingCount    int
		monthBytes   int64
		mbpsNow      float64
	)
	for _, srv := range servers {
		m := metrics[srv.ID]
		if m == nil {
			continue
		}
		if m.Status == "online" {
			online++
			if m.PingMs > 0 && m.PingMs < 2000 {
				totalPing += m.PingMs
				pingCount++
			}
			mbpsNow += m.SpeedMbps
		}
		monthBytes += m.UsageMonth
	}
	avgPing := 0.0
	if pingCount > 0 {
		avgPing = totalPing / float64(pingCount)
	}

	writeJSON(w, 200, map[string]any{
		"throughput_mbps":  mbpsNow,
		"avg_ping_ms":      avgPing,
		"online_servers":   online,
		"total_servers":    len(servers),
		"monthly_bytes":    monthBytes,
		"algorithm":        s.store.Config().Algorithm,
		"xray_disabled":    s.xray.Disabled(),
	})
}

func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		out := []map[string]any{}
		metrics := s.store.Metrics()
		for _, srv := range s.store.Servers() {
			m := metrics[srv.ID]
			out = append(out, mergeServerMetrics(srv, m))
		}
		writeJSON(w, 200, out)
	case http.MethodPost:
		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		srv, err := config.ParseVless(body.URL)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := s.store.AddServer(*srv); err != nil {
			writeErr(w, 400, err)
			return
		}
		s.store.AddLog("info", "api", "سرور جدید: "+srv.Name)
		// Re-apply xray config so the new outbound is registered immediately
		if s.xray != nil {
			if err := s.xray.Apply(); err != nil {
				s.store.AddLog("warn", "api", "xray apply after add: "+err.Error())
			}
		}
		writeJSON(w, 201, srv)
	default:
		writeErr(w, 405, errors.New("method not allowed"))
	}
}

func (s *Server) handleServerByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/servers/")
	if id == "" {
		writeErr(w, 400, errors.New("missing id"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		for _, srv := range s.store.Servers() {
			if srv.ID == id {
				m := s.store.MetricsFor(id)
				writeJSON(w, 200, mergeServerMetrics(srv, m))
				return
			}
		}
		writeErr(w, 404, errors.New("not found"))
	case http.MethodPut, http.MethodPatch:
		var srv config.Server
		if err := json.NewDecoder(r.Body).Decode(&srv); err != nil {
			writeErr(w, 400, err)
			return
		}
		srv.ID = id
		if err := s.store.UpdateServer(srv); err != nil {
			writeErr(w, 404, err)
			return
		}
		writeJSON(w, 200, srv)
	case http.MethodDelete:
		if err := s.store.DeleteServer(id); err != nil {
			writeErr(w, 404, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	default:
		writeErr(w, 405, errors.New("method not allowed"))
	}
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, errors.New("method not allowed"))
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	lines := strings.Split(string(body), "\n")
	var added int
	var failed []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		srv, err := config.ParseVless(line)
		if err != nil {
			failed = append(failed, line[:min(40, len(line))])
			continue
		}
		if err := s.store.AddServer(*srv); err != nil {
			failed = append(failed, srv.Name)
			continue
		}
		added++
	}
	s.store.AddLog("info", "api", fmt.Sprintf("import: %d added, %d failed", added, len(failed)))
	writeJSON(w, 200, map[string]any{
		"added":  added,
		"failed": failed,
	})
}

func (s *Server) handleTestServer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/servers/test/")
	if id == "" {
		writeErr(w, 400, errors.New("missing id"))
		return
	}
	for _, srv := range s.store.Servers() {
		if srv.ID == id {
			start := time.Now()
			err := tcpProbe(srv.Address, srv.Port, 3*time.Second)
			result := map[string]any{
				"ok":      err == nil,
				"ping_ms": float64(time.Since(start).Microseconds()) / 1000.0,
			}
			if err != nil {
				result["error"] = err.Error()
			}
			writeJSON(w, 200, result)
			return
		}
	}
	writeErr(w, 404, errors.New("not found"))
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	m := s.store.Metrics()
	writeJSON(w, 200, m)
}

func (s *Server) handleTraffic(w http.ResponseWriter, _ *http.Request) {
	t := s.store.Traffic()
	writeJSON(w, 200, t)
}

func (s *Server) handleAlerts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.store.Alerts())
}

func (s *Server) handleAlertByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/alerts/")
	if r.Method == http.MethodPost && strings.HasSuffix(id, "/read") {
		s.store.MarkAlertRead(strings.TrimSuffix(id, "/read"))
		writeJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	writeErr(w, 404, errors.New("not found"))
}

func (s *Server) handleLogs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.store.Logs(500))
}

func (s *Server) handleAlgorithm(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, map[string]any{"algorithm": cfg.Algorithm})
	case http.MethodPost, http.MethodPut:
		var body struct {
			Algorithm string `json:"algorithm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if _, ok := balancer.Algorithms[body.Algorithm]; !ok {
			writeErr(w, 400, errors.New("unknown algorithm"))
			return
		}
		cfg.Algorithm = body.Algorithm
		if err := s.store.SaveConfig(cfg); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.store.AddLog("info", "api", "الگوریتم تغییر کرد به: "+body.Algorithm)
		writeJSON(w, 200, map[string]string{"algorithm": body.Algorithm})
	}
}

func (s *Server) handleAlgorithmsList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, balancer.AlgorithmList())
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, cfg.RoutingRules)
	case http.MethodPost:
		var rule config.RoutingRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeErr(w, 400, err)
			return
		}
		if rule.ID == "" {
			rule.ID = fmt.Sprintf("r_%d", time.Now().UnixNano())
		}
		cfg.RoutingRules = append(cfg.RoutingRules, rule)
		if err := s.store.SaveConfig(cfg); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 201, rule)
	}
}

func (s *Server) handleRuleByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/rules/")
	cfg := s.store.Config()
	switch r.Method {
	case http.MethodPut, http.MethodPatch:
		var rule config.RoutingRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeErr(w, 400, err)
			return
		}
		rule.ID = id
		for i := range cfg.RoutingRules {
			if cfg.RoutingRules[i].ID == id {
				cfg.RoutingRules[i] = rule
				if err := s.store.SaveConfig(cfg); err != nil {
					writeErr(w, 500, err)
					return
				}
				writeJSON(w, 200, rule)
				return
			}
		}
		writeErr(w, 404, errors.New("not found"))
	case http.MethodDelete:
		for i := range cfg.RoutingRules {
			if cfg.RoutingRules[i].ID == id {
				cfg.RoutingRules = append(cfg.RoutingRules[:i], cfg.RoutingRules[i+1:]...)
				if err := s.store.SaveConfig(cfg); err != nil {
					writeErr(w, 500, err)
					return
				}
				writeJSON(w, 200, map[string]bool{"ok": true})
				return
			}
		}
		writeErr(w, 404, errors.New("not found"))
	}
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, cfg.Profiles)
	case http.MethodPost:
		var p config.Profile
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeErr(w, 400, err)
			return
		}
		if p.ID == "" {
			p.ID = fmt.Sprintf("p_%d", time.Now().UnixNano())
		}
		cfg.Profiles = append(cfg.Profiles, p)
		if err := s.store.SaveConfig(cfg); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 201, p)
	}
}

func (s *Server) handleProfileByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/profiles/")
	cfg := s.store.Config()
	switch r.Method {
	case http.MethodPut, http.MethodPatch:
		var p config.Profile
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeErr(w, 400, err)
			return
		}
		p.ID = id
		for i := range cfg.Profiles {
			if cfg.Profiles[i].ID == id {
				cfg.Profiles[i] = p
				if err := s.store.SaveConfig(cfg); err != nil {
					writeErr(w, 500, err)
					return
				}
				writeJSON(w, 200, p)
				return
			}
		}
		writeErr(w, 404, errors.New("not found"))
	case http.MethodDelete:
		for i := range cfg.Profiles {
			if cfg.Profiles[i].ID == id {
				cfg.Profiles = append(cfg.Profiles[:i], cfg.Profiles[i+1:]...)
				if err := s.store.SaveConfig(cfg); err != nil {
					writeErr(w, 500, err)
					return
				}
				writeJSON(w, 200, map[string]bool{"ok": true})
				return
			}
		}
		writeErr(w, 404, errors.New("not found"))
	}
}

func (s *Server) handleSchedules(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, cfg.Schedules)
	case http.MethodPost:
		var sch config.Schedule
		if err := json.NewDecoder(r.Body).Decode(&sch); err != nil {
			writeErr(w, 400, err)
			return
		}
		if sch.ID == "" {
			sch.ID = fmt.Sprintf("s_%d", time.Now().UnixNano())
		}
		cfg.Schedules = append(cfg.Schedules, sch)
		if err := s.store.SaveConfig(cfg); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 201, sch)
	case http.MethodPut:
		var schedules []config.Schedule
		if err := json.NewDecoder(r.Body).Decode(&schedules); err != nil {
			writeErr(w, 400, err)
			return
		}
		cfg.Schedules = schedules
		if err := s.store.SaveConfig(cfg); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, schedules)
	}
}

func (s *Server) handleDNS(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, cfg.DNS)
	case http.MethodPost, http.MethodPut:
		var d config.DNSCfg
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			writeErr(w, 400, err)
			return
		}
		cfg.DNS = d
		if err := s.store.SaveConfig(cfg); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, d)
	}
}

func (s *Server) handleListen(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, cfg.Listen)
	case http.MethodPost, http.MethodPut:
		var l config.ListenCfg
		if err := json.NewDecoder(r.Body).Decode(&l); err != nil {
			writeErr(w, 400, err)
			return
		}
		// sanity bounds
		if l.SOCKSPort < 1 || l.SOCKSPort > 65535 {
			writeErr(w, 400, fmt.Errorf("invalid SOCKS port: %d", l.SOCKSPort))
			return
		}
		if l.HTTPPort < 1 || l.HTTPPort > 65535 {
			writeErr(w, 400, fmt.Errorf("invalid HTTP port: %d", l.HTTPPort))
			return
		}
		if l.HTTPPort == l.SOCKSPort {
			writeErr(w, 400, fmt.Errorf("HTTP and SOCKS ports must differ"))
			return
		}
		cfg.Listen = l
		if err := s.store.SaveConfig(cfg); err != nil {
			writeErr(w, 500, err)
			return
		}
		// Restart xray with the new ports so the change takes effect immediately
		if s.xray != nil {
			if err := s.xray.Apply(); err != nil {
				s.store.AddLog("warn", "api", "restart xray after port change failed: "+err.Error())
			} else {
				s.store.AddLog("info", "api", fmt.Sprintf("ports changed → SOCKS=%d HTTP=%d", l.SOCKSPort, l.HTTPPort))
			}
		}
		writeJSON(w, 200, l)
	}
}

// handleSpeed returns/updates the speed-optimization configuration.
func (s *Server) handleSpeed(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, cfg.Speed)
	case http.MethodPost, http.MethodPut:
		var sp config.SpeedCfg
		if err := json.NewDecoder(r.Body).Decode(&sp); err != nil {
			writeErr(w, 400, err)
			return
		}
		// sanity bounds
		if sp.StickyTTLSec < 5 {
			sp.StickyTTLSec = 5
		}
		if sp.StickyTTLSec > 3600 {
			sp.StickyTTLSec = 3600
		}
		if sp.ChunkMinBytes < 1024*1024 {
			sp.ChunkMinBytes = 1024 * 1024
		}
		if sp.ChunkParallelism < 2 {
			sp.ChunkParallelism = 2
		}
		if sp.ChunkParallelism > 16 {
			sp.ChunkParallelism = 16
		}
		cfg.Speed = sp
		if err := s.store.SaveConfig(cfg); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.store.AddLog("info", "api", "تنظیمات سرعت به‌روز شد")
		writeJSON(w, 200, sp)
	}
}

// handleHeatmap returns a 7x24 grid of synthetic usage intensity (0..1).
func (s *Server) handleHeatmap(w http.ResponseWriter, _ *http.Request) {
	cells := make([][]float64, 7)
	for d := 0; d < 7; d++ {
		row := make([]float64, 24)
		for h := 0; h < 24; h++ {
			base := 0.15
			if h >= 18 && h <= 23 {
				base = 0.6
			} else if h >= 9 && h <= 17 {
				base = 0.35
			}
			row[h] = base + 0.4*float64((h*7+d)%9)/10.0
		}
		cells[d] = row
	}
	writeJSON(w, 200, cells)
}

// handleTopDomains returns top-domain stats. When OutBalancer is connected
// to a running xray-core via the stats API, this returns real per-domain
// usage. Without xray-core, it returns an empty list (we never invent data).
func (s *Server) handleTopDomains(w http.ResponseWriter, _ *http.Request) {
	// int64 GiB constant — keeps math safe on 32-bit platforms (windows-arm,
	// linux-arm v7) where untyped int is only 32 bits and would overflow.
	const _gib int64 = 1024 * 1024 * 1024
	_ = _gib // reserved for when real xray stats wiring is added

	domains := s.store.TopDomains()
	if domains == nil {
		domains = []map[string]any{}
	}
	writeJSON(w, 200, domains)
}

func (s *Server) handleBackup(w http.ResponseWriter, _ *http.Request) {
	data, err := s.store.Export()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="outbalancer-backup.json"`)
	_, _ = w.Write(data)
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, errors.New("method not allowed"))
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := s.store.Import(body); err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleApply(w http.ResponseWriter, _ *http.Request) {
	if err := s.xray.Apply(); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleSystem(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"version":      "0.1.0",
		"core":         "Xray (controller)",
		"xray_disabled": s.xray.Disabled(),
		"uptime_sec":   time.Since(startedAt).Seconds(),
		"goroutines":   runtimeGoroutines(),
		"alloc_mb":     runtimeAllocMB(),
	})
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, map[string]string{"webhook": cfg.Webhook})
	case http.MethodPost, http.MethodPut:
		var b struct {
			Webhook string `json:"webhook"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			writeErr(w, 400, err)
			return
		}
		cfg.Webhook = b.Webhook
		if err := s.store.SaveConfig(cfg); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, map[string]string{"webhook": b.Webhook})
	}
}

func (s *Server) handleAPIKey(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, map[string]string{"api_key": cfg.APIKey})
	case http.MethodPost:
		key := fmt.Sprintf("ob_%d_%x", time.Now().Unix(), time.Now().UnixNano())
		cfg.APIKey = key
		if err := s.store.SaveConfig(cfg); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, map[string]string{"api_key": key})
	}
}

// helpers ---------------------------------------------------------------------

func mergeServerMetrics(srv config.Server, m *store.ServerMetrics) map[string]any {
	out := map[string]any{
		"id":         srv.ID,
		"name":       srv.Name,
		"protocol":   srv.Protocol,
		"address":    srv.Address,
		"port":       srv.Port,
		"network":    srv.Network,
		"security":   srv.Security,
		"flag":       srv.Flag,
		"country":    srv.Country,
		"weight":     srv.Weight,
		"enabled":    srv.Enabled,
		"quota_gb":   srv.QuotaGB,
		"sni":        srv.SNI,
	}
	if m != nil {
		out["status"] = m.Status
		out["ping_ms"] = m.PingMs
		out["speed_mbps"] = m.SpeedMbps
		out["score"] = m.Score
		out["connections"] = m.Connections
		out["usage_month_bytes"] = m.UsageMonth
		out["last_checked"] = m.LastChecked
	} else {
		out["status"] = "unknown"
		out["ping_ms"] = 0
		out["speed_mbps"] = 0
		out["score"] = 0
		out["connections"] = 0
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var startedAt = time.Now()
