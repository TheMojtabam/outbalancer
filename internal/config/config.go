package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Server represents a single proxy server configuration parsed from a vless:// URL.
type Server struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Protocol   string            `json:"protocol"`
	Address    string            `json:"address"`
	Port       int               `json:"port"`
	UUID       string            `json:"uuid"`
	Encryption string            `json:"encryption"`
	Network    string            `json:"network"`
	Security   string            `json:"security"`
	SNI        string            `json:"sni,omitempty"`
	Host       string            `json:"host,omitempty"`
	Path       string            `json:"path,omitempty"`
	Flow       string            `json:"flow,omitempty"`
	PublicKey  string            `json:"pbk,omitempty"`
	ShortID    string            `json:"sid,omitempty"`
	Fingerprint string           `json:"fp,omitempty"`
	SpiderX     string           `json:"spx,omitempty"`
	ALPN        string           `json:"alpn,omitempty"`
	AllowInsecure bool           `json:"allow_insecure,omitempty"`
	ServiceName string           `json:"service_name,omitempty"` // grpc
	GrpcMulti   bool             `json:"grpc_multi,omitempty"`
	HeaderType  string           `json:"header_type,omitempty"` // tcp http obfs
	Tag        string            `json:"tag"`
	Country    string            `json:"country,omitempty"`
	Flag       string            `json:"flag,omitempty"`
	Weight     float64           `json:"weight"`
	Enabled    bool              `json:"enabled"`
	QuotaGB    int               `json:"quota_gb,omitempty"`
	Raw        string            `json:"raw"`
	Extra      map[string]string `json:"extra,omitempty"`
}

// AppConfig is the full persistent configuration of OutBalancer.
type AppConfig struct {
	Servers      []Server      `json:"servers"`
	Algorithm    string        `json:"algorithm"`
	HealthCheck  HealthCfg     `json:"health_check"`
	Listen       ListenCfg     `json:"listen"`
	RoutingRules []RoutingRule `json:"routing_rules"`
	Profiles     []Profile     `json:"profiles"`
	Schedules    []Schedule    `json:"schedules"`
	DNS          DNSCfg        `json:"dns"`
	Auth         AuthCfg       `json:"auth"`
	WebPort      int           `json:"web_port"`
	APIKey       string        `json:"api_key,omitempty"`
	Webhook      string        `json:"webhook,omitempty"`
	Speed        SpeedCfg      `json:"speed"`
}

type HealthCfg struct {
	IntervalSec int    `json:"interval_sec"`
	TimeoutSec  int    `json:"timeout_sec"`
	Failures    int    `json:"failures"`
	Probe       string `json:"probe"` // "tcp" | "http"
	URL         string `json:"url"`
}

type ListenCfg struct {
	HTTPPort  int    `json:"http_port"`
	SOCKSPort int    `json:"socks_port"`
	Address   string `json:"address,omitempty"` // bind addr; empty = 127.0.0.1 (LAN-safe default)
	TUNName   string `json:"tun_name"`
	TUNAddr   string `json:"tun_addr"`
	TUNEnable bool   `json:"tun_enable"`
}

type RoutingRule struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Domains []string `json:"domains"`
	IPs     []string `json:"ips"`
	GeoIP   string   `json:"geoip,omitempty"`
	Target  string   `json:"target"` // server ID, "direct", "block", "balanced", or special tag
	Enabled bool     `json:"enabled"`
}

type Profile struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	ServerIDs  []string `json:"server_ids"`
	Algorithm  string   `json:"algorithm"`
	Active     bool     `json:"active"`
}

type Schedule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	From       string `json:"from"` // HH:MM
	To         string `json:"to"`   // HH:MM
	ProfileID  string `json:"profile_id"`
	Enabled    bool   `json:"enabled"`
}

type DNSCfg struct {
	Servers       []string `json:"servers"`
	LeakProtect   bool     `json:"leak_protect"`
	BlockMalware  bool     `json:"block_malware"`
}

type AuthCfg struct {
	Username string `json:"username"`
	Password string `json:"password_hash"`
	Enabled  bool   `json:"enabled"`
}

// SpeedCfg controls speed-optimization behaviors:
//   - StickyByDomain: each domain stays on the same proxy for its session,
//     avoiding repeated TLS handshakes (faster, lower jitter, never slower).
//   - StickyTTLSec: how long a domain keeps its assignment (lower = more
//     parallelism for streaming; higher = more stability).
//   - SmartSplit: when on, new connections are spread across multiple servers
//     by score & weight to maximize aggregate bandwidth for multi-connection
//     workloads (browsing many tabs, HLS/DASH streaming, parallel downloads).
//   - ChunkDownloader: when on, OutBalancer's HTTP proxy detects large file
//     downloads (Content-Length > threshold + Range support) and splits them
//     into N chunks fetched in parallel from N different servers.
type SpeedCfg struct {
	StickyByDomain   bool `json:"sticky_by_domain"`
	StickyTTLSec     int  `json:"sticky_ttl_sec"`
	SmartSplit       bool `json:"smart_split"`
	ChunkDownloader  bool `json:"chunk_downloader"`
	ChunkMinBytes    int  `json:"chunk_min_bytes"`
	ChunkParallelism int  `json:"chunk_parallelism"`
}

// ParseVless parses a vless:// URI into a Server.
// Format: vless://uuid@host:port?type=tcp&security=tls&sni=...&pbk=...#name
func ParseVless(uri string) (*Server, error) {
	uri = strings.TrimSpace(uri)
	if !strings.HasPrefix(uri, "vless://") && !strings.HasPrefix(uri, "vmess://") {
		return nil, errors.New("not a vless/vmess URL")
	}

	// vmess:// is base64 JSON
	if strings.HasPrefix(uri, "vmess://") {
		return parseVmess(uri)
	}

	rawURL := uri
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	if u.User == nil {
		return nil, errors.New("missing UUID in vless URL")
	}
	uuid := u.User.Username()

	host := u.Hostname()
	portStr := u.Port()
	if host == "" || portStr == "" {
		return nil, errors.New("missing host or port")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	q := u.Query()
	name, _ := url.QueryUnescape(u.Fragment)
	if name == "" {
		name = fmt.Sprintf("%s:%d", host, port)
	}

	srv := &Server{
		ID:          shortHash(rawURL),
		Name:        name,
		Protocol:    "vless",
		Address:     host,
		Port:        port,
		UUID:        uuid,
		Encryption:  firstNonEmpty(q.Get("encryption"), "none"),
		Network:     firstNonEmpty(q.Get("type"), "tcp"),
		Security:    firstNonEmpty(q.Get("security"), "none"),
		SNI:         q.Get("sni"),
		Host:        q.Get("host"),
		Path:        q.Get("path"),
		Flow:        q.Get("flow"),
		PublicKey:   q.Get("pbk"),
		ShortID:     q.Get("sid"),
		Fingerprint: q.Get("fp"),
		Tag:         "out_" + shortHash(rawURL),
		Weight:      1.0,
		Enabled:     true,
		Raw:         rawURL,
	}
	srv.Country, srv.Flag = guessCountry(name, host)
	return srv, nil
}

func parseVmess(uri string) (*Server, error) {
	payload := strings.TrimPrefix(uri, "vmess://")
	// Try standard base64
	dec, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		dec, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			dec, err = base64.URLEncoding.DecodeString(payload)
			if err != nil {
				return nil, fmt.Errorf("vmess base64: %w", err)
			}
		}
	}
	var v struct {
		V    interface{} `json:"v"`
		Ps   string      `json:"ps"`
		Add  string      `json:"add"`
		Port interface{} `json:"port"`
		ID   string      `json:"id"`
		Net  string      `json:"net"`
		Type string      `json:"type"`
		Host string      `json:"host"`
		Path string      `json:"path"`
		TLS  string      `json:"tls"`
		SNI  string      `json:"sni"`
	}
	if err := json.Unmarshal(dec, &v); err != nil {
		return nil, fmt.Errorf("vmess json: %w", err)
	}
	port := 443
	switch p := v.Port.(type) {
	case float64:
		port = int(p)
	case string:
		port, _ = strconv.Atoi(p)
	}
	srv := &Server{
		ID:       shortHash(uri),
		Name:     firstNonEmpty(v.Ps, fmt.Sprintf("%s:%d", v.Add, port)),
		Protocol: "vmess",
		Address:  v.Add,
		Port:     port,
		UUID:     v.ID,
		Network:  firstNonEmpty(v.Net, "tcp"),
		Security: firstNonEmpty(v.TLS, "none"),
		Host:     v.Host,
		Path:     v.Path,
		SNI:      v.SNI,
		Tag:      "out_" + shortHash(uri),
		Weight:   1.0,
		Enabled:  true,
		Raw:      uri,
	}
	srv.Country, srv.Flag = guessCountry(srv.Name, srv.Address)
	return srv, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// shortHash returns a short stable hash for use as ID.
func shortHash(s string) string {
	const k = 14695981039346656037
	const p = 1099511628211
	h := uint64(k)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= p
	}
	return fmt.Sprintf("%012x", h>>16)
}

// guessCountry tries to identify the country from name patterns.
func guessCountry(name, host string) (string, string) {
	n := strings.ToLower(name + " " + host)
	for _, c := range countryHints {
		for _, kw := range c.Keywords {
			if strings.Contains(n, kw) {
				return c.Code, c.Flag
			}
		}
	}
	return "", "🌐"
}

type countryHint struct {
	Code     string
	Flag     string
	Keywords []string
}

var countryHints = []countryHint{
	{"DE", "🇩🇪", []string{"germany", "german", "frankfurt", "berlin", "fra", "ger", ".de"}},
	{"NL", "🇳🇱", []string{"netherlands", "amsterdam", "ams", "holland"}},
	{"FI", "🇫🇮", []string{"finland", "helsinki", "hel"}},
	{"FR", "🇫🇷", []string{"france", "paris", "par"}},
	{"GB", "🇬🇧", []string{"uk", "britain", "london", "lon"}},
	{"US", "🇺🇸", []string{"united states", "usa", "new york", "ny", "los angeles", "la-", "miami", "dallas", "chicago"}},
	{"CA", "🇨🇦", []string{"canada", "toronto", "montreal"}},
	{"TR", "🇹🇷", []string{"turkey", "istanbul", "ist"}},
	{"AE", "🇦🇪", []string{"uae", "dubai", "emirates"}},
	{"SE", "🇸🇪", []string{"sweden", "stockholm"}},
	{"NO", "🇳🇴", []string{"norway", "oslo"}},
	{"PL", "🇵🇱", []string{"poland", "warsaw"}},
	{"RO", "🇷🇴", []string{"romania", "bucharest"}},
	{"RU", "🇷🇺", []string{"russia", "moscow"}},
	{"JP", "🇯🇵", []string{"japan", "tokyo", "osaka"}},
	{"SG", "🇸🇬", []string{"singapore", "sg-"}},
	{"HK", "🇭🇰", []string{"hong kong", "hk"}},
	{"AU", "🇦🇺", []string{"australia", "sydney"}},
	{"IN", "🇮🇳", []string{"india", "mumbai", "delhi"}},
	{"BR", "🇧🇷", []string{"brazil", "sao paulo"}},
	{"CH", "🇨🇭", []string{"switzerland", "zurich"}},
	{"AT", "🇦🇹", []string{"austria", "vienna"}},
	{"IT", "🇮🇹", []string{"italy", "milan", "rome"}},
	{"ES", "🇪🇸", []string{"spain", "madrid"}},
	{"IR", "🇮🇷", []string{"iran", "tehran"}},
	{"AM", "🇦🇲", []string{"armenia", "yerevan"}},
}

// Default returns sensible defaults for AppConfig.
func Default() AppConfig {
	return AppConfig{
		Algorithm: "latency",
		HealthCheck: HealthCfg{
			IntervalSec: 15,
			TimeoutSec:  4,
			Failures:    3,
			Probe:       "tcp",
		},
		Listen: ListenCfg{
			HTTPPort:  10809,
			SOCKSPort: 10808,
			TUNName:   "outb0",
			TUNAddr:   "10.10.0.1/24",
			TUNEnable: false,
		},
		DNS: DNSCfg{
			Servers:     []string{"1.1.1.1", "8.8.8.8"},
			LeakProtect: true,
		},
		Speed: SpeedCfg{
			StickyByDomain:   true,  // safe default: never slower than 1 server
			StickyTTLSec:     60,
			SmartSplit:       true,  // distributes new connections for aggregate BW
			ChunkDownloader:  false, // off by default; opt-in advanced feature
			ChunkMinBytes:    50 * 1024 * 1024, // 50 MB threshold
			ChunkParallelism: 4,
		},
		WebPort: 8088,
	}
}
