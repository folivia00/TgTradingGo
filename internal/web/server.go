package web

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

//go:embed ui/*
var uiFS embed.FS

type Server struct {
	BotToken string
	Addr     string
	DevMode  bool // если true, ослабляем проверку initData для локалки

	hub  *sseHub
	stop chan struct{}
}

func NewServer(botToken, addr string, dev bool) *Server {
	if addr == "" {
		addr = ":8080"
	}
	return &Server{BotToken: botToken, Addr: addr, DevMode: dev, hub: newHub(), stop: make(chan struct{})}
}

func (s *Server) Serve() error {
	mux := http.NewServeMux()
	// статика (SPA)
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
	// ping
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	// SSE
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		s.hub.Subscribe(w, r)
	})

	// auth middleware (только для /api/* и /sse); для W1 включаем мягко
	h := s.authMiddleware(mux)
	log.Printf("web: listening on %s (dev=%v)", s.Addr, s.DevMode)
	go s.mockTicker()
	return http.ListenAndServe(s.Addr, h)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Пропускаем статику
		if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/ui/") {
			next.ServeHTTP(w, r)
			return
		}
		if s.DevMode || s.BotToken == "" {
			// Локальная разработка без проверки
			next.ServeHTTP(w, r)
			return
		}
		// Ищем initData: из заголовка X-TG-Init-Data или query ?initData=...
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

// Публичные методы для публикации событий (подключим на W2 к свечам/сделкам)
func (s *Server) PublishJSON(line string) { s.hub.Broadcast(line) }
func (s *Server) Stop()                   { close(s.stop) }

// Helper: читать DEV_MODE/WEB_ADDR из env (если нужно в main)
func EnvDev() bool { return strings.ToLower(os.Getenv("DEV_MODE")) == "true" }
func EnvAddr() string {
	if v := os.Getenv("WEB_ADDR"); v != "" {
		return v
	}
	return ":8080"
}
