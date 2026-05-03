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
type Balancer struct {
	store   *store.Store
	checker *HealthChecker
	stop    chan struct{}
	wg      sync.WaitGroup
	rng     *rand.Rand
	demo    bool // when true, generate synthetic traffic. Default: false.
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

// SetDemoMode enables synthetic traffic generation. Call this only from the
// demo command path; never from normal startup.
func (b *Balancer) SetDemoMode(on bool) { b.demo = on }

// Start begins all background workers.
func (b *Balancer) Start() {
	b.checker.Start()
	b.wg.Add(1)
	go b.trafficLoop()
	if b.demo {
		b.store.AddLog("info", "balancer", "بالانسر در حالت دمو شروع به کار کرد")
	} else {
		b.store.AddLog("info", "balancer", "بالانسر شروع به کار کرد (واقعی - منتظر آمار از xray)")
	}
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
