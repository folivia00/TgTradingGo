package web

import (
	"encoding/json"
	"net/http"
)

// ====== API: Control & Status ======

// POST /api/ctrl/switch_feed {"feed":"random"|"rest"}
func (s *Server) handleSwitchFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Feed string `json:"feed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.Feed == "" {
		http.Error(w, "feed required", http.StatusBadRequest)
		return
	}
	if s.OnSwitchFeed == nil {
		http.Error(w, "not bound", http.StatusNotImplemented)
		return
	}
	if err := s.OnSwitchFeed(body.Feed); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// POST /api/ctrl/save_state
func (s *Server) handleSaveState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	if s.OnSaveState == nil {
		http.Error(w, "not bound", http.StatusNotImplemented)
		return
	}
	if err := s.OnSaveState(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// POST /api/ctrl/load_state
func (s *Server) handleLoadState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	if s.OnLoadState == nil {
		http.Error(w, "not bound", http.StatusNotImplemented)
		return
	}
	if err := s.OnLoadState(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// POST /api/ctrl/reset_state
func (s *Server) handleResetState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	if s.OnResetState == nil {
		http.Error(w, "not bound", http.StatusNotImplemented)
		return
	}
	if err := s.OnResetState(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// GET /api/status -> текущее состояние (mode/symbol/tf/feed/equity)
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if s.GetStatus == nil {
		http.Error(w, "not bound", http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.GetStatus())
}
