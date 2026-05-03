// genshots is a small helper that emits SVG screenshots for each panel page.
// Run with: go run scripts/genshots/main.go docs/images/
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	W, H = 1280, 800
)

// Common visual primitives — the SVGs use the same color system as the panel.
const styleDefs = `
<defs>
  <linearGradient id="bg" x1="0" x2="1" y1="0" y2="1">
    <stop offset="0" stop-color="#05060d"/>
    <stop offset="1" stop-color="#0a0d1a"/>
  </linearGradient>
  <radialGradient id="glow1" cx="0.85" cy="-0.1" r="0.8">
    <stop offset="0" stop-color="#7c5cff" stop-opacity="0.18"/>
    <stop offset="1" stop-color="#7c5cff" stop-opacity="0"/>
  </radialGradient>
  <radialGradient id="glow2" cx="0.1" cy="1.1" r="0.8">
    <stop offset="0" stop-color="#00ffd1" stop-opacity="0.12"/>
    <stop offset="1" stop-color="#00ffd1" stop-opacity="0"/>
  </radialGradient>
  <linearGradient id="card" x1="0" x2="0" y1="0" y2="1">
    <stop offset="0" stop-color="#ffffff" stop-opacity="0.07"/>
    <stop offset="1" stop-color="#ffffff" stop-opacity="0.04"/>
  </linearGradient>
  <linearGradient id="accent" x1="0" x2="1" y1="0" y2="0">
    <stop offset="0" stop-color="#00ffd1"/>
    <stop offset="1" stop-color="#7c5cff"/>
  </linearGradient>
  <linearGradient id="primary" x1="0" x2="1" y1="0" y2="0">
    <stop offset="0" stop-color="#00ffd1"/>
    <stop offset="1" stop-color="#00cfa6"/>
  </linearGradient>
  <linearGradient id="brand" x1="0" x2="1" y1="0" y2="1">
    <stop offset="0" stop-color="#00ffd1"/>
    <stop offset="0.5" stop-color="#7c5cff"/>
    <stop offset="1" stop-color="#ff5c87"/>
  </linearGradient>
  <linearGradient id="trafficGreen" x1="0" x2="0" y1="0" y2="1">
    <stop offset="0" stop-color="#00ffd1" stop-opacity="0.4"/>
    <stop offset="1" stop-color="#00ffd1" stop-opacity="0"/>
  </linearGradient>
  <linearGradient id="trafficPurple" x1="0" x2="0" y1="0" y2="1">
    <stop offset="0" stop-color="#7c5cff" stop-opacity="0.35"/>
    <stop offset="1" stop-color="#7c5cff" stop-opacity="0"/>
  </linearGradient>
  <linearGradient id="trafficPink" x1="0" x2="0" y1="0" y2="1">
    <stop offset="0" stop-color="#ff5c87" stop-opacity="0.3"/>
    <stop offset="1" stop-color="#ff5c87" stop-opacity="0"/>
  </linearGradient>
  <pattern id="grid" width="56" height="56" patternUnits="userSpaceOnUse">
    <path d="M 56 0 L 0 0 0 56" fill="none" stroke="#ffffff" stroke-opacity="0.018" stroke-width="1"/>
  </pattern>
</defs>
`

// background fills the SVG with the panel's atmospheric backdrop.
func background() string {
	return `<rect width="100%" height="100%" fill="url(#bg)"/>
<rect width="100%" height="100%" fill="url(#glow1)"/>
<rect width="100%" height="100%" fill="url(#glow2)"/>
<rect width="100%" height="100%" fill="url(#grid)"/>`
}

// sidebar emits the fixed sidebar with the active item highlighted.
// activeRoute should match one of: dashboard, stats, heatmap, servers, routing,
// algorithm, speed, profiles, schedule, alerts, dns, api, logs, settings, backup.
func sidebar(activeRoute string) string {
	type navItem struct {
		Route, Icon, Label, Badge string
		Group                     string
	}
	items := []navItem{
		{"dashboard", "▦", "نمای کلی", "", "DASHBOARD"},
		{"stats", "📈", "آمار زنده", "LIVE", "DASHBOARD"},
		{"heatmap", "🔥", "هیت‌مپ مصرف", "", "DASHBOARD"},
		{"servers", "🗄", "سرورها", "10", "CONFIG"},
		{"routing", "🛣", "قوانین مسیریابی", "", "CONFIG"},
		{"algorithm", "⇅", "الگوریتم بالانس", "", "CONFIG"},
		{"speed", "🚀", "Speed Boost", "★", "CONFIG"},
		{"profiles", "❍", "پروفایل‌ها", "", "CONFIG"},
		{"schedule", "⌚", "زمانبندی", "", "CONFIG"},
		{"alerts", "🔔", "هشدارها", "3", "ADV"},
		{"dns", "🛡", "DNS هوشمند", "", "ADV"},
		{"api", "⌥", "API & Webhooks", "", "ADV"},
		{"logs", "≡", "لاگ‌ها", "", "ADV"},
		{"settings", "⚙", "تنظیمات", "", "SYSTEM"},
		{"backup", "☁", "پشتیبان‌گیری", "", "SYSTEM"},
	}

	groupLabels := map[string]string{
		"DASHBOARD": "— داشبورد",
		"CONFIG":    "— پیکربندی",
		"ADV":       "— پیشرفته",
		"SYSTEM":    "— سیستم",
	}

	var b strings.Builder
	// sidebar background
	b.WriteString(`<rect x="1000" y="0" width="280" height="800" fill="#0d1124" opacity="0.85"/>
<line x1="1000" y1="0" x2="1000" y2="800" stroke="#ffffff" stroke-opacity="0.08" stroke-width="1"/>`)

	// brand
	b.WriteString(`
<g transform="translate(1020,30)">
  <rect width="44" height="44" rx="12" fill="url(#brand)"/>
  <rect x="3" y="3" width="38" height="38" rx="9" fill="#0a0d1a"/>
  <text x="22" y="30" text-anchor="middle" font-size="20" fill="#00ffd1" font-family="Vazirmatn, sans-serif">⚡</text>
  <text x="60" y="22" font-family="Syne, sans-serif" font-size="17" font-weight="800" fill="#fff">OutBalancer</text>
  <text x="60" y="36" font-family="JetBrains Mono, monospace" font-size="9" fill="#5e6585" letter-spacing="2">v0.1.0 · CORE</text>
</g>
<line x1="1020" y1="92" x2="1260" y2="92" stroke="#ffffff" stroke-opacity="0.08"/>`)

	// nav groups
	y := 110
	curGroup := ""
	for _, it := range items {
		if it.Group != curGroup {
			curGroup = it.Group
			b.WriteString(fmt.Sprintf(`
<text x="1240" y="%d" text-anchor="end" font-family="JetBrains Mono, monospace" font-size="9" fill="#5e6585" letter-spacing="2">%s</text>`, y+10, groupLabels[curGroup]))
			y += 22
		}

		isActive := it.Route == activeRoute
		fill := "transparent"
		stroke := "transparent"
		textColor := "#a8b0c9"
		if isActive {
			fill = "rgba(0,255,209,0.1)"
			stroke = "rgba(0,255,209,0.18)"
			textColor = "#f4f6ff"
			// glow indicator
			b.WriteString(fmt.Sprintf(`<rect x="1257" y="%d" width="3" height="22" rx="2" fill="#00ffd1"/>`, y+5))
		}
		b.WriteString(fmt.Sprintf(`
<g>
  <rect x="1020" y="%d" width="220" height="32" rx="11" fill="%s" stroke="%s"/>
  <text x="1228" y="%d" text-anchor="end" font-family="Vazirmatn, sans-serif" font-size="13" font-weight="500" fill="%s" direction="rtl">%s %s</text>`,
			y, fill, stroke, y+20, textColor, it.Label, it.Icon))
		if it.Badge != "" {
			badgeColor := "#00ffd1"
			if it.Badge == "★" {
				badgeColor = "#ff5c87"
			}
			b.WriteString(fmt.Sprintf(`
  <rect x="1030" y="%d" width="34" height="18" rx="6" fill="%s" fill-opacity="0.15"/>
  <text x="1047" y="%d" text-anchor="middle" font-family="JetBrains Mono, monospace" font-size="9" fill="%s" font-weight="700">%s</text>`,
				y+7, badgeColor, y+19, badgeColor, it.Badge))
		}
		b.WriteString(`</g>`)
		y += 36
	}

	// user footer
	b.WriteString(`
<g transform="translate(1020,720)">
  <rect width="220" height="58" rx="14" fill="#ffffff" fill-opacity="0.04" stroke="#ffffff" stroke-opacity="0.08"/>
  <circle cx="36" cy="29" r="18" fill="url(#brand)"/>
  <text x="36" y="35" text-anchor="middle" font-family="Vazirmatn" font-size="14" font-weight="700" fill="#fff">A</text>
  <text x="60" y="26" font-family="Vazirmatn" font-size="13" font-weight="600" fill="#f4f6ff">Admin</text>
  <text x="60" y="40" font-family="JetBrains Mono" font-size="10" fill="#5e6585">SUPERUSER</text>
  <circle cx="200" cy="30" r="4" fill="#00ffa3" filter="url(#glow)"/>
</g>`)
	return b.String()
}

// topbar emits the title bar with page title and search/actions.
func topbar(title, subtitle string) string {
	parts := strings.Fields(title)
	first := strings.Join(parts[:len(parts)-1], " ")
	last := parts[len(parts)-1]
	return fmt.Sprintf(`
<g transform="translate(40,30)">
  <text x="920" y="28" text-anchor="end" font-family="Syne, sans-serif" font-size="28" font-weight="700" fill="#fff" direction="rtl">%s <tspan fill="url(#accent)">%s</tspan></text>
  <text x="920" y="50" text-anchor="end" font-family="JetBrains Mono, monospace" font-size="11" fill="#5e6585" direction="rtl">%s</text>

  <rect x="120" y="14" width="240" height="38" rx="12" fill="#ffffff" fill-opacity="0.04" stroke="#ffffff" stroke-opacity="0.08"/>
  <text x="135" y="38" font-family="Vazirmatn, sans-serif" font-size="14" fill="#5e6585">⌕</text>
  <text x="160" y="38" font-family="Vazirmatn, sans-serif" font-size="12" fill="#5e6585">جستجو...</text>
  <rect x="320" y="22" width="32" height="20" rx="6" fill="#ffffff" fill-opacity="0.05" stroke="#ffffff" stroke-opacity="0.08"/>
  <text x="336" y="36" text-anchor="middle" font-family="JetBrains Mono" font-size="9" fill="#5e6585">⌘K</text>

  <rect x="380" y="14" width="38" height="38" rx="12" fill="#ffffff" fill-opacity="0.04" stroke="#ffffff" stroke-opacity="0.08"/>
  <text x="399" y="38" text-anchor="middle" font-size="14">🔔</text>
  <circle cx="411" cy="22" r="4" fill="#ff5c87"/>

  <rect x="430" y="14" width="38" height="38" rx="12" fill="#ffffff" fill-opacity="0.04" stroke="#ffffff" stroke-opacity="0.08"/>
  <text x="449" y="38" text-anchor="middle" font-size="14">🌙</text>

  <rect x="478" y="14" width="170" height="38" rx="12" fill="url(#primary)"/>
  <text x="563" y="38" text-anchor="middle" font-family="Vazirmatn, sans-serif" font-size="13" font-weight="700" fill="#001a14" direction="rtl">+ کانفیگ جدید</text>
</g>`, first, last, subtitle)
}

// statusbar emits the bottom status bar.
func statusbar(extra string) string {
	if extra == "" {
		extra = "Goroutines: 24 · RAM: 184MB · SOCKS: :10808 · HTTP: :10809"
	}
	return fmt.Sprintf(`
<g transform="translate(40,750)">
  <rect width="920" height="36" rx="12" fill="#ffffff" fill-opacity="0.04" stroke="#ffffff" stroke-opacity="0.08"/>
  <text x="20" y="23" font-family="JetBrains Mono, monospace" font-size="10" fill="#a8b0c9">✓ Core: Xray v25.3.6 · %s</text>
  <text x="900" y="23" text-anchor="end" font-family="JetBrains Mono, monospace" font-size="10" fill="#00ffa3">● healthy</text>
</g>`, extra)
}

// kpiCard emits a KPI card.
func kpiCard(x, y int, icon, value, unit, label, trend, trendType string) string {
	trendColor := "#00ffa3"
	trendBg := "rgba(0,255,163,0.12)"
	if trendType == "down" {
		trendColor = "#ff4d6d"
		trendBg = "rgba(255,77,109,0.12)"
	} else if trendType == "neutral" {
		trendColor = "#a8b0c9"
		trendBg = "rgba(255,255,255,0.06)"
	}
	return fmt.Sprintf(`
<g transform="translate(%d,%d)">
  <rect width="220" height="140" rx="20" fill="url(#card)" stroke="#ffffff" stroke-opacity="0.08"/>
  <rect x="20" y="22" width="44" height="44" rx="14" fill="#ffffff" fill-opacity="0.08" stroke="#ffffff" stroke-opacity="0.14"/>
  <text x="42" y="50" text-anchor="middle" font-size="20" fill="#00ffd1">%s</text>
  <rect x="155" y="28" width="50" height="22" rx="8" fill="%s"/>
  <text x="180" y="44" text-anchor="middle" font-family="JetBrains Mono" font-size="10" font-weight="700" fill="%s">%s</text>
  <text x="200" y="98" text-anchor="end" font-family="Syne, sans-serif" font-size="28" font-weight="700" fill="#f4f6ff">%s<tspan font-size="14" fill="#5e6585"> %s</tspan></text>
  <text x="200" y="120" text-anchor="end" font-family="JetBrains Mono, monospace" font-size="10" fill="#5e6585" letter-spacing="1">%s</text>
</g>`, x, y, icon, trendBg, trendColor, trend, value, unit, label)
}

// panel emits the outer panel rectangle and title.
func panelBox(x, y, w, h int, title, sub string) string {
	return fmt.Sprintf(`
<rect x="%d" y="%d" width="%d" height="%d" rx="20" fill="url(#card)" stroke="#ffffff" stroke-opacity="0.08"/>
<text x="%d" y="%d" text-anchor="end" font-family="Syne, sans-serif" font-size="15" font-weight="700" fill="#f4f6ff" direction="rtl">%s</text>
<text x="%d" y="%d" text-anchor="end" font-family="JetBrains Mono, monospace" font-size="10" fill="#5e6585" direction="rtl">%s</text>`,
		x, y, w, h, x+w-22, y+30, title, x+w-22, y+46, sub)
}

// =============================================================================
// PAGE: DASHBOARD
// =============================================================================
func dashboardPage() string {
	var b strings.Builder
	b.WriteString(svgOpen())
	b.WriteString(background())
	b.WriteString(sidebar("dashboard"))
	b.WriteString(topbar("نمای کلی", "// real-time balancer telemetry · uptime 14d 06h"))

	// KPI row
	b.WriteString(kpiCard(40, 100, "⚡", "847", "Mbps", "throughput فعلی", "↑ live", "up"))
	b.WriteString(kpiCard(280, 100, "⏱", "42", "ms", "پینگ میانگین", "↓ 8ms", "up"))
	b.WriteString(kpiCard(520, 100, "🗄", "9", "active", "سرورهای آنلاین", "9/10", "up"))
	b.WriteString(kpiCard(760, 100, "💾", "1.4", "TB", "مصرف ماهانه", "↑ 2.1%", "neutral"))

	// Alerts widget (left)
	b.WriteString(panelBox(40, 260, 460, 240, "آخرین هشدارها", "// ۴ مورد · جدیدترین"))
	alerts := []struct{ Icon, Title, Body, Color string }{
		{"⚠", "سرور France-PAR-02 آفلاین شد", "3 health-check ناموفق · از pool خارج", "#ff4d6d"},
		{"⏱", "پینگ Turkey-IST بالا رفت", "میانگین: 187ms (آستانه: 150ms)", "#ffb547"},
		{"💾", "سهمیه ماهانه US-NY: 80%", "40GB از 50GB استفاده شده", "#ffb547"},
		{"ℹ", "وزن DE-FRA به‌روز شد", "auto-tune: 5 → 7 (بر اساس performance)", "#00ffd1"},
	}
	for i, a := range alerts {
		yy := 305 + i*42
		b.WriteString(fmt.Sprintf(`
<rect x="60" y="%d" width="28" height="28" rx="8" fill="%s" fill-opacity="0.12"/>
<text x="74" y="%d" text-anchor="middle" font-size="13" fill="%s">%s</text>
<text x="475" y="%d" text-anchor="end" font-family="Vazirmatn" font-size="12" font-weight="600" fill="#f4f6ff" direction="rtl">%s</text>
<text x="475" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="9" fill="#5e6585" direction="rtl">%s</text>`,
			yy, a.Color, yy+19, a.Color, a.Icon, yy+13, a.Title, yy+27, a.Body))
	}

	// Status widget (right)
	b.WriteString(panelBox(520, 260, 440, 240, "وضعیت بالانسر", "// configuration snapshot"))
	rows := []struct{ Label, Value, Color string }{
		{"الگوریتم فعال", "بر اساس کم‌ترین پینگ", "#00ffd1"},
		{"سرورها", "9 / 10 آنلاین", "#f4f6ff"},
		{"Xray Core", "✓ فعال", "#00ffa3"},
	}
	for i, r := range rows {
		yy := 320 + i*44
		b.WriteString(fmt.Sprintf(`
<rect x="540" y="%d" width="400" height="34" rx="10" fill="#ffffff" fill-opacity="0.04"/>
<text x="555" y="%d" font-family="Vazirmatn" font-size="11" fill="#5e6585" direction="rtl">%s</text>
<text x="925" y="%d" text-anchor="end" font-family="Vazirmatn" font-size="13" font-weight="600" fill="%s" direction="rtl">%s</text>`,
			yy, yy+20, r.Label, yy+20, r.Color, r.Value))
	}
	b.WriteString(`
<rect x="540" y="450" width="400" height="36" rx="12" fill="url(#primary)"/>
<text x="740" y="473" text-anchor="middle" font-family="Vazirmatn" font-size="13" font-weight="700" fill="#001a14" direction="rtl">📈 مشاهده آمار زنده</text>`)

	// Quick links
	b.WriteString(panelBox(40, 520, 920, 220, "دسترسی سریع", "// shortcuts"))
	links := []struct{ Icon, Title, Sub string }{
		{"🗄", "مدیریت سرورها", "افزودن، ویرایش و حذف"},
		{"🛣", "قوانین مسیریابی", "هدایت دامنه‌ها"},
		{"⇅", "الگوریتم بالانس", "انتخاب از ۶ روش"},
		{"🔥", "هیت‌مپ مصرف", "تحلیل ساعتی"},
		{"🛡", "DNS هوشمند", "حفاظت از نشتی"},
		{"☁", "پشتیبان‌گیری", "import / export"},
	}
	for i, l := range links {
		col := i % 3
		row := i / 3
		x := 60 + col*295
		y := 580 + row*70
		b.WriteString(fmt.Sprintf(`
<rect x="%d" y="%d" width="285" height="60" rx="14" fill="#ffffff" fill-opacity="0.04" stroke="#ffffff" stroke-opacity="0.08"/>
<rect x="%d" y="%d" width="32" height="32" rx="10" fill="#00ffd1" fill-opacity="0.08"/>
<text x="%d" y="%d" text-anchor="middle" font-size="14" fill="#00ffd1">%s</text>
<text x="%d" y="%d" text-anchor="end" font-family="Syne" font-size="13" font-weight="700" fill="#f4f6ff" direction="rtl">%s</text>
<text x="%d" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="10" fill="#5e6585" direction="rtl">%s</text>`,
			x, y, x+15, y+14, x+31, y+33, l.Icon, x+265, y+25, l.Title, x+265, y+44, l.Sub))
	}

	b.WriteString(statusbar(""))
	b.WriteString(svgClose())
	return b.String()
}

// =============================================================================
// PAGE: SERVERS
// =============================================================================
func serversPage() string {
	var b strings.Builder
	b.WriteString(svgOpen())
	b.WriteString(background())
	b.WriteString(sidebar("servers"))
	b.WriteString(topbar("سرورها", "// مدیریت کانفیگ‌ها · ۱۰ سرور · drag to reorder"))

	// table panel
	b.WriteString(panelBox(40, 100, 920, 640, "سرورهای متصل", "// click on a row for detail"))

	// chips
	chips := []struct {
		Label  string
		Active bool
	}{
		{"همه", true}, {"آنلاین", false}, {"degraded", false}, {"آفلاین", false},
		{"Test All", false}, {"Bulk Import", false}, {"+ سرور جدید", false},
	}
	cx := 60
	for _, c := range chips {
		w := len(c.Label)*7 + 24
		fill := "rgba(255,255,255,0.04)"
		stroke := "rgba(255,255,255,0.08)"
		color := "#a8b0c9"
		if c.Active {
			fill = "rgba(0,255,209,0.08)"
			stroke = "rgba(0,255,209,0.3)"
			color = "#00ffd1"
		}
		b.WriteString(fmt.Sprintf(`
<rect x="%d" y="115" width="%d" height="26" rx="10" fill="%s" stroke="%s"/>
<text x="%d" y="132" text-anchor="middle" font-family="JetBrains Mono" font-size="10" fill="%s" font-weight="600">%s</text>`,
			cx, w, fill, stroke, cx+w/2, color, c.Label))
		cx += w + 8
	}

	// table header
	headers := []struct {
		X     int
		Label string
	}{
		{60, "سرور"}, {350, "وضعیت"}, {450, "پینگ"}, {580, "سرعت"},
		{670, "کانکشن"}, {760, "وزن"}, {820, "نمره"}, {900, "عملیات"},
	}
	for _, h := range headers {
		b.WriteString(fmt.Sprintf(`<text x="%d" y="180" font-family="JetBrains Mono" font-size="9" fill="#5e6585" letter-spacing="1.5" text-anchor="end">%s</text>`, h.X+30, h.Label))
	}
	b.WriteString(`<line x1="60" y1="190" x2="940" y2="190" stroke="#ffffff" stroke-opacity="0.08"/>`)

	// rows
	type srv struct {
		Flag, Name, Url, Status, StatusColor string
		Ping, Speed, Conn, Score             string
		PingPct, PingClass                   string
		Weight                               string
	}
	rows := []srv{
		{"🇩🇪", "Germany-FRA-01", "vless://...frankfurt:443", "ONLINE", "#00ffa3", "28ms", "284", "142", "94", "18", "good", "7.0"},
		{"🇳🇱", "Netherlands-AMS-01", "vless://...amsterdam:8443", "ONLINE", "#00ffa3", "35ms", "198", "98", "88", "24", "good", "5.5"},
		{"🇫🇮", "Finland-HEL-01", "vless://...helsinki:443", "ONLINE", "#00ffa3", "48ms", "156", "76", "82", "32", "good", "4.0"},
		{"🇹🇷", "Turkey-IST-01", "vless://...istanbul:2096", "DEGRADED", "#ffb547", "187ms", "62", "34", "58", "78", "med", "2.0"},
		{"🇫🇷", "France-PAR-02", "vless://...paris:443", "OFFLINE", "#ff4d6d", "timeout", "—", "0", "12", "100", "bad", "0.0"},
		{"🇺🇸", "US-NY-01", "vless://...newyork:443", "ONLINE", "#00ffa3", "128ms", "96", "21", "74", "56", "med", "3.0"},
		{"🇸🇪", "Sweden-STO-01", "vless://...stockholm:443", "ONLINE", "#00ffa3", "52ms", "134", "52", "86", "34", "good", "4.5"},
		{"🇵🇱", "Poland-WAW-01", "vless://...warsaw:443", "ONLINE", "#00ffa3", "64ms", "88", "41", "78", "42", "good", "3.5"},
		{"🇦🇹", "Austria-VIE-01", "vless://...vienna:443", "ONLINE", "#00ffa3", "45ms", "142", "58", "84", "30", "good", "4.0"},
		{"🇯🇵", "Japan-TYO-01", "vless://...tokyo:8443", "ONLINE", "#00ffa3", "95ms", "102", "30", "72", "48", "med", "3.0"},
	}
	for i, r := range rows {
		y := 220 + i*48
		// flag
		b.WriteString(fmt.Sprintf(`
<rect x="60" y="%d" width="34" height="34" rx="9" fill="#ffffff" fill-opacity="0.06" stroke="#ffffff" stroke-opacity="0.14"/>
<text x="77" y="%d" text-anchor="middle" font-size="16">%s</text>
<text x="380" y="%d" text-anchor="end" font-family="Vazirmatn" font-size="13" font-weight="600" fill="#f4f6ff">%s</text>
<text x="380" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="10" fill="#5e6585">%s</text>`,
			y, y+22, r.Flag, y+13, r.Name, y+27, r.Url))
		// status
		b.WriteString(fmt.Sprintf(`
<rect x="395" y="%d" width="76" height="22" rx="8" fill="%s" fill-opacity="0.12"/>
<circle cx="408" cy="%d" r="3" fill="%s"/>
<text x="465" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="9" font-weight="700" fill="%s">%s</text>`,
			y+8, r.StatusColor, y+19, r.StatusColor, y+22, r.StatusColor, r.Status))
		// ping bar
		barColor := "#00ffa3"
		if r.PingClass == "med" {
			barColor = "#ffb547"
		} else if r.PingClass == "bad" {
			barColor = "#ff4d6d"
		}
		barWidth := 60
		pct := 18 // default
		fmt.Sscanf(r.PingPct, "%d", &pct)
		fillW := pct * barWidth / 100
		b.WriteString(fmt.Sprintf(`
<rect x="490" y="%d" width="%d" height="5" rx="2" fill="#ffffff" fill-opacity="0.06"/>
<rect x="490" y="%d" width="%d" height="5" rx="2" fill="%s"/>
<text x="610" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="11" font-weight="600" fill="%s">%s</text>`,
			y+15, barWidth, y+15, fillW, barColor, y+19, barColor, r.Ping))
		// speed
		b.WriteString(fmt.Sprintf(`<text x="685" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="11" font-weight="600" fill="#f4f6ff">%s Mbps</text>`, y+19, r.Speed))
		// connections
		b.WriteString(fmt.Sprintf(`<text x="755" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="11" fill="#a8b0c9">%s</text>`, y+19, r.Conn))
		// weight
		b.WriteString(fmt.Sprintf(`
<rect x="785" y="%d" width="40" height="20" rx="6" fill="#ffffff" fill-opacity="0.05"/>
<text x="805" y="%d" text-anchor="middle" font-family="JetBrains Mono" font-size="10" font-weight="600" fill="#a8b0c9">%s</text>`,
			y+10, y+24, r.Weight))
		// score ring
		scoreColor := "#00ffa3"
		if r.PingClass == "med" {
			scoreColor = "#ffb547"
		} else if r.PingClass == "bad" {
			scoreColor = "#ff4d6d"
		}
		scoreNum := 50
		fmt.Sscanf(r.Score, "%d", &scoreNum)
		dash := scoreNum
		b.WriteString(fmt.Sprintf(`
<g transform="translate(845,%d)">
  <circle cx="14" cy="14" r="11" fill="none" stroke="#ffffff" stroke-opacity="0.08" stroke-width="2.5"/>
  <circle cx="14" cy="14" r="11" fill="none" stroke="%s" stroke-width="2.5" stroke-linecap="round"
          stroke-dasharray="%d 100" transform="rotate(-90 14 14)"/>
  <text x="14" y="18" text-anchor="middle" font-family="JetBrains Mono" font-size="9" font-weight="700" fill="%s">%d</text>
</g>`, y+8, scoreColor, dash, scoreColor, scoreNum))
		// row actions
		b.WriteString(fmt.Sprintf(`
<rect x="885" y="%d" width="22" height="22" rx="6" fill="#ffffff" fill-opacity="0.05" stroke="#ffffff" stroke-opacity="0.08"/>
<text x="896" y="%d" text-anchor="middle" font-size="10" fill="#a8b0c9">⚡</text>
<rect x="912" y="%d" width="22" height="22" rx="6" fill="#ffffff" fill-opacity="0.05" stroke="#ffffff" stroke-opacity="0.08"/>
<text x="923" y="%d" text-anchor="middle" font-size="10" fill="#a8b0c9">✎</text>`,
			y+9, y+23, y+9, y+23))
		// separator
		if i < len(rows)-1 {
			b.WriteString(fmt.Sprintf(`<line x1="60" y1="%d" x2="940" y2="%d" stroke="#ffffff" stroke-opacity="0.04"/>`, y+44, y+44))
		}
	}

	b.WriteString(statusbar(""))
	b.WriteString(svgClose())
	return b.String()
}

// =============================================================================
// PAGE: STATS (live traffic chart)
// =============================================================================
func statsPage() string {
	var b strings.Builder
	b.WriteString(svgOpen())
	b.WriteString(background())
	b.WriteString(sidebar("stats"))
	b.WriteString(topbar("آمار زنده", "// 60s rolling window · 1Hz refresh"))

	// big chart
	b.WriteString(panelBox(40, 100, 920, 360, "ترافیک زنده per-server", "// مجموع: 847 Mbps"))

	// chips
	chips := []struct {
		L      string
		Active bool
	}{
		{"۱ دقیقه", true}, {"۱ ساعت", false}, {"۲۴ ساعت", false}, {"۷ روز", false},
	}
	cx := 60
	for _, c := range chips {
		w := len(c.L)*7 + 28
		fill := "rgba(255,255,255,0.04)"
		stroke := "rgba(255,255,255,0.08)"
		color := "#a8b0c9"
		if c.Active {
			fill = "rgba(0,255,209,0.08)"
			stroke = "rgba(0,255,209,0.3)"
			color = "#00ffd1"
		}
		b.WriteString(fmt.Sprintf(`
<rect x="%d" y="115" width="%d" height="26" rx="10" fill="%s" stroke="%s"/>
<text x="%d" y="132" text-anchor="middle" font-family="Vazirmatn" font-size="11" fill="%s" font-weight="600">%s</text>`,
			cx, w, fill, stroke, cx+w/2, color, c.L))
		cx += w + 8
	}

	// chart area
	chartX, chartY, chartW, chartH := 70, 170, 870, 260
	// grid lines
	for i := 1; i <= 4; i++ {
		yy := chartY + (chartH * i / 5)
		b.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#ffffff" stroke-opacity="0.04"/>
<text x="%d" y="%d" font-family="JetBrains Mono" font-size="9" fill="#5e6585">%dMb</text>`,
			chartX+50, yy, chartX+chartW, yy, chartX, yy+3, 800-i*200))
	}

	// area chart 1 (Germany - top)
	b.WriteString(fmt.Sprintf(`
<path d="M%d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d L%d,%d L%d,%d Z" fill="url(#trafficGreen)"/>
<path d="M%d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d" fill="none" stroke="#00ffd1" stroke-width="2"/>`,
		chartX+50, chartY+130, chartX+150, chartY+110, chartX+250, chartY+90, chartX+350, chartY+85,
		chartX+450, chartY+95, chartX+550, chartY+70, chartX+650, chartY+75,
		chartX+750, chartY+50, chartX+850, chartY+55, chartX+870, chartY+45,
		chartX+880, chartY+50, chartX+880, chartY+45, chartX+870, chartY+45,
		chartX+870, chartY+chartH, chartX+50, chartY+chartH,

		chartX+50, chartY+130, chartX+150, chartY+110, chartX+250, chartY+90, chartX+350, chartY+85,
		chartX+450, chartY+95, chartX+550, chartY+70, chartX+650, chartY+75,
		chartX+750, chartY+50, chartX+850, chartY+55, chartX+870, chartY+45,
		chartX+880, chartY+50, chartX+880, chartY+45, chartX+870, chartY+45))

	// area chart 2 (Netherlands - mid)
	b.WriteString(fmt.Sprintf(`
<path d="M%d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d L%d,%d L%d,%d Z" fill="url(#trafficPurple)"/>
<path d="M%d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d" fill="none" stroke="#7c5cff" stroke-width="2"/>`,
		chartX+50, chartY+170, chartX+150, chartY+160, chartX+250, chartY+150, chartX+350, chartY+155,
		chartX+450, chartY+165, chartX+550, chartY+135, chartX+650, chartY+140,
		chartX+750, chartY+115, chartX+870, chartY+120, chartX+870, chartY+120,
		chartX+870, chartY+chartH, chartX+50, chartY+chartH,

		chartX+50, chartY+170, chartX+150, chartY+160, chartX+250, chartY+150, chartX+350, chartY+155,
		chartX+450, chartY+165, chartX+550, chartY+135, chartX+650, chartY+140,
		chartX+750, chartY+115, chartX+870, chartY+120, chartX+870, chartY+120))

	// area chart 3 (Finland - low)
	b.WriteString(fmt.Sprintf(`
<path d="M%d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d L%d,%d L%d,%d Z" fill="url(#trafficPink)"/>
<path d="M%d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d" fill="none" stroke="#ff5c87" stroke-width="2"/>`,
		chartX+50, chartY+220, chartX+250, chartY+215, chartX+450, chartY+205,
		chartX+650, chartY+200, chartX+870, chartY+195, chartX+870, chartY+195,
		chartX+870, chartY+195,
		chartX+870, chartY+chartH, chartX+50, chartY+chartH,

		chartX+50, chartY+220, chartX+250, chartY+215, chartX+450, chartY+205,
		chartX+650, chartY+200, chartX+870, chartY+195, chartX+870, chartY+195,
		chartX+870, chartY+195))

	// pulse marker
	b.WriteString(fmt.Sprintf(`<circle cx="%d" cy="%d" r="6" fill="#00ffd1" opacity="0.4"/>
<circle cx="%d" cy="%d" r="3" fill="#00ffd1"/>`, chartX+870, chartY+45, chartX+870, chartY+45))

	// legend
	legend := []struct{ Color, Name string }{
		{"#00ffd1", "🇩🇪 Germany-FRA"},
		{"#7c5cff", "🇳🇱 Netherlands-AMS"},
		{"#ff5c87", "🇫🇮 Finland-HEL"},
	}
	lx := 70
	for _, lg := range legend {
		b.WriteString(fmt.Sprintf(`
<circle cx="%d" cy="442" r="4" fill="%s"/>
<text x="%d" y="446" font-family="JetBrains Mono" font-size="10" fill="#a8b0c9">%s</text>`,
			lx, lg.Color, lx+12, lg.Name))
		lx += 200
	}
	b.WriteString(`<text x="940" y="446" text-anchor="end" font-family="JetBrains Mono" font-size="10" fill="#5e6585">+7 سرور دیگر</text>`)

	// per-server breakdown panel below
	b.WriteString(panelBox(40, 480, 920, 260, "تفکیک per-server", "// آخرین snapshot"))
	servers := []struct {
		Flag, Name, Bar, Color string
		Speed                  string
	}{
		{"🇩🇪", "Germany-FRA-01", "70", "#00ffa3", "284 Mbps"},
		{"🇳🇱", "Netherlands-AMS-01", "55", "#00ffa3", "198 Mbps"},
		{"🇦🇹", "Austria-VIE-01", "42", "#00ffa3", "142 Mbps"},
		{"🇸🇪", "Sweden-STO-01", "38", "#00ffa3", "134 Mbps"},
		{"🇯🇵", "Japan-TYO-01", "30", "#00ffa3", "102 Mbps"},
	}
	for i, s := range servers {
		y := 540 + i*36
		b.WriteString(fmt.Sprintf(`
<text x="80" y="%d" font-size="14">%s</text>
<text x="115" y="%d" font-family="Vazirmatn" font-size="13" font-weight="600" fill="#f4f6ff">%s</text>
<rect x="320" y="%d" width="500" height="6" rx="3" fill="#ffffff" fill-opacity="0.06"/>
<rect x="320" y="%d" width="%s" height="6" rx="3" fill="%s"/>
<text x="940" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="11" font-weight="600" fill="%s">%s</text>`,
			y+16, s.Flag, y+15, s.Name, y+12, y+12, s.Bar+"0", s.Color, y+15, s.Color, s.Speed))
	}

	b.WriteString(statusbar(""))
	b.WriteString(svgClose())
	return b.String()
}

// =============================================================================
// PAGE: SPEED BOOST
// =============================================================================
func speedPage() string {
	var b strings.Builder
	b.WriteString(svgOpen())
	b.WriteString(background())
	b.WriteString(sidebar("speed"))
	b.WriteString(topbar("Speed Boost", "// optimizations to maximize aggregate bandwidth"))

	// hero panel
	b.WriteString(`
<rect x="40" y="100" width="920" height="290" rx="20" fill="url(#card)" stroke="#7c5cff" stroke-opacity="0.25"/>
<rect x="40" y="100" width="920" height="290" rx="20" fill="url(#brand)" fill-opacity="0.04"/>
<text x="940" y="135" text-anchor="end" font-family="Syne" font-size="20" font-weight="700" fill="#f4f6ff" direction="rtl">🚀 Speed Boost</text>
<text x="940" y="155" text-anchor="end" font-family="JetBrains Mono" font-size="11" fill="#5e6585" direction="rtl">// چطور OutBalancer سرعت رو حداکثر می‌کنه</text>`)

	tips := []struct{ Color, Title, Body string }{
		{"#00ffd1", "هیچ‌وقت کندتر از یه سرور نیست",
			"با leastPing همیشه بهترین سرور انتخاب میشه. در بدترین حالت = سرعت سریع‌ترین کانفیگ."},
		{"#7c5cff", "مرور چندتاب / استریم: جمع پهنای باند",
			"یوتیوب، نتفلیکس و مرورگرها هر چند ثانیه chunk جدید می‌خوان. هر chunk از یه سرور = ۵+۵+۵=۱۵ مگ"},
		{"#ff5c87", "دانلود فایل بزرگ: Chunk Downloader",
			"فایل‌های مستقیم HTTPS رو با Range Requests میشکنیم به N تکه و موازی دانلود می‌کنیم."},
		{"#ffb547", "⚠ محدودیت ذاتی TCP",
			"یه stream واحد همیشه از یه سرور میره — این محدودیت TCP است نه OutBalancer."},
	}
	for i, t := range tips {
		y := 175 + i*52
		b.WriteString(fmt.Sprintf(`
<rect x="60" y="%d" width="880" height="42" rx="11" fill="#ffffff" fill-opacity="0.04"/>
<rect x="935" y="%d" width="3" height="32" fill="%s"/>
<text x="925" y="%d" text-anchor="end" font-family="Vazirmatn" font-size="13" font-weight="700" fill="#f4f6ff" direction="rtl">%s</text>
<text x="925" y="%d" text-anchor="end" font-family="Vazirmatn" font-size="11" fill="#a8b0c9" direction="rtl">%s</text>`,
			y, y+5, t.Color, y+18, t.Title, y+34, t.Body))
	}

	// settings panel
	b.WriteString(panelBox(40, 410, 920, 330, "تنظیمات Speed Boost", "// همه پیش‌فرض‌ها برای حداکثر سرعت بدون افت"))

	settings := []struct {
		Icon, IconColor, Title, Body, ValueOrSwitch string
		On                                          bool
	}{
		{"🔗", "#00ffd1", "Sticky-by-Domain", "هر دامنه به یه سرور می‌چسبه — handshake های TLS تکرار نمیشن", "ON", true},
		{"⏱", "#7c5cff", "Sticky TTL (ثانیه)", "مدت چسبیدن هر دامنه به سرور — کم: parallelism بیشتر، زیاد: stability بیشتر", "60", false},
		{"🔀", "#ff5c87", "Smart Split", "پخش هوشمند کانکشن‌های جدید بین سرورها برای جمع پهنای باند", "ON", true},
		{"⬇", "#00ffd1", "Chunk Downloader [PRO]", "دانلود مستقیم فایل‌های بزرگ از چند سرور همزمان (Range Requests)", "OFF", false},
	}
	for i, s := range settings {
		y := 470 + i*60
		b.WriteString(fmt.Sprintf(`
<rect x="60" y="%d" width="880" height="50" rx="14" fill="#ffffff" fill-opacity="0.04" stroke="#ffffff" stroke-opacity="0.08"/>
<rect x="884" y="%d" width="44" height="44" rx="12" fill="%s" fill-opacity="0.1"/>
<text x="906" y="%d" text-anchor="middle" font-size="18" fill="%s">%s</text>
<text x="870" y="%d" text-anchor="end" font-family="Vazirmatn" font-size="13" font-weight="600" fill="#f4f6ff" direction="rtl">%s</text>
<text x="870" y="%d" text-anchor="end" font-family="Vazirmatn" font-size="10" fill="#5e6585" direction="rtl">%s</text>`,
			y, y+3, s.IconColor, y+30, s.IconColor, s.Icon, y+19, s.Title, y+34, s.Body))

		// switch or value
		if s.ValueOrSwitch == "ON" || s.ValueOrSwitch == "OFF" {
			swFill := "rgba(255,255,255,0.05)"
			swStroke := "rgba(255,255,255,0.08)"
			knobX := 84
			knobColor := "#a8b0c9"
			if s.On {
				swFill = "rgba(0,255,209,0.2)"
				swStroke = "rgba(0,255,209,0.4)"
				knobX = 102
				knobColor = "#00ffd1"
			}
			b.WriteString(fmt.Sprintf(`
<rect x="80" y="%d" width="42" height="24" rx="12" fill="%s" stroke="%s"/>
<circle cx="%d" cy="%d" r="8" fill="%s"/>`, y+13, swFill, swStroke, knobX, y+25, knobColor))
		} else {
			b.WriteString(fmt.Sprintf(`
<rect x="80" y="%d" width="60" height="28" rx="8" fill="#ffffff" fill-opacity="0.05" stroke="#ffffff" stroke-opacity="0.1"/>
<text x="110" y="%d" text-anchor="middle" font-family="JetBrains Mono" font-size="13" font-weight="600" fill="#f4f6ff">%s</text>`,
				y+11, y+30, s.ValueOrSwitch))
		}
	}

	b.WriteString(statusbar(""))
	b.WriteString(svgClose())
	return b.String()
}

// =============================================================================
// PAGE: ROUTING
// =============================================================================
func routingPage() string {
	var b strings.Builder
	b.WriteString(svgOpen())
	b.WriteString(background())
	b.WriteString(sidebar("routing"))
	b.WriteString(topbar("قوانین مسیریابی", "// custom rules engine · ۶ rule فعال"))

	b.WriteString(panelBox(40, 100, 920, 640, "قوانین فعال", "// click on a rule to edit"))

	// add button
	b.WriteString(`
<rect x="800" y="115" width="140" height="30" rx="10" fill="url(#primary)"/>
<text x="870" y="135" text-anchor="middle" font-family="Vazirmatn" font-size="12" font-weight="700" fill="#001a14" direction="rtl">+ قانون جدید</text>`)

	rules := []struct {
		Name, Domains, Target, Status string
		StatusColor                   string
	}{
		{"YouTube → پرسرعت", "*.youtube.com, googlevideo.com", "⚡ HIGH-SPEED", "ENABLED", "#00ffa3"},
		{"Telegram → کم‌پینگ", "telegram.org, t.me", "📡 LOW-PING", "ENABLED", "#00ffa3"},
		{"Iran → Direct", "geoip:ir", "↩ DIRECT", "ENABLED", "#00ffa3"},
		{"Spotify → استریم", "*.spotify.com, scdn.co", "🎵 STREAMING", "ENABLED", "#00ffa3"},
		{"Banking → Sticky DE", "bank-*.com, paypal.com", "🔒 STICKY: DE-01", "ENABLED", "#00ffa3"},
		{"Ads → Block", "*.doubleclick.net, *.googletagmanager.com", "✕ BLOCK", "DISABLED", "#5e6585"},
	}
	for i, r := range rules {
		y := 200 + i*84
		b.WriteString(fmt.Sprintf(`
<rect x="60" y="%d" width="880" height="68" rx="14" fill="#ffffff" fill-opacity="0.04" stroke="#ffffff" stroke-opacity="0.08"/>

<text x="920" y="%d" text-anchor="end" font-family="Syne" font-size="14" font-weight="700" fill="#f4f6ff" direction="rtl">%s</text>

<text x="920" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="10" fill="#5e6585" direction="rtl">domains: %s</text>

<rect x="100" y="%d" width="160" height="28" rx="8" fill="#00ffd1" fill-opacity="0.08" stroke="#00ffd1" stroke-opacity="0.2"/>
<text x="180" y="%d" text-anchor="middle" font-family="Vazirmatn" font-size="11" font-weight="600" fill="#00ffd1" direction="rtl">%s</text>

<rect x="60" y="%d" width="32" height="22" rx="6" fill="%s" fill-opacity="0.12"/>
<text x="76" y="%d" text-anchor="middle" font-family="JetBrains Mono" font-size="9" font-weight="700" fill="%s">%s</text>`,
			y, y+24, r.Name, y+44, r.Domains,
			y+24, y+43, r.Target,
			y+27, r.StatusColor, y+42, r.StatusColor, r.Status))
	}

	b.WriteString(statusbar(""))
	b.WriteString(svgClose())
	return b.String()
}

// =============================================================================
// PAGE: ALGORITHM
// =============================================================================
func algorithmPage() string {
	var b strings.Builder
	b.WriteString(svgOpen())
	b.WriteString(background())
	b.WriteString(sidebar("algorithm"))
	b.WriteString(topbar("الگوریتم بالانس", "// 6 algorithms available · انتخاب فعلی: latency"))

	b.WriteString(panelBox(40, 100, 920, 640, "انتخاب الگوریتم", "// click to activate"))

	algs := []struct {
		Icon, Title, Body, Tag string
		Active                 bool
	}{
		{"⏱", "بر اساس کم‌ترین پینگ", "بهترین برای گیمینگ و کال — به محض کاهش پینگ، traffic منتقل میشه", "latency", true},
		{"🚀", "بر اساس بیشترین سرعت", "بهترین برای استریم و دانلود — متصل به سرور با بالاترین Mbps", "speed", false},
		{"⚖", "وزن‌دار (Weighted RR)", "تقسیم بر اساس وزن دستی هر سرور — اولویت با سرورهای قوی‌تر", "weighted", false},
		{"📊", "کم‌ترین کانکشن فعال", "تقسیم بر اساس بار فعلی — متعادل کردن load بین سرورها", "leastconn", false},
		{"🔄", "گردشی ساده (Round Robin)", "نوبتی به همه سرورها — ساده و قابل پیش‌بینی", "roundrobin", false},
		{"🎲", "تصادفی", "انتخاب رندوم — برای تست و scenarios خاص", "random", false},
	}
	for i, a := range algs {
		col := i % 2
		row := i / 2
		x := 60 + col*450
		y := 180 + row*180

		fill := "rgba(255,255,255,0.04)"
		stroke := "rgba(255,255,255,0.08)"
		check := ""
		if a.Active {
			fill = "rgba(0,255,209,0.06)"
			stroke = "rgba(0,255,209,0.3)"
			check = `<circle cx="65" cy="22" r="11" fill="#00ffd1"/><text x="65" y="27" text-anchor="middle" font-size="13" fill="#001a14" font-weight="900">✓</text>`
		}
		b.WriteString(fmt.Sprintf(`
<g transform="translate(%d,%d)">
  <rect width="430" height="160" rx="16" fill="%s" stroke="%s"/>
  <rect x="378" y="20" width="44" height="44" rx="14" fill="#ffffff" fill-opacity="0.06" stroke="#ffffff" stroke-opacity="0.14"/>
  <text x="400" y="48" text-anchor="middle" font-size="20" fill="#00ffd1">%s</text>
  <text x="370" y="48" text-anchor="end" font-family="Syne" font-size="15" font-weight="700" fill="#f4f6ff" direction="rtl">%s</text>
  <text x="370" y="76" text-anchor="end" font-family="Vazirmatn" font-size="11" fill="#a8b0c9" direction="rtl">%s</text>
  <rect x="20" y="120" width="80" height="22" rx="7" fill="#ffffff" fill-opacity="0.05"/>
  <text x="60" y="135" text-anchor="middle" font-family="JetBrains Mono" font-size="10" fill="#a8b0c9">%s</text>
  %s
</g>`, x, y, fill, stroke, a.Icon, a.Title, a.Body, a.Tag, check))
	}

	b.WriteString(statusbar(""))
	b.WriteString(svgClose())
	return b.String()
}

// =============================================================================
// PAGE: HEATMAP
// =============================================================================
func heatmapPage() string {
	var b strings.Builder
	b.WriteString(svgOpen())
	b.WriteString(background())
	b.WriteString(sidebar("heatmap"))
	b.WriteString(topbar("هیت‌مپ مصرف", "// hour × day · last 7 days"))

	b.WriteString(panelBox(40, 100, 920, 360, "نمودار حرارتی مصرف", "// 7 day × 24 hour"))

	// heatmap grid
	startX, startY := 130, 200
	cellW, cellH := 32, 24

	// hour labels (top)
	for h := 0; h < 24; h++ {
		if h%3 == 0 {
			b.WriteString(fmt.Sprintf(`<text x="%d" y="190" text-anchor="middle" font-family="JetBrains Mono" font-size="9" fill="#5e6585">%02d</text>`,
				startX+h*cellW+cellW/2, h))
		}
	}
	days := []string{"شنبه", "یکشنبه", "دوشنبه", "سه‌شنبه", "چهارشنبه", "پنج‌شنبه", "جمعه"}
	// cells
	for d := 0; d < 7; d++ {
		b.WriteString(fmt.Sprintf(`<text x="120" y="%d" text-anchor="end" font-family="Vazirmatn" font-size="10" fill="#a8b0c9">%s</text>`,
			startY+d*cellH+15, days[d]))
		for h := 0; h < 24; h++ {
			intensity := 0.05 + 0.1*float64((h*7+d*3)%9)/10.0
			if h >= 18 && h <= 23 {
				intensity = 0.5 + 0.4*float64((h*3+d)%5)/10.0
			} else if h >= 9 && h <= 17 {
				intensity = 0.2 + 0.3*float64((h*5+d)%7)/10.0
			}
			x := startX + h*cellW
			y := startY + d*cellH
			b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" rx="3" fill="#00ffd1" fill-opacity="%.2f"/>`,
				x, y, cellW-3, cellH-3, intensity))
		}
	}

	// legend
	b.WriteString(`<g transform="translate(60,420)">
  <text x="0" y="13" font-family="JetBrains Mono" font-size="10" fill="#5e6585">کم</text>
  <rect x="30" y="2" width="20" height="14" rx="3" fill="#00ffd1" fill-opacity="0.05"/>
  <rect x="54" y="2" width="20" height="14" rx="3" fill="#00ffd1" fill-opacity="0.2"/>
  <rect x="78" y="2" width="20" height="14" rx="3" fill="#00ffd1" fill-opacity="0.4"/>
  <rect x="102" y="2" width="20" height="14" rx="3" fill="#00ffd1" fill-opacity="0.65"/>
  <rect x="126" y="2" width="20" height="14" rx="3" fill="#00ffd1" fill-opacity="0.9"/>
  <text x="155" y="13" font-family="JetBrains Mono" font-size="10" fill="#5e6585">زیاد</text>
</g>`)

	// top domains panel below
	b.WriteString(panelBox(40, 480, 460, 260, "پر مصرف‌ترین دامنه‌ها", "// last 24 hours"))
	domains := []struct {
		Icon, IconColor, Name, Bytes string
	}{
		{"▶", "#ff0000", "youtube.com", "412 GB"},
		{"♫", "#1ed760", "spotify.com", "187 GB"},
		{"✈", "#0088cc", "telegram.org", "94 GB"},
		{"🎬", "#e50914", "netflix.com", "61 GB"},
		{"⌥", "#aaaaaa", "github.com", "38 GB"},
	}
	for i, d := range domains {
		y := 540 + i*36
		b.WriteString(fmt.Sprintf(`
<rect x="60" y="%d" width="24" height="24" rx="6" fill="%s" fill-opacity="0.15"/>
<text x="72" y="%d" text-anchor="middle" font-size="12" fill="%s">%s</text>
<text x="100" y="%d" font-family="JetBrains Mono" font-size="12" fill="#a8b0c9">%s</text>
<text x="480" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="11" font-weight="700" fill="#00ffd1">%s</text>`,
			y, d.IconColor, y+15, d.IconColor, d.Icon, y+15, d.Name, y+15, d.Bytes))
	}

	// stats panel right
	b.WriteString(panelBox(520, 480, 440, 260, "تحلیل دوره", "// last 7 days"))
	stats := []struct {
		Label, Value, Color string
	}{
		{"کل ترافیک", "9.2 TB", "#00ffd1"},
		{"اوج مصرف", "21:00 (پنج‌شنبه)", "#ff5c87"},
		{"کم‌ترین مصرف", "05:00 (دوشنبه)", "#a8b0c9"},
		{"میانگین روزانه", "1.31 TB", "#7c5cff"},
		{"رشد نسبت به هفته قبل", "+12.4%", "#00ffa3"},
	}
	for i, s := range stats {
		y := 540 + i*36
		b.WriteString(fmt.Sprintf(`
<rect x="540" y="%d" width="400" height="28" rx="8" fill="#ffffff" fill-opacity="0.04"/>
<text x="555" y="%d" font-family="Vazirmatn" font-size="11" fill="#5e6585" direction="rtl">%s</text>
<text x="925" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="13" font-weight="700" fill="%s">%s</text>`,
			y, y+18, s.Label, y+18, s.Color, s.Value))
	}

	b.WriteString(statusbar(""))
	b.WriteString(svgClose())
	return b.String()
}

// =============================================================================
// PAGE: ALERTS
// =============================================================================
func alertsPage() string {
	var b strings.Builder
	b.WriteString(svgOpen())
	b.WriteString(background())
	b.WriteString(sidebar("alerts"))
	b.WriteString(topbar("هشدارها", "// realtime alert stream · 14 alerts in last 24h"))

	b.WriteString(panelBox(40, 100, 920, 640, "همه هشدارها", "// click to mark as read"))

	// chips
	chips := []struct {
		L      string
		Active bool
	}{
		{"همه (14)", true}, {"بحرانی", false}, {"هشدار", false}, {"اطلاع", false}, {"خوانده‌نشده", false},
	}
	cx := 60
	for _, c := range chips {
		w := len(c.L)*7 + 28
		fill := "rgba(255,255,255,0.04)"
		stroke := "rgba(255,255,255,0.08)"
		color := "#a8b0c9"
		if c.Active {
			fill = "rgba(0,255,209,0.08)"
			stroke = "rgba(0,255,209,0.3)"
			color = "#00ffd1"
		}
		b.WriteString(fmt.Sprintf(`
<rect x="%d" y="115" width="%d" height="26" rx="10" fill="%s" stroke="%s"/>
<text x="%d" y="132" text-anchor="middle" font-family="Vazirmatn" font-size="11" fill="%s" font-weight="600">%s</text>`,
			cx, w, fill, stroke, cx+w/2, color, c.L))
		cx += w + 8
	}

	alertsData := []struct {
		Level, Icon, Title, Body, Time string
		LevelColor                     string
	}{
		{"crit", "⚠", "سرور France-PAR-02 آفلاین شد", "3 health-check پیاپی ناموفق · از pool خارج شد · IP: 185.x.x.x", "2 دقیقه قبل", "#ff4d6d"},
		{"warn", "📈", "پینگ Turkey-IST بالا رفت", "میانگین: 187ms (آستانه: 150ms) · بازه: 5m", "5 دقیقه قبل", "#ffb547"},
		{"warn", "💾", "سهمیه ماهانه US-NY: 80%", "40GB از 50GB استفاده شده · باقی‌مانده: 10GB", "12 دقیقه قبل", "#ffb547"},
		{"info", "✓", "وزن DE-FRA به‌روز شد", "auto-tune: 5 → 7 (بر اساس performance طی ۲۴ ساعت)", "23 دقیقه قبل", "#00ffd1"},
		{"info", "↻", "سرور Germany-FRA recovered", "بعد از 38 ثانیه آفلاین بودن، به pool برگشت", "1 ساعت قبل", "#00ffd1"},
		{"warn", "⏱", "Throughput دروپ شد", "از 920 Mbps به 540 Mbps · احتمالاً مشکل ISP", "2 ساعت قبل", "#ffb547"},
	}
	for i, a := range alertsData {
		y := 180 + i*88
		b.WriteString(fmt.Sprintf(`
<rect x="60" y="%d" width="880" height="72" rx="14" fill="#ffffff" fill-opacity="0.04" stroke="#ffffff" stroke-opacity="0.08"/>

<rect x="884" y="%d" width="36" height="36" rx="11" fill="%s" fill-opacity="0.12"/>
<text x="902" y="%d" text-anchor="middle" font-size="16" fill="%s">%s</text>

<text x="870" y="%d" text-anchor="end" font-family="Vazirmatn" font-size="13" font-weight="700" fill="#f4f6ff" direction="rtl">%s</text>
<text x="870" y="%d" text-anchor="end" font-family="Vazirmatn" font-size="11" fill="#a8b0c9" direction="rtl">%s</text>
<text x="870" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="9" fill="#5e6585" direction="rtl">%s</text>`,
			y, y+8, a.LevelColor, y+30, a.LevelColor, a.Icon,
			y+22, a.Title, y+44, a.Body, y+62, a.Time))
	}

	b.WriteString(statusbar(""))
	b.WriteString(svgClose())
	return b.String()
}

// =============================================================================
// PAGE: SETTINGS
// =============================================================================
func settingsPage() string {
	var b strings.Builder
	b.WriteString(svgOpen())
	b.WriteString(background())
	b.WriteString(sidebar("settings"))
	b.WriteString(topbar("تنظیمات", "// listen ports · TUN · DNS · health-check"))

	// listen ports panel
	b.WriteString(panelBox(40, 100, 920, 240, "پورت‌های گوش‌دهنده", "// proxy listen settings"))
	fields := []struct{ Label, Value, Desc string }{
		{"SOCKS Port", "10808", "استفاده در مرورگر/سیستم: socks5://127.0.0.1:10808"},
		{"HTTP Port", "10809", "استفاده برای Linux env: HTTP_PROXY=http://127.0.0.1:10809"},
		{"Web Panel Port", "8088", "این صفحه روی این پورت اجرا میشه"},
		{"TUN Interface", "outb0 / 10.10.0.1/24", "(غیرفعال) tun mode for system-wide routing"},
	}
	for i, f := range fields {
		col := i % 2
		row := i / 2
		x := 60 + col*450
		y := 170 + row*70
		b.WriteString(fmt.Sprintf(`
<rect x="%d" y="%d" width="430" height="60" rx="12" fill="#ffffff" fill-opacity="0.04" stroke="#ffffff" stroke-opacity="0.08"/>
<text x="%d" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="9" fill="#5e6585" letter-spacing="1.5">%s</text>
<text x="%d" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="14" font-weight="700" fill="#00ffd1">%s</text>
<text x="%d" y="%d" text-anchor="end" font-family="Vazirmatn" font-size="10" fill="#a8b0c9" direction="rtl">%s</text>`,
			x, y, x+415, y+18, f.Label, x+415, y+38, f.Value, x+415, y+54, f.Desc))
	}

	// health check panel
	b.WriteString(panelBox(40, 360, 460, 380, "Health Check", "// probe interval & thresholds"))
	hcFields := []struct{ Label, Value string }{
		{"INTERVAL", "15 ثانیه"},
		{"TIMEOUT", "4 ثانیه"},
		{"FAILURES THRESHOLD", "3"},
		{"PROBE TYPE", "TCP Connect"},
		{"PING WARN THRESHOLD", "150ms"},
	}
	for i, f := range hcFields {
		y := 430 + i*54
		b.WriteString(fmt.Sprintf(`
<rect x="60" y="%d" width="420" height="42" rx="10" fill="#ffffff" fill-opacity="0.04"/>
<text x="75" y="%d" font-family="JetBrains Mono" font-size="9" fill="#5e6585" letter-spacing="1.5">%s</text>
<text x="465" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="13" font-weight="700" fill="#f4f6ff">%s</text>`,
			y, y+18, f.Label, y+27, f.Value))
	}

	// system info panel
	b.WriteString(panelBox(520, 360, 440, 380, "System Info", "// runtime statistics"))
	sysFields := []struct {
		Label, Value, Color string
	}{
		{"Version", "v0.1.0 (build a3f291c)", "#f4f6ff"},
		{"Core", "Xray v25.3.6", "#00ffd1"},
		{"Goroutines", "24 active", "#a8b0c9"},
		{"Memory", "184.2 MB", "#a8b0c9"},
		{"Uptime", "14d 06h 23m", "#00ffa3"},
		{"Total Requests", "1,847,392", "#7c5cff"},
		{"WebSocket Clients", "1 connected", "#00ffd1"},
	}
	for i, s := range sysFields {
		y := 420 + i*44
		b.WriteString(fmt.Sprintf(`
<line x1="540" y1="%d" x2="940" y2="%d" stroke="#ffffff" stroke-opacity="0.04"/>
<text x="555" y="%d" font-family="JetBrains Mono" font-size="10" fill="#5e6585">%s</text>
<text x="925" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="12" font-weight="700" fill="%s">%s</text>`,
			y, y, y+22, s.Label, y+22, s.Color, s.Value))
	}

	b.WriteString(statusbar(""))
	b.WriteString(svgClose())
	return b.String()
}

// =============================================================================
// PAGE: LOGS
// =============================================================================
func logsPage() string {
	var b strings.Builder
	b.WriteString(svgOpen())
	b.WriteString(background())
	b.WriteString(sidebar("logs"))
	b.WriteString(topbar("لاگ‌ها", "// last 500 entries · live tailing"))

	b.WriteString(panelBox(40, 100, 920, 640, "Log Stream", "// click row to expand"))

	// chips
	chips := []struct {
		L      string
		Active bool
	}{
		{"همه", true}, {"info", false}, {"warn", false}, {"error", false},
		{"http", false}, {"xray", false}, {"balancer", false},
	}
	cx := 60
	for _, c := range chips {
		w := len(c.L)*7 + 24
		fill := "rgba(255,255,255,0.04)"
		stroke := "rgba(255,255,255,0.08)"
		color := "#a8b0c9"
		if c.Active {
			fill = "rgba(0,255,209,0.08)"
			stroke = "rgba(0,255,209,0.3)"
			color = "#00ffd1"
		}
		b.WriteString(fmt.Sprintf(`
<rect x="%d" y="115" width="%d" height="26" rx="10" fill="%s" stroke="%s"/>
<text x="%d" y="132" text-anchor="middle" font-family="JetBrains Mono" font-size="10" fill="%s" font-weight="600">%s</text>`,
			cx, w, fill, stroke, cx+w/2, color, c.L))
		cx += w + 8
	}

	// log table header
	b.WriteString(`
<text x="120" y="180" text-anchor="end" font-family="JetBrains Mono" font-size="9" fill="#5e6585">TIME</text>
<text x="190" y="180" text-anchor="end" font-family="JetBrains Mono" font-size="9" fill="#5e6585">LEVEL</text>
<text x="290" y="180" text-anchor="end" font-family="JetBrains Mono" font-size="9" fill="#5e6585">SOURCE</text>
<text x="320" y="180" font-family="JetBrains Mono" font-size="9" fill="#5e6585">MESSAGE</text>
<line x1="60" y1="190" x2="940" y2="190" stroke="#ffffff" stroke-opacity="0.08"/>`)

	logs := []struct {
		Time, Level, Source, Msg string
		LevelColor               string
	}{
		{"15:42:18.234", "info", "http", "GET /api/overview -> 200 (3ms)", "#00ffd1"},
		{"15:42:17.891", "info", "balancer", "tick: 9 servers online, 847 Mbps total", "#00ffd1"},
		{"15:42:15.642", "info", "xray", "config رندر شد (10 outbound)", "#00ffd1"},
		{"15:42:12.103", "warn", "healthcheck", "Turkey-IST-01 ping 187ms (threshold: 150ms)", "#ffb547"},
		{"15:42:08.451", "info", "http", "POST /api/servers -> 201 (12ms)", "#00ffd1"},
		{"15:42:03.998", "error", "healthcheck", "France-PAR-02: dial tcp: lookup paris.example.com: no such host", "#ff4d6d"},
		{"15:42:01.554", "info", "api", "سرور جدید: TestServer-1", "#00ffd1"},
		{"15:41:58.122", "warn", "balancer", "high latency detected on Turkey-IST-01", "#ffb547"},
		{"15:41:54.789", "info", "http", "GET /api/servers -> 200 (8ms)", "#00ffd1"},
		{"15:41:52.301", "info", "xray", "outbound: out_43cf3be33da3 connected", "#00ffd1"},
		{"15:41:48.876", "info", "balancer", "algorithm: latency, picking Germany-FRA-01", "#00ffd1"},
		{"15:41:43.224", "warn", "store", "config persistence took 142ms", "#ffb547"},
		{"15:41:38.991", "info", "http", "GET /api/heatmap -> 200 (15ms)", "#00ffd1"},
		{"15:41:35.553", "error", "xray", "France-PAR-02 outbound failed 3 times, removed from pool", "#ff4d6d"},
		{"15:41:30.111", "info", "balancer", "tick: traffic 920 Mbps -> 845 Mbps", "#00ffd1"},
	}
	for i, lg := range logs {
		y := 215 + i*32
		b.WriteString(fmt.Sprintf(`
<text x="120" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="10" fill="#5e6585">%s</text>
<rect x="135" y="%d" width="58" height="18" rx="4" fill="%s" fill-opacity="0.12"/>
<text x="164" y="%d" text-anchor="middle" font-family="JetBrains Mono" font-size="9" font-weight="700" fill="%s">%s</text>
<text x="290" y="%d" text-anchor="end" font-family="JetBrains Mono" font-size="10" fill="#7c5cff">[%s]</text>
<text x="305" y="%d" font-family="JetBrains Mono" font-size="10" fill="#a8b0c9">%s</text>`,
			y, lg.Time, y-13, lg.LevelColor, y, lg.LevelColor, lg.Level,
			y, lg.Source, y, lg.Msg))
	}

	b.WriteString(statusbar(""))
	b.WriteString(svgClose())
	return b.String()
}

// =============================================================================
// SVG OPEN/CLOSE
// =============================================================================
func svgOpen() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">
%s`, W, H, W, H, styleDefs)
}
func svgClose() string {
	return `</svg>`
}

// =============================================================================
// MAIN
// =============================================================================
func main() {
	outDir := "docs/images"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	pages := map[string]func() string{
		"01-dashboard.svg": dashboardPage,
		"02-servers.svg":   serversPage,
		"03-stats.svg":     statsPage,
		"04-speed.svg":     speedPage,
		"05-routing.svg":   routingPage,
		"06-algorithm.svg": algorithmPage,
		"07-heatmap.svg":   heatmapPage,
		"08-alerts.svg":    alertsPage,
		"09-settings.svg":  settingsPage,
		"10-logs.svg":      logsPage,
	}
	for name, fn := range pages {
		path := filepath.Join(outDir, name)
		if err := os.WriteFile(path, []byte(fn()), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Println("wrote", path)
	}
}
