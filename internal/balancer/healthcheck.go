package balancer

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/outbalancer/outbalancer/internal/config"
	"github.com/outbalancer/outbalancer/internal/store"
)

// HealthChecker periodically tests each server's reachability and latency.
type HealthChecker struct {
	store *store.Store
	stop  chan struct{}
	wg    sync.WaitGroup
}

// NewHealthChecker constructs a checker bound to a Store.
func NewHealthChecker(s *store.Store) *HealthChecker {
	return &HealthChecker{
		store: s,
		stop:  make(chan struct{}),
	}
}

// Start runs the periodic health-check loop in a goroutine.
func (h *HealthChecker) Start() {
	h.wg.Add(1)
	go h.loop()
}

// Stop signals the loop to exit and waits.
func (h *HealthChecker) Stop() {
	close(h.stop)
	h.wg.Wait()
}

func (h *HealthChecker) loop() {
	defer h.wg.Done()

	// run an initial check immediately, then on the configured interval
	h.checkAll()
	ticker := time.NewTicker(time.Duration(h.store.Config().HealthCheck.IntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.stop:
			return
		case <-ticker.C:
			cfg := h.store.Config()
			ticker.Reset(time.Duration(cfg.HealthCheck.IntervalSec) * time.Second)
			h.checkAll()
		}
	}
}

func (h *HealthChecker) checkAll() {
	servers := h.store.Servers()
	cfg := h.store.Config()
	timeout := time.Duration(cfg.HealthCheck.TimeoutSec) * time.Second
	if timeout < time.Second {
		timeout = 4 * time.Second
	}

	var wg sync.WaitGroup
	for _, srv := range servers {
		if !srv.Enabled {
			continue
		}
		wg.Add(1)
		go func(s config.Server) {
			defer wg.Done()
			h.checkOne(s, timeout, cfg.HealthCheck.Failures)
		}(srv)
	}
	wg.Wait()
}

func (h *HealthChecker) checkOne(srv config.Server, timeout time.Duration, maxFails int) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	prev := h.store.MetricsFor(srv.ID)
	if prev == nil {
		// First time seeing this server. Mark it as "checking" so the traffic
		// loop won't generate fake traffic before we've verified it.
		prev = &store.ServerMetrics{ServerID: srv.ID, Status: "checking"}
	}

	addr := fmt.Sprintf("%s:%d", srv.Address, srv.Port)
	start := time.Now()
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	pingMs := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		prev.FailCount++
		prev.PingMs = 9999
		prev.LastChecked = time.Now()
		// On any failure, immediately drop traffic generation for this server
		// (set SpeedMbps to 0 and mark non-online status). After `maxFails`
		// consecutive failures, escalate to "offline" with an alert.
		prev.SpeedMbps = 0
		prev.Connections = 0
		if prev.FailCount >= maxFails {
			if prev.Status != "offline" {
				h.store.AddAlert(store.Alert{
					Level:    "crit",
					Title:    fmt.Sprintf("سرور %s آفلاین شد", srv.Name),
					Message:  fmt.Sprintf("%d health-check پیاپی ناموفق", prev.FailCount),
					ServerID: srv.ID,
				})
				h.store.AddLog("error", "healthcheck", fmt.Sprintf("server %s offline: %v", srv.Name, err))
			}
			prev.Status = "offline"
			prev.Score = 0
		} else {
			// Not yet "offline" but unreachable right now — use a status the
			// traffic loop won't accept as healthy.
			prev.Status = "checking"
		}
		h.store.SetMetrics(srv.ID, prev)
		return
	}
	_ = conn.Close()

	// recovered from offline
	wasOffline := prev.Status == "offline"
	prev.FailCount = 0
	prev.PingMs = pingMs
	prev.LastChecked = time.Now()

	if pingMs < 80 {
		prev.Status = "online"
	} else if pingMs < 200 {
		prev.Status = "online"
	} else {
		prev.Status = "degraded"
		h.store.AddAlert(store.Alert{
			Level:    "warn",
			Title:    fmt.Sprintf("پینگ %s بالا رفت", srv.Name),
			Message:  fmt.Sprintf("میانگین: %.0fms", pingMs),
			ServerID: srv.ID,
		})
	}

	prev.Score = computeScore(prev.PingMs, prev.SpeedMbps, prev.FailCount)

	if prev.UpSince.IsZero() || wasOffline {
		prev.UpSince = time.Now()
	}

	if wasOffline {
		h.store.AddAlert(store.Alert{
			Level:    "info",
			Title:    fmt.Sprintf("سرور %s ریکاوری شد", srv.Name),
			Message:  "به pool برگشت",
			ServerID: srv.ID,
		})
	}

	h.store.SetMetrics(srv.ID, prev)
}

// computeScore maps ping/speed/failures into a 0-100 score.
func computeScore(pingMs, speedMbps float64, fails int) int {
	pingScore := 100.0 - pingMs/3.0 // 0ms => 100, 300ms => 0
	if pingScore < 0 {
		pingScore = 0
	}
	if pingScore > 100 {
		pingScore = 100
	}
	speedScore := speedMbps * 2
	if speedScore > 100 {
		speedScore = 100
	}
	if speedMbps == 0 {
		speedScore = 50 // unknown speed, neutral
	}
	score := 0.7*pingScore + 0.3*speedScore - float64(fails*10)
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return int(score)
}
