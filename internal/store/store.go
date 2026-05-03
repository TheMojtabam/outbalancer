package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/outbalancer/outbalancer/internal/config"
)

// Store persists app config + runtime metrics to disk.
type Store struct {
	mu       sync.RWMutex
	path     string
	cfg      config.AppConfig
	metrics  map[string]*ServerMetrics // key=server ID
	traffic  []TrafficSample           // ring buffer of recent samples
	alerts   []Alert
	logs     []LogEntry
	topDomains []DomainStat // populated from xray stats; empty until then
	maxAlerts int
	maxLogs   int
}

// DomainStat is a single row in the "top domains" table. These rows are only
// produced by the xray-stats sampler — we never invent rows here.
type DomainStat struct {
	Domain string `json:"domain"`
	Bytes  int64  `json:"bytes"`
	Icon   string `json:"icon"`
	Color  string `json:"color"`
}

// SetTopDomains replaces the cached top-domain stats. Called by the
// stats sampler when xray-core reports per-domain bytes.
func (s *Store) SetTopDomains(list []DomainStat) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.topDomains = list
}

// ServerMetrics holds per-server runtime metrics.
type ServerMetrics struct {
	ServerID    string    `json:"server_id"`
	PingMs      float64   `json:"ping_ms"`
	SpeedMbps   float64   `json:"speed_mbps"`
	UpBytes     int64     `json:"up_bytes"`
	DownBytes   int64     `json:"down_bytes"`
	Connections int       `json:"connections"`
	Status      string    `json:"status"` // online | degraded | offline
	Score       int       `json:"score"`  // 0-100
	LastChecked time.Time `json:"last_checked"`
	FailCount   int       `json:"fail_count"`
	UpSince     time.Time `json:"up_since"`
	UsageMonth  int64     `json:"usage_month_bytes"`
}

// TrafficSample is one second of throughput data.
type TrafficSample struct {
	Time      time.Time          `json:"time"`
	PerServer map[string]float64 `json:"per_server"` // server ID -> Mbps
	TotalUp   float64            `json:"total_up"`
	TotalDown float64            `json:"total_down"`
}

// Alert is a notification.
type Alert struct {
	ID       string    `json:"id"`
	Level    string    `json:"level"` // info | warn | crit
	Title    string    `json:"title"`
	Message  string    `json:"message"`
	ServerID string    `json:"server_id,omitempty"`
	Time     time.Time `json:"time"`
	Read     bool      `json:"read"`
}

// LogEntry is a single log line.
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Source  string    `json:"source"`
	Message string    `json:"message"`
}

// New creates a new Store, loading from disk if present.
func New(path string) (*Store, error) {
	s := &Store{
		path:      path,
		metrics:   make(map[string]*ServerMetrics),
		traffic:   make([]TrafficSample, 0, 120),
		alerts:    make([]Alert, 0, 200),
		logs:      make([]LogEntry, 0, 1000),
		maxAlerts: 200,
		maxLogs:   1000,
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		s.cfg = config.Default()
		if err := s.persist(); err != nil {
			return nil, err
		}
		return s, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &s.cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// initialize metrics for each known server
	for _, srv := range s.cfg.Servers {
		s.metrics[srv.ID] = &ServerMetrics{
			ServerID: srv.ID,
			Status:   "unknown",
			Score:    0,
		}
	}

	return s, nil
}

func (s *Store) persist() error {
	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Config returns a copy of the current config.
func (s *Store) Config() config.AppConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// SaveConfig replaces the config and persists.
func (s *Store) SaveConfig(c config.AppConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = c
	return s.persist()
}

// AddServer appends a new server (deduplicated by ID).
func (s *Store) AddServer(srv config.Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.cfg.Servers {
		if existing.ID == srv.ID {
			return errors.New("server already exists")
		}
	}
	s.cfg.Servers = append(s.cfg.Servers, srv)
	s.metrics[srv.ID] = &ServerMetrics{
		ServerID: srv.ID,
		Status:   "unknown",
	}
	return s.persist()
}

// UpdateServer replaces a server in-place.
func (s *Store) UpdateServer(srv config.Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.cfg.Servers {
		if existing.ID == srv.ID {
			s.cfg.Servers[i] = srv
			return s.persist()
		}
	}
	return errors.New("server not found")
}

// DeleteServer removes a server.
func (s *Store) DeleteServer(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.cfg.Servers {
		if existing.ID == id {
			s.cfg.Servers = append(s.cfg.Servers[:i], s.cfg.Servers[i+1:]...)
			delete(s.metrics, id)
			return s.persist()
		}
	}
	return errors.New("server not found")
}

// Servers returns a copy of the server list.
func (s *Store) Servers() []config.Server {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]config.Server, len(s.cfg.Servers))
	copy(out, s.cfg.Servers)
	return out
}

// SetMetrics updates runtime metrics for a server.
func (s *Store) SetMetrics(id string, m *ServerMetrics) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics[id] = m
}

// Metrics returns a snapshot of all metrics.
func (s *Store) Metrics() map[string]*ServerMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*ServerMetrics, len(s.metrics))
	for k, v := range s.metrics {
		copy := *v
		out[k] = &copy
	}
	return out
}

// MetricsFor returns a snapshot of one server's metrics, or nil.
func (s *Store) MetricsFor(id string) *ServerMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m, ok := s.metrics[id]; ok {
		copy := *m
		return &copy
	}
	return nil
}

// PushTraffic stores one traffic sample (keeps last 120).
func (s *Store) PushTraffic(sample TrafficSample) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traffic = append(s.traffic, sample)
	if len(s.traffic) > 120 {
		s.traffic = s.traffic[len(s.traffic)-120:]
	}
}

// Traffic returns recent traffic samples.
func (s *Store) Traffic() []TrafficSample {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TrafficSample, len(s.traffic))
	copy(out, s.traffic)
	return out
}

// AddAlert appends an alert (deduplicated by title+server within 1 min).
func (s *Store) AddAlert(a Alert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.ID == "" {
		a.ID = fmt.Sprintf("a_%d", time.Now().UnixNano())
	}
	if a.Time.IsZero() {
		a.Time = time.Now()
	}
	// simple dedup: skip if same title+server in last 60s
	cutoff := time.Now().Add(-60 * time.Second)
	for i := len(s.alerts) - 1; i >= 0 && s.alerts[i].Time.After(cutoff); i-- {
		if s.alerts[i].Title == a.Title && s.alerts[i].ServerID == a.ServerID {
			return
		}
	}
	s.alerts = append(s.alerts, a)
	if len(s.alerts) > s.maxAlerts {
		s.alerts = s.alerts[len(s.alerts)-s.maxAlerts:]
	}
}

// Alerts returns recent alerts (newest first).
func (s *Store) Alerts() []Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Alert, len(s.alerts))
	for i, a := range s.alerts {
		out[len(s.alerts)-1-i] = a
	}
	return out
}

// MarkAlertRead marks an alert as read.
func (s *Store) MarkAlertRead(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.alerts {
		if s.alerts[i].ID == id {
			s.alerts[i].Read = true
			return
		}
	}
}

// AddLog appends a log line.
func (s *Store) AddLog(level, source, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, LogEntry{
		Time:    time.Now(),
		Level:   level,
		Source:  source,
		Message: msg,
	})
	if len(s.logs) > s.maxLogs {
		s.logs = s.logs[len(s.logs)-s.maxLogs:]
	}
}

// Logs returns recent logs (newest first).
func (s *Store) Logs(limit int) []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.logs) {
		limit = len(s.logs)
	}
	out := make([]LogEntry, limit)
	for i := 0; i < limit; i++ {
		out[i] = s.logs[len(s.logs)-1-i]
	}
	return out
}

// Export returns the entire config as JSON for backup.
func (s *Store) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(s.cfg, "", "  ")
}

// TopDomains returns aggregated traffic per domain (descending). When no
// xray-core stats source has populated this data, returns an empty slice —
// we never invent traffic that didn't happen.
func (s *Store) TopDomains() []map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.topDomains) == 0 {
		return []map[string]any{}
	}
	// copy to detach from the lock
	out := make([]map[string]any, 0, len(s.topDomains))
	for _, d := range s.topDomains {
		out = append(out, map[string]any{
			"domain": d.Domain,
			"bytes":  d.Bytes,
			"icon":   d.Icon,
			"color":  d.Color,
		})
	}
	return out
}

// Import replaces the config from JSON bytes.
func (s *Store) Import(data []byte) error {
	var cfg config.AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	return s.SaveConfig(cfg)
}
