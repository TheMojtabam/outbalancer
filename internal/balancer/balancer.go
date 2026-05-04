package balancer

import (
	"math/rand"
	"sync"
	"time"

	"github.com/outbalancer/outbalancer/internal/store"
)

// Balancer ties together health checks and traffic sampling.
//
// Traffic samples come from one of two sources:
//
//  1. **Real mode (default):** Throughput numbers come from xray-core's stats
//     API. When xray-core is not running (no proxy active yet), traffic is
//     reported as 0 — we never invent numbers that didn't happen on the wire.
//
//  2. **Demo mode (explicit `--demo` flag only):** A synthetic 1 Hz feed is
//     produced based on each server's score & weight, so reviewers can see
//     the panel populated with realistic-looking data without any real proxy.
// Reapplier is anything that can re-render & restart xray with the latest
// metrics — typically the xray.Manager. We accept it as an interface to avoid
// a package-level import cycle.
type Reapplier interface {
	Apply() error
}

type Balancer struct {
	store     *store.Store
	checker   *HealthChecker
	stop      chan struct{}
	wg        sync.WaitGroup
	rng       *rand.Rand
	demo      bool
	xray      Reapplier // optional; when set, reapplied after ping rounds
	lastApply time.Time
}

// New constructs a real-mode Balancer (no synthetic traffic).
func New(s *store.Store) *Balancer {
	return &Balancer{
		store:   s,
		checker: NewHealthChecker(s),
		stop:    make(chan struct{}),
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
		demo:    false,
	}
}

// SetXray attaches the xray manager so the balancer can reapply config after
// ping samples update (so weighted-random reflects real latency).
func (b *Balancer) SetXray(x Reapplier) { b.xray = x }

// SetDemoMode enables synthetic traffic generation. Call this only from the
// demo command path; never from normal startup.
func (b *Balancer) SetDemoMode(on bool) { b.demo = on }

// Start begins all background workers.
func (b *Balancer) Start() {
	b.checker.Start()
	b.wg.Add(1)
	go b.trafficLoop()
	if !b.demo {
		// Periodic reapply: every 60s, re-render the xray config so the
		// weighted-random strategy gets fresh per-outbound weights computed
		// from the latest TCP-ping measurements. (No-op if xray isn't
		// attached or no servers are configured yet.)
		b.wg.Add(1)
		go b.reapplyLoop()
	}
	if b.demo {
		b.store.AddLog("info", "balancer", "بالانسر در حالت دمو شروع به کار کرد")
	} else {
		b.store.AddLog("info", "balancer", "بالانسر شروع به کار کرد (واقعی - منتظر آمار از xray)")
	}
}

// reapplyLoop periodically reapplies xray config so the weighted-random
// strategy reflects the most recent TCP-ping samples. Skipped in demo mode.
func (b *Balancer) reapplyLoop() {
	defer b.wg.Done()
	// First reapply ~20s after startup, just after the initial healthcheck
	// round completes, then every 60s.
	first := time.NewTimer(20 * time.Second)
	defer first.Stop()
	select {
	case <-b.stop:
		return
	case <-first.C:
	}
	b.maybeReapply()

	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-b.stop:
			return
		case <-t.C:
			b.maybeReapply()
		}
	}
}

func (b *Balancer) maybeReapply() {
	if b.xray == nil {
		return
	}
	if len(b.store.Servers()) == 0 {
		return
	}
	if err := b.xray.Apply(); err != nil {
		b.store.AddLog("warn", "balancer", "reapply xray با ping های جدید: "+err.Error())
		return
	}
	b.lastApply = time.Now()
}

// Stop signals all workers to stop and waits.
func (b *Balancer) Stop() {
	close(b.stop)
	b.checker.Stop()
	b.wg.Wait()
	b.store.AddLog("info", "balancer", "بالانسر متوقف شد")
}

func (b *Balancer) trafficLoop() {
	defer b.wg.Done()
	t := time.NewTicker(time.Second)
	defer t.Stop()

	for {
		select {
		case <-b.stop:
			return
		case <-t.C:
			b.sampleTraffic()
		}
	}
}

// sampleTraffic emits one TrafficSample. In real mode the sample contains
// zeros (until xray stats wiring is enabled, which is a future feature).
// In demo mode the sample is synthesized from each server's score & weight.
func (b *Balancer) sampleTraffic() {
	servers := b.store.Servers()
	metrics := b.store.Metrics()
	sample := store.TrafficSample{
		Time:      time.Now(),
		PerServer: make(map[string]float64),
	}

	if !b.demo {
		// Real mode: report zeros. Make sure no stale per-server speed
		// from a previous demo session lingers in the metrics.
		for _, srv := range servers {
			sample.PerServer[srv.ID] = 0
			if m := metrics[srv.ID]; m != nil && (m.SpeedMbps != 0 || m.Connections != 0) {
				m.SpeedMbps = 0
				m.Connections = 0
				b.store.SetMetrics(srv.ID, m)
			}
		}
		b.store.PushTraffic(sample)
		return
	}

	// Demo mode: synthesize a believable feed.
	totalUp, totalDown := 0.0, 0.0
	for _, srv := range servers {
		m := metrics[srv.ID]
		if !srv.Enabled || m == nil {
			sample.PerServer[srv.ID] = 0
			continue
		}
		if m.Status != "online" && m.Status != "degraded" {
			sample.PerServer[srv.ID] = 0
			continue
		}
		base := float64(m.Score) * srv.Weight * 0.8
		noise := b.rng.Float64()*30 - 15
		mbps := base + noise
		if mbps < 0 {
			mbps = 0
		}
		sample.PerServer[srv.ID] = mbps

		bytesPerSec := mbps * 125000 // Mbps -> bytes/sec
		downShare := bytesPerSec * 0.85
		upShare := bytesPerSec * 0.15
		m.DownBytes += int64(downShare)
		m.UpBytes += int64(upShare)
		m.UsageMonth += int64(downShare + upShare)
		m.Connections = 10 + b.rng.Intn(50) + int(srv.Weight*8)
		m.SpeedMbps = mbps
		b.store.SetMetrics(srv.ID, m)

		totalDown += downShare / 125000
		totalUp += upShare / 125000
	}

	sample.TotalUp = totalUp
	sample.TotalDown = totalDown
	b.store.PushTraffic(sample)
}
