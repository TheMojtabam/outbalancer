package balancer

import (
	"sort"
	"strings"
	"sync/atomic"

	"github.com/outbalancer/outbalancer/internal/config"
	"github.com/outbalancer/outbalancer/internal/store"
)

// Algorithm is a function that picks one server from a list given current metrics.
type Algorithm func(servers []config.Server, metrics map[string]*store.ServerMetrics) *config.Server

var rrCounter uint64

// Algorithms maps name -> implementation.
var Algorithms = map[string]Algorithm{
	"latency":      latencyBased,
	"speed":        speedBased,
	"weighted":     weightedRoundRobin,
	"leastconn":    leastConnections,
	"roundrobin":   roundRobin,
	"random":       randomPick,
}

// Pick selects a server using the named algorithm.
func Pick(name string, servers []config.Server, metrics map[string]*store.ServerMetrics) *config.Server {
	enabled := filterOnline(servers, metrics)
	if len(enabled) == 0 {
		return nil
	}
	algo, ok := Algorithms[strings.ToLower(name)]
	if !ok {
		algo = latencyBased
	}
	return algo(enabled, metrics)
}

func filterOnline(servers []config.Server, metrics map[string]*store.ServerMetrics) []config.Server {
	out := make([]config.Server, 0, len(servers))
	for _, s := range servers {
		if !s.Enabled {
			continue
		}
		m := metrics[s.ID]
		if m != nil && m.Status == "offline" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func latencyBased(servers []config.Server, metrics map[string]*store.ServerMetrics) *config.Server {
	type entry struct {
		s    config.Server
		ping float64
	}
	list := make([]entry, 0, len(servers))
	for _, s := range servers {
		ping := 999.0
		if m := metrics[s.ID]; m != nil && m.PingMs > 0 {
			ping = m.PingMs
		}
		list = append(list, entry{s, ping})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ping < list[j].ping })
	chosen := list[0].s
	return &chosen
}

func speedBased(servers []config.Server, metrics map[string]*store.ServerMetrics) *config.Server {
	type entry struct {
		s     config.Server
		speed float64
	}
	list := make([]entry, 0, len(servers))
	for _, s := range servers {
		speed := 0.0
		if m := metrics[s.ID]; m != nil {
			speed = m.SpeedMbps
		}
		list = append(list, entry{s, speed})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].speed > list[j].speed })
	chosen := list[0].s
	return &chosen
}

func weightedRoundRobin(servers []config.Server, metrics map[string]*store.ServerMetrics) *config.Server {
	total := 0.0
	for _, s := range servers {
		w := s.Weight
		if w <= 0 {
			w = 1
		}
		total += w
	}
	if total == 0 {
		return roundRobin(servers, metrics)
	}
	idx := float64(atomic.AddUint64(&rrCounter, 1)%1000) / 1000.0 * total
	acc := 0.0
	for _, s := range servers {
		w := s.Weight
		if w <= 0 {
			w = 1
		}
		acc += w
		if idx <= acc {
			ch := s
			return &ch
		}
	}
	ch := servers[len(servers)-1]
	return &ch
}

func leastConnections(servers []config.Server, metrics map[string]*store.ServerMetrics) *config.Server {
	type entry struct {
		s     config.Server
		conns int
	}
	list := make([]entry, 0, len(servers))
	for _, s := range servers {
		conns := 0
		if m := metrics[s.ID]; m != nil {
			conns = m.Connections
		}
		list = append(list, entry{s, conns})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].conns < list[j].conns })
	chosen := list[0].s
	return &chosen
}

func roundRobin(servers []config.Server, _ map[string]*store.ServerMetrics) *config.Server {
	idx := atomic.AddUint64(&rrCounter, 1) % uint64(len(servers))
	chosen := servers[idx]
	return &chosen
}

func randomPick(servers []config.Server, _ map[string]*store.ServerMetrics) *config.Server {
	idx := atomic.AddUint64(&rrCounter, 1) % uint64(len(servers))
	chosen := servers[idx]
	return &chosen
}

// AlgorithmList returns the user-facing algorithm options.
func AlgorithmList() []map[string]string {
	return []map[string]string{
		{"id": "latency", "name": "بر اساس کم‌ترین پینگ", "desc": "بهترین برای گیمینگ و کال"},
		{"id": "speed", "name": "بر اساس بیشترین سرعت", "desc": "بهترین برای استریم و دانلود"},
		{"id": "weighted", "name": "وزن‌دار (Weighted RR)", "desc": "تقسیم بر اساس وزن دستی هر سرور"},
		{"id": "leastconn", "name": "کم‌ترین کانکشن فعال", "desc": "تقسیم بر اساس بار فعلی"},
		{"id": "roundrobin", "name": "گردشی ساده (Round Robin)", "desc": "نوبتی به همه سرورها"},
		{"id": "random", "name": "تصادفی", "desc": "انتخاب رندوم"},
	}
}
