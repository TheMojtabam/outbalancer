//go:build !noxray
// +build !noxray

package xray

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/outbalancer/outbalancer/internal/store"

	// Core xray imports — these embed the entire xray-core into our binary.
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"

	// Required handlers — register via init()
	_ "github.com/xtls/xray-core/app/dispatcher"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"

	// Optional features
	_ "github.com/xtls/xray-core/app/dns"
	_ "github.com/xtls/xray-core/app/dns/fakedns"
	_ "github.com/xtls/xray-core/app/metrics"
	_ "github.com/xtls/xray-core/app/observatory" // REQUIRED for leastPing/leastLoad
	_ "github.com/xtls/xray-core/app/policy"
	_ "github.com/xtls/xray-core/app/router"
	_ "github.com/xtls/xray-core/app/stats"

	// Fix dependency cycle (per Piazche / xray-core requirements)
	_ "github.com/xtls/xray-core/transport/internet/tagged/taggedimpl"

	// Inbound + outbound proxies we support
	_ "github.com/xtls/xray-core/proxy/blackhole"
	_ "github.com/xtls/xray-core/proxy/dns"
	_ "github.com/xtls/xray-core/proxy/dokodemo"
	_ "github.com/xtls/xray-core/proxy/freedom"
	_ "github.com/xtls/xray-core/proxy/http"
	_ "github.com/xtls/xray-core/proxy/loopback"
	_ "github.com/xtls/xray-core/proxy/socks"
	_ "github.com/xtls/xray-core/proxy/trojan"
	_ "github.com/xtls/xray-core/proxy/vless/inbound"
	_ "github.com/xtls/xray-core/proxy/vless/outbound"
	_ "github.com/xtls/xray-core/proxy/vmess/inbound"
	_ "github.com/xtls/xray-core/proxy/vmess/outbound"

	// Transports
	_ "github.com/xtls/xray-core/transport/internet/grpc"
	_ "github.com/xtls/xray-core/transport/internet/http"
	_ "github.com/xtls/xray-core/transport/internet/httpupgrade"
	_ "github.com/xtls/xray-core/transport/internet/reality"
	_ "github.com/xtls/xray-core/transport/internet/splithttp"
	_ "github.com/xtls/xray-core/transport/internet/tcp"
	_ "github.com/xtls/xray-core/transport/internet/tls"
	_ "github.com/xtls/xray-core/transport/internet/udp"
	_ "github.com/xtls/xray-core/transport/internet/websocket"

	// Transport headers
	_ "github.com/xtls/xray-core/transport/internet/headers/http"
	_ "github.com/xtls/xray-core/transport/internet/headers/tls"

	// JSON config loader
	_ "github.com/xtls/xray-core/main/json"
)

// Manager runs an *embedded* xray-core instance — no subprocess, no separate
// binary. The whole engine lives inside this Go process. This is essential for
// air-gapped servers (Iran with no international internet) because there's
// nothing to download.
type Manager struct {
	mu       sync.Mutex
	store    *store.Store
	workDir  string
	cfgPath  string
	instance *core.Instance
}

// NewManager creates a new embedded-xray manager. The legacy `binPath`
// parameter is accepted but ignored — kept only so existing callers compile
// without modification.
func NewManager(s *store.Store, _ string, workDir string) *Manager {
	if workDir == "" {
		workDir = filepath.Join(os.TempDir(), "outbalancer")
	}
	_ = os.MkdirAll(workDir, 0o755)
	return &Manager{
		store:   s,
		workDir: workDir,
		cfgPath: filepath.Join(workDir, "xray-config.json"),
	}
}

// Disabled is always false now: xray is embedded in the binary, it's always available.
func (m *Manager) Disabled() bool { return false }

// BinaryPath returns a descriptive label for the embedded core (no real path).
func (m *Manager) BinaryPath() string { return "embedded (xray-core in process)" }

// ConfigPath returns the path of the rendered xray config snapshot.
func (m *Manager) ConfigPath() string { return m.cfgPath }

// Apply rebuilds the xray config and (re)starts the embedded core.
func (m *Manager) Apply() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := m.store.Config()

	// Skip starting xray when there are no servers configured yet.
	if len(cfg.Servers) == 0 {
		m.store.AddLog("info", "xray", "هیچ سروری تعریف نشده — xray هنوز start نشده")
		return m.stopLocked()
	}

	// Snapshot the current ping metrics so the balancer strategy weights
	// can be computed from real TCP pings (no Observatory / HTTP probe).
	metrics := m.store.Metrics()
	pings := make(map[string]float64, len(metrics))
	for id, met := range metrics {
		if met != nil && met.PingMs > 0 && met.PingMs < 9000 {
			pings[id] = met.PingMs
		}
	}
	xc := buildXrayConfigWithMetrics(cfg, pings)
	data, err := json.MarshalIndent(xc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal xray cfg: %w", err)
	}
	// Save snapshot for inspection / debugging
	if err := os.WriteFile(m.cfgPath, data, 0o644); err != nil {
		// non-fatal
		m.store.AddLog("warn", "xray", "نتوانستم config را روی دیسک ذخیره کنم: "+err.Error())
	}
	m.store.AddLog("info", "xray", fmt.Sprintf("config رندر شد (%d outbound)", len(cfg.Servers)))

	// Stop existing instance
	if err := m.stopLocked(); err != nil {
		m.store.AddLog("warn", "xray", "stop قبلی: "+err.Error())
	}

	// Decode JSON → protobuf → instance
	jsonCfg, err := serial.DecodeJSONConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode JSON config: %w", err)
	}
	pbCfg, err := jsonCfg.Build()
	if err != nil {
		return fmt.Errorf("build xray config: %w", err)
	}
	inst, err := core.New(pbCfg)
	if err != nil {
		return fmt.Errorf("create xray instance: %w", err)
	}
	if err := inst.Start(); err != nil {
		_ = inst.Close()
		return fmt.Errorf("start xray instance: %w", err)
	}

	m.instance = inst
	m.store.AddLog("info", "xray", fmt.Sprintf(
		"xray-core فعال شد · SOCKS=%d · HTTP=%d",
		cfg.Listen.SOCKSPort, cfg.Listen.HTTPPort))

	// Tiny grace period so listeners are surely accepting before callers test.
	time.Sleep(150 * time.Millisecond)
	return nil
}

// Stop closes the running xray instance. Safe to call on already-stopped manager.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.stopLocked()
}

// stopLocked closes the instance — caller must hold the mutex.
func (m *Manager) stopLocked() error {
	if m.instance == nil {
		return nil
	}
	err := m.instance.Close()
	m.instance = nil
	return err
}
