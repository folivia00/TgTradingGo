package web

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

//go:embed ui/*
var uiFS embed.FS

type Server struct {
	BotToken        string
	Addr            string
	DevMode         bool
	DefaultExchange string
	CurSymbol       string
	CurTF           string
	CurMode         string

	hub         *sseHub
	stop        chan struct{}
	mu          sync.Mutex
	selectedDSL string

	// callbacks bound from main.go
	OnSwitchFeed func(string) error
	OnSaveState  func() error
	OnLoadState  func() error
	OnResetState func() error
	GetStatus    func() any
	OnSetSymbol  func(symbol, tf, mode string) error
}

func NewServer(botToken, addr string, dev bool) *Server {
	if addr == "" {
		addr = ":8080"
	}
	return &Server{BotToken: botToken, Addr: addr, DevMode: dev, hub: newHub(), stop: make(chan struct{})}
}

func (s *Server) Serve() error {
	mux := http.NewServeMux()
	// static
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		b, err := uiFS.ReadFile("ui/index.html")
		if err != nil {
			http.Error(w, "index missing", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("/ui/app.js", func(w http.ResponseWriter, r *http.Request) {
		b, err := uiFS.ReadFile("ui/app.js")
		if err != nil {
			http.Error(w, "app missing", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write(b)
	})
	// API
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/backtest", s.handleBacktest)
	mux.HandleFunc("/api/export", s.handleExport)
	mux.HandleFunc("/api/file", s.handleFile)
	mux.HandleFunc("/api/strategy/upload", s.handleStrategyUpload)
	mux.HandleFunc("/api/strategy/list", s.handleStrategyList)
	mux.HandleFunc("/api/strategy/select", s.handleStrategySelect)
	// control
	mux.HandleFunc("/api/ctrl/switch_feed", s.handleSwitchFeed)
	mux.HandleFunc("/api/ctrl/save_state", s.handleSaveState)
	mux.HandleFunc("/api/ctrl/load_state", s.handleLoadState)
	mux.HandleFunc("/api/ctrl/reset_state", s.handleResetState)
	mux.HandleFunc("/api/ctrl/set_symbol", s.handleSetSymbol)
	mux.HandleFunc("/api/ctrl/sim_trade", s.handleSimTrade)
	mux.HandleFunc("/api/status", s.handleStatus)
	// SSE
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) { s.hub.Subscribe(w, r) })

	h := s.authMiddleware(mux)
	log.Printf("web: listening on %s (dev=%v)", s.Addr, s.DevMode)
	go s.mockTicker()
	return http.ListenAndServe(s.Addr, h)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/ui/") {
			next.ServeHTTP(w, r)
			return
		}
		if s.DevMode || s.BotToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		initData := r.Header.Get("X-TG-Init-Data")
		if initData == "" {
			initData = r.URL.Query().Get("initData")
		}
		if !ValidateInitData(initData, s.BotToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) PublishJSON(line string) { s.hub.Broadcast(line) }
func (s *Server) Stop()                   { close(s.stop) }

func (s *Server) setSelectedDSL(id string) {
	s.mu.Lock()
	s.selectedDSL = id
	s.mu.Unlock()
}

func (s *Server) SelectedDSL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.selectedDSL
}

func EnvDev() bool {
	return strings.ToLower(os.Getenv("DEV_MODE")) == "true"
}

func EnvAddr() string {
	if v := os.Getenv("WEB_ADDR"); v != "" {
		return v
	}
	return ":8080"
}
