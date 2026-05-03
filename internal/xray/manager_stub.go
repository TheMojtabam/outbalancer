//go:build noxray
// +build noxray

package xray

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/outbalancer/outbalancer/internal/store"
)

// Manager (stub variant) — used only in local development builds with `-tags
// noxray`. In production builds, xray-core is embedded via manager_xray.go.
type Manager struct {
	mu      sync.Mutex
	store   *store.Store
	workDir string
	cfgPath string
}

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

func (m *Manager) Disabled() bool      { return true }
func (m *Manager) BinaryPath() string  { return "stub (built with -tags noxray)" }
func (m *Manager) ConfigPath() string  { return m.cfgPath }

func (m *Manager) Apply() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg := m.store.Config()
	xc := buildXrayConfig(cfg)
	data, _ := json.MarshalIndent(xc, "", "  ")
	_ = os.WriteFile(m.cfgPath, data, 0o644)
	m.store.AddLog("info", "xray", fmt.Sprintf("config رندر شد (stub): %d outbound", len(cfg.Servers)))
	return nil
}

func (m *Manager) Stop() {}
