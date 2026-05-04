package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/outbalancer/outbalancer/internal/api"
	"github.com/outbalancer/outbalancer/internal/balancer"
	"github.com/outbalancer/outbalancer/internal/store"
	"github.com/outbalancer/outbalancer/internal/xray"
	"github.com/outbalancer/outbalancer/web"
)

// version is injected by the build via -ldflags="-X main.version=..."
var version = "dev"

const banner = `
╔═══════════════════════════════════════════════════╗
║         OutBalancer · Premium V2Ray Balancer      ║
║         single-binary · embedded xray-core        ║
╚═══════════════════════════════════════════════════╝
`

func main() {
	var (
		port      int
		dataDir   string
		demoMode  bool
		socksPort int
		httpPort  int
		listenAt  string
	)
	flag.IntVar(&port, "port", 8088, "Web panel port")
	flag.StringVar(&dataDir, "data", "", "Data directory (default: ~/.outbalancer)")
	flag.BoolVar(&demoMode, "demo", false, "Seed demo servers for showcasing the panel")
	flag.IntVar(&socksPort, "socks", 0, "Override SOCKS proxy port (default from saved config: 10808)")
	flag.IntVar(&httpPort, "http", 0, "Override HTTP proxy port (default from saved config: 10809)")
	flag.StringVar(&listenAt, "listen", "", "Bind address for proxies (default 127.0.0.1; use 0.0.0.0 to expose on LAN)")
	flag.Parse()

	fmt.Print(banner)
	fmt.Printf("  Version: %s\n", version)

	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dataDir = filepath.Join(home, ".outbalancer")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("mkdir data dir: %v", err)
	}

	// 1. Storage
	st, err := store.New(filepath.Join(dataDir, "config.json"))
	if err != nil {
		log.Fatalf("init store: %v", err)
	}

	// Apply CLI port overrides on top of persisted config
	cfg := st.Config()
	if cfg.Listen.SOCKSPort == 0 {
		cfg.Listen.SOCKSPort = 10808
	}
	if cfg.Listen.HTTPPort == 0 {
		cfg.Listen.HTTPPort = 10809
	}
	if socksPort > 0 {
		cfg.Listen.SOCKSPort = socksPort
	}
	if httpPort > 0 {
		cfg.Listen.HTTPPort = httpPort
	}
	if listenAt != "" {
		cfg.Listen.Address = listenAt
	}
	_ = st.SaveConfig(cfg)

	// 2. Optional demo data
	if demoMode && len(st.Servers()) == 0 {
		seedDemo(st)
	}

	// 3. Xray manager — embedded core, no subprocess.
	xm := xray.NewManager(st, "", filepath.Join(dataDir, "run"))
	if err := xm.Apply(); err != nil {
		log.Printf("xray apply: %v (panel will still serve)", err)
	}

	// 4. Balancer — synthetic traffic only when --demo is set
	bal := balancer.New(st)
	bal.SetDemoMode(demoMode)
	bal.Start()
	defer bal.Stop()

	// 5. HTTP API + WebSocket + embedded UI
	apiSrv := api.NewServer(st, bal, xm, web.Handler())
	apiSrv.Hub().StartBroadcaster(st)

	addr := fmt.Sprintf(":%d", port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           apiSrv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	bindLabel := cfg.Listen.Address
	if bindLabel == "" {
		bindLabel = "127.0.0.1"
	}
	fmt.Printf("\n  ➜ Panel:  http://localhost:%d\n", port)
	fmt.Printf("  ➜ Data:   %s\n", dataDir)
	fmt.Printf("  ➜ Xray:   ✓  embedded (xray-core in-process)\n")
	fmt.Printf("  ➜ SOCKS:  %s:%d\n", bindLabel, cfg.Listen.SOCKSPort)
	fmt.Printf("  ➜ HTTP:   %s:%d\n\n", bindLabel, cfg.Listen.HTTPPort)

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	fmt.Println("\nShutting down...")
	xm.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}
