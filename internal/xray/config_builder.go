package xray

import (
	"strings"

	"github.com/outbalancer/outbalancer/internal/config"
)

// xrayConfig is the top-level JSON schema xray-core consumes via DecodeJSONConfig.
type xrayConfig struct {
	Log       map[string]string `json:"log,omitempty"`
	Inbounds  []inbound         `json:"inbounds"`
	Outbounds []outbound        `json:"outbounds"`
	Routing   *routing          `json:"routing,omitempty"`
	Stats     map[string]any    `json:"stats,omitempty"`
	Policy    map[string]any    `json:"policy,omitempty"`
	DNS       *dnsConf          `json:"dns,omitempty"`
}

type inbound struct {
	Tag      string         `json:"tag"`
	Listen   string         `json:"listen"`
	Port     int            `json:"port"`
	Protocol string         `json:"protocol"`
	Settings map[string]any `json:"settings,omitempty"`
	Sniffing *sniffConf     `json:"sniffing,omitempty"`
}

type sniffConf struct {
	Enabled      bool     `json:"enabled"`
	DestOverride []string `json:"destOverride"`
}

type outbound struct {
	Tag            string         `json:"tag"`
	Protocol       string         `json:"protocol"`
	Settings       map[string]any `json:"settings"`
	StreamSettings map[string]any `json:"streamSettings,omitempty"`
}

type routing struct {
	DomainStrategy string         `json:"domainStrategy,omitempty"`
	Balancers      []balancerConf `json:"balancers,omitempty"`
	Rules          []routeRule    `json:"rules,omitempty"`
}

type balancerConf struct {
	Tag      string         `json:"tag"`
	Selector []string       `json:"selector"`
	Strategy map[string]any `json:"strategy,omitempty"`
}

type routeRule struct {
	Type        string   `json:"type"`
	Domain      []string `json:"domain,omitempty"`
	IP          []string `json:"ip,omitempty"`
	Port        string   `json:"port,omitempty"`
	OutboundTag string   `json:"outboundTag,omitempty"`
	BalancerTag string   `json:"balancerTag,omitempty"`
	Network     string   `json:"network,omitempty"`
}

type dnsConf struct {
	Servers []any `json:"servers,omitempty"`
}

// buildXrayConfig assembles the full JSON config xray will run with, based on
// the user's saved AppConfig (servers + algorithm + routing rules).
func buildXrayConfig(cfg config.AppConfig) xrayConfig {
	xc := xrayConfig{
		Log: map[string]string{
			"loglevel": "warning",
		},
		Stats:  map[string]any{},
		Policy: defaultPolicy(),
	}

	// --- Inbounds: SOCKS + HTTP for clients on the LAN ---
	xc.Inbounds = []inbound{
		{
			Tag:      "socks-in",
			Listen:   listenAddr(cfg),
			Port:     cfg.Listen.SOCKSPort,
			Protocol: "socks",
			Settings: map[string]any{
				"auth": "noauth",
				"udp":  true,
			},
			Sniffing: &sniffConf{
				Enabled:      true,
				DestOverride: []string{"http", "tls"},
			},
		},
		{
			Tag:      "http-in",
			Listen:   listenAddr(cfg),
			Port:     cfg.Listen.HTTPPort,
			Protocol: "http",
			Settings: map[string]any{
				"allowTransparent": false,
			},
			Sniffing: &sniffConf{
				Enabled:      true,
				DestOverride: []string{"http", "tls"},
			},
		},
	}

	// --- Outbounds: one per upstream server, plus direct + blackhole ---
	selector := make([]string, 0, len(cfg.Servers))
	for i := range cfg.Servers {
		s := cfg.Servers[i]
		if !s.Enabled {
			continue
		}
		ob := buildOutbound(s)
		xc.Outbounds = append(xc.Outbounds, ob)
		selector = append(selector, ob.Tag)
	}
	// "direct" outbound for traffic that should bypass the proxy
	xc.Outbounds = append(xc.Outbounds, outbound{
		Tag:      "direct",
		Protocol: "freedom",
		Settings: map[string]any{},
	})
	// "block" for ad/malware blocking rules
	xc.Outbounds = append(xc.Outbounds, outbound{
		Tag:      "block",
		Protocol: "blackhole",
		Settings: map[string]any{},
	})

	// --- Routing: balancer + custom rules ---
	r := &routing{
		DomainStrategy: "AsIs",
	}
	if len(selector) > 0 {
		r.Balancers = []balancerConf{
			{
				Tag:      "balanced",
				Selector: selector,
				Strategy: balancerStrategy(cfg.Algorithm),
			},
		}
		// Default catch-all: send everything through the balancer
		r.Rules = append(r.Rules, routeRule{
			Type:        "field",
			Network:     "tcp,udp",
			BalancerTag: "balanced",
		})
	}
	// User-defined routing rules
	for _, rule := range cfg.RoutingRules {
		if !rule.Enabled {
			continue
		}
		rr := routeRule{Type: "field"}
		if len(rule.Domains) > 0 {
			rr.Domain = rule.Domains
		}
		if rule.GeoIP != "" {
			rr.IP = []string{"geoip:" + strings.ToLower(rule.GeoIP)}
		}
		switch rule.Target {
		case "direct":
			rr.OutboundTag = "direct"
		case "block":
			rr.OutboundTag = "block"
		case "balanced", "":
			rr.BalancerTag = "balanced"
		default:
			// Treat as outbound tag (e.g. specific server tag)
			rr.OutboundTag = rule.Target
		}
		// Prepend custom rules so they override the catch-all
		r.Rules = append([]routeRule{rr}, r.Rules...)
	}
	xc.Routing = r

	// --- DNS (optional, only if configured) ---
	if len(cfg.DNS.Servers) > 0 {
		dnsServers := make([]any, 0, len(cfg.DNS.Servers))
		for _, s := range cfg.DNS.Servers {
			dnsServers = append(dnsServers, s)
		}
		xc.DNS = &dnsConf{Servers: dnsServers}
	}

	return xc
}

// listenAddr returns the address xray should listen on for inbound proxies.
// Default is loopback (safe). When the user explicitly sets a wider listen
// address (e.g. 0.0.0.0 to share the proxy on the LAN), honor it.
func listenAddr(cfg config.AppConfig) string {
	if cfg.Listen.Address != "" {
		return cfg.Listen.Address
	}
	return "127.0.0.1"
}

// buildOutbound generates the xray outbound for one upstream server.
func buildOutbound(s config.Server) outbound {
	switch s.Protocol {
	case "vless":
		return buildVlessOutbound(s)
	case "vmess":
		return buildVmessOutbound(s)
	case "trojan":
		return buildTrojanOutbound(s)
	}
	// Fallback: treat as freedom so xray validates
	return outbound{Tag: s.Tag, Protocol: "freedom", Settings: map[string]any{}}
}

func buildVlessOutbound(s config.Server) outbound {
	return outbound{
		Tag:      s.Tag,
		Protocol: "vless",
		Settings: map[string]any{
			"vnext": []map[string]any{
				{
					"address": s.Address,
					"port":    s.Port,
					"users": []map[string]any{
						{
							"id":         s.UUID,
							"encryption": defaultStr(s.Encryption, "none"),
							"flow":       s.Flow,
						},
					},
				},
			},
		},
		StreamSettings: buildStreamSettings(s),
	}
}

func buildVmessOutbound(s config.Server) outbound {
	return outbound{
		Tag:      s.Tag,
		Protocol: "vmess",
		Settings: map[string]any{
			"vnext": []map[string]any{
				{
					"address": s.Address,
					"port":    s.Port,
					"users": []map[string]any{
						{
							"id":       s.UUID,
							"alterId":  0,
							"security": defaultStr(s.Encryption, "auto"),
						},
					},
				},
			},
		},
		StreamSettings: buildStreamSettings(s),
	}
}

func buildTrojanOutbound(s config.Server) outbound {
	return outbound{
		Tag:      s.Tag,
		Protocol: "trojan",
		Settings: map[string]any{
			"servers": []map[string]any{
				{
					"address":  s.Address,
					"port":     s.Port,
					"password": s.UUID, // trojan uses password in UUID slot
				},
			},
		},
		StreamSettings: buildStreamSettings(s),
	}
}

// buildStreamSettings constructs xray's `streamSettings` block from a server's
// transport + security parameters.
func buildStreamSettings(s config.Server) map[string]any {
	ss := map[string]any{
		"network":  defaultStr(s.Network, "tcp"),
		"security": defaultStr(s.Security, "none"),
	}

	// TLS / Reality
	switch s.Security {
	case "tls":
		tls := map[string]any{}
		if s.SNI != "" {
			tls["serverName"] = s.SNI
		}
		if s.Fingerprint != "" {
			tls["fingerprint"] = s.Fingerprint
		}
		if s.ALPN != "" {
			tls["alpn"] = strings.Split(s.ALPN, ",")
		}
		if s.AllowInsecure {
			tls["allowInsecure"] = true
		}
		ss["tlsSettings"] = tls
	case "reality":
		rt := map[string]any{
			"serverName":  s.SNI,
			"fingerprint": defaultStr(s.Fingerprint, "chrome"),
			"publicKey":   s.PublicKey,
			"shortId":     s.ShortID,
		}
		if s.SpiderX != "" {
			rt["spiderX"] = s.SpiderX
		}
		ss["realitySettings"] = rt
	}

	// Per-transport extras
	switch s.Network {
	case "ws":
		hdr := map[string]any{}
		if s.Host != "" {
			hdr["Host"] = s.Host
		}
		ss["wsSettings"] = map[string]any{
			"path":    defaultStr(s.Path, "/"),
			"headers": hdr,
		}
	case "grpc":
		ss["grpcSettings"] = map[string]any{
			"serviceName": s.ServiceName,
			"multiMode":   s.GrpcMulti,
		}
	case "h2", "http":
		ss["httpSettings"] = map[string]any{
			"path": defaultStr(s.Path, "/"),
			"host": []string{s.Host},
		}
	case "httpupgrade":
		ss["httpupgradeSettings"] = map[string]any{
			"path": defaultStr(s.Path, "/"),
			"host": s.Host,
		}
	case "tcp":
		if s.HeaderType == "http" {
			ss["tcpSettings"] = map[string]any{
				"header": map[string]any{
					"type": "http",
				},
			}
		}
	}

	return ss
}

// balancerStrategy maps OutBalancer's algorithm IDs to xray's strategy block.
func balancerStrategy(alg string) map[string]any {
	switch alg {
	case "latency", "leastping":
		return map[string]any{"type": "leastPing"}
	case "leastload":
		return map[string]any{"type": "leastLoad"}
	case "speed":
		return map[string]any{"type": "leastLoad"}
	case "roundrobin":
		return map[string]any{"type": "roundRobin"}
	case "random":
		return map[string]any{"type": "random"}
	}
	// default
	return map[string]any{"type": "leastPing"}
}

// defaultPolicy enables stats collection so we can read per-outbound bytes.
func defaultPolicy() map[string]any {
	return map[string]any{
		"levels": map[string]any{
			"0": map[string]any{
				"statsUserUplink":   true,
				"statsUserDownlink": true,
			},
		},
		"system": map[string]any{
			"statsInboundUplink":    true,
			"statsInboundDownlink":  true,
			"statsOutboundUplink":   true,
			"statsOutboundDownlink": true,
		},
	}
}

func defaultStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
