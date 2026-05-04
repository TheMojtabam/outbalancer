package main

import (
	"time"

	"github.com/outbalancer/outbalancer/internal/config"
	"github.com/outbalancer/outbalancer/internal/store"
)

// seedDemo populates the store with a realistic-looking demo dataset
// so the panel has something to show on a fresh install.
func seedDemo(s *store.Store) {
	demoURLs := []string{
		"vless://11111111-1111-1111-1111-111111111111@frankfurt-01.example.com:443?type=tcp&security=reality&sni=cloudflare.com&pbk=demo&sid=01&fp=chrome#Germany-FRA-01",
		"vless://22222222-2222-2222-2222-222222222222@amsterdam.example.com:8443?type=ws&security=tls&sni=cdn.example.com&path=%2Fws&host=cdn.example.com#Netherlands-AMS-01",
		"vless://33333333-3333-3333-3333-333333333333@helsinki.example.com:443?type=tcp&security=reality&sni=microsoft.com&pbk=demo&sid=02#Finland-HEL-01",
		"vless://44444444-4444-4444-4444-444444444444@istanbul.example.com:2096?type=ws&security=tls&path=%2Fpath&host=cdn-tr.example.com#Turkey-IST-01",
		"vless://55555555-5555-5555-5555-555555555555@paris.example.com:443?type=tcp&security=reality&sni=apple.com&pbk=demo&sid=03#France-PAR-02",
		"vless://66666666-6666-6666-6666-666666666666@newyork.example.com:443?type=grpc&security=tls&sni=cdn-us.example.com&path=service#US-NY-01",
		"vless://77777777-7777-7777-7777-777777777777@stockholm.example.com:443?type=tcp&security=tls&sni=cdn-se.example.com#Sweden-STO-01",
		"vless://88888888-8888-8888-8888-888888888888@warsaw.example.com:443?type=ws&security=tls&path=%2Fproxy&host=cdn-pl.example.com#Poland-WAW-01",
		"vless://99999999-9999-9999-9999-999999999999@vienna.example.com:443?type=tcp&security=reality&sni=netflix.com&pbk=demo&sid=04#Austria-VIE-01",
		"vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@tokyo.example.com:8443?type=tcp&security=tls&sni=cdn-jp.example.com#Japan-TYO-01",
	}

	weights := []float64{7.0, 5.5, 4.0, 2.0, 0.0, 3.0, 4.5, 3.5, 4.0, 3.0}

	for i, url := range demoURLs {
		srv, err := config.ParseVless(url)
		if err != nil {
			continue
		}
		srv.Weight = weights[i]
		if i == 4 {
			srv.Enabled = true // PAR-02 will go offline naturally because hostname doesn't resolve
		}
		_ = s.AddServer(*srv)

		// seed metrics so the panel has something to show before first health check
		m := &store.ServerMetrics{
			ServerID:    srv.ID,
			Status:      "online",
			PingMs:      []float64{28, 35, 48, 187, 999, 128, 52, 64, 45, 95}[i],
			SpeedMbps:   []float64{284, 198, 156, 62, 0, 96, 134, 88, 142, 102}[i],
			Score:       []int{94, 88, 82, 58, 12, 74, 86, 78, 84, 72}[i],
			Connections: []int{142, 98, 76, 34, 0, 21, 52, 41, 58, 30}[i],
			UsageMonth:  int64(312+i*50) * 1024 * 1024 * 1024,
			UpSince:     time.Now().Add(-time.Duration(72-i*3) * time.Hour),
			LastChecked: time.Now(),
		}
		if i == 3 {
			m.Status = "degraded"
		}
		if i == 4 {
			m.Status = "offline"
			m.Score = 12
		}
		s.SetMetrics(srv.ID, m)
	}

	// seed routing rules
	cfg := s.Config()
	cfg.RoutingRules = []config.RoutingRule{
		{ID: "r_1", Name: "YouTube → پرسرعت", Domains: []string{"youtube.com", "googlevideo.com"}, Target: "balanced", Enabled: true},
		{ID: "r_2", Name: "Telegram → کم‌پینگ", Domains: []string{"telegram.org", "t.me"}, Target: "balanced", Enabled: true},
		{ID: "r_3", Name: "Iran → Direct", GeoIP: "ir", Target: "direct", Enabled: true},
		{ID: "r_4", Name: "Spotify → استریم", Domains: []string{"spotify.com", "scdn.co"}, Target: "balanced", Enabled: true},
	}
	cfg.DNS = config.DNSCfg{Servers: []string{"1.1.1.1", "8.8.8.8"}, LeakProtect: true, BlockMalware: false}
	_ = s.SaveConfig(cfg)

	// seed alerts
	s.AddAlert(store.Alert{Level: "crit", Title: "سرور France-PAR-02 آفلاین شد", Message: "3 health-check پیاپی ناموفق · از pool خارج شد"})
	s.AddAlert(store.Alert{Level: "warn", Title: "پینگ Turkey-IST بالا رفت", Message: "میانگین: 187ms (آستانه: 150ms)"})
	s.AddAlert(store.Alert{Level: "warn", Title: "سهمیه ماهانه US-NY: 80%", Message: "40GB از 50GB استفاده شده"})
	s.AddAlert(store.Alert{Level: "info", Title: "وزن DE-FRA به‌روز شد", Message: "auto-tune: 5 → 7 (بر اساس performance)"})

	// seed top-domains (demo only — int64 GiB constants are overflow-safe
	// even on 32-bit platforms because we cast each multiplier to int64 first).
	const gib int64 = 1024 * 1024 * 1024
	s.SetTopDomains([]store.DomainStat{
		{Domain: "youtube.com", Bytes: 412 * gib, Icon: "youtube", Color: "#ff0000"},
		{Domain: "spotify.com", Bytes: 187 * gib, Icon: "spotify", Color: "#1ed760"},
		{Domain: "telegram.org", Bytes: 94 * gib, Icon: "telegram", Color: "#0088cc"},
		{Domain: "netflix.com", Bytes: 61 * gib, Icon: "film", Color: "#e50914"},
		{Domain: "github.com", Bytes: 38 * gib, Icon: "github", Color: "#aaaaaa"},
		{Domain: "openai.com", Bytes: 22 * gib, Icon: "cloud", Color: "#10a37f"},
	})

	s.AddLog("info", "balancer", "حالت دمو فعال شد - 10 سرور نمونه اضافه شد")
}
