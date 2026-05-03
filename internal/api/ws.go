package api

import (
	"encoding/json"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/outbalancer/outbalancer/internal/store"
)

// Hub broadcasts live updates over WebSocket.
type Hub struct {
	mu        sync.Mutex
	clients   map[*websocket.Conn]struct{}
	upgrader  websocket.Upgrader
}

// NewHub returns a configured Hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*websocket.Conn]struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
			ReadBufferSize: 1024, WriteBufferSize: 4096,
		},
	}
}

// ServeWS upgrades the request to a WebSocket connection.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			_ = conn.Close()
		}()
		conn.SetReadLimit(1024)
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

// Broadcast sends a JSON-serialisable message to all connected clients.
func (h *Hub) Broadcast(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			delete(h.clients, c)
			_ = c.Close()
		}
	}
}

// StartBroadcaster starts a goroutine that sends periodic snapshots to clients.
func (h *Hub) StartBroadcaster(s *store.Store) {
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for range t.C {
			servers := s.Servers()
			metrics := s.Metrics()
			snapshot := map[string]any{
				"type":    "tick",
				"time":    time.Now().Unix(),
				"servers": []map[string]any{},
				"alerts":  s.Alerts()[:min(5, len(s.Alerts()))],
			}
			for _, srv := range servers {
				m := metrics[srv.ID]
				snapshot["servers"] = append(snapshot["servers"].([]map[string]any),
					mergeServerMetrics(srv, m))
			}
			// also push the last traffic sample
			tf := s.Traffic()
			if len(tf) > 0 {
				snapshot["traffic"] = tf[len(tf)-1]
			}
			h.Broadcast(snapshot)
		}
	}()
}

// loggingMiddleware logs each request to the in-memory log buffer.
func loggingMiddleware(s *store.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Common health-check noise — skip
		if r.URL.Path == "/api/health" || r.URL.Path == "/ws" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, code: 200}
		next.ServeHTTP(rec, r)
		ms := time.Since(start).Milliseconds()
		level := "info"
		if rec.code >= 400 {
			level = "warn"
		}
		s.AddLog(level, "http", r.Method+" "+r.URL.Path+" -> "+strconv.Itoa(rec.code)+" ("+strconv.FormatInt(ms, 10)+"ms)")
	})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (s *statusRecorder) WriteHeader(c int) {
	s.code = c
	s.ResponseWriter.WriteHeader(c)
}

// tcpProbe performs a TCP connect to host:port with the given timeout.
func tcpProbe(host string, port int, timeout time.Duration) error {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

func runtimeGoroutines() int { return runtime.NumGoroutine() }

func runtimeAllocMB() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024.0 / 1024.0
}
