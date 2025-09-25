package web

import (
	"encoding/json"
	"net/http"
	"time"
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

// POST /api/ctrl/set_symbol {"symbol":"BTCUSDT","tf":"1m","mode":"futures|spot"}
func (s *Server) handleSetSymbol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Symbol string `json:"symbol"`
		TF     string `json:"tf"`
		Mode   string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Symbol == "" || req.TF == "" {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if s.OnSetSymbol == nil {
		http.Error(w, "not bound", http.StatusNotImplemented)
		return
	}
	if err := s.OnSetSymbol(req.Symbol, req.TF, req.Mode); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// POST /api/ctrl/sim_trade {"side":"buy|sell","price":..., "qty":...}
func (s *Server) handleSimTrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Side  string  `json:"side"`
		Price float64 `json:"price"`
		Qty   float64 `json:"qty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Side != "buy" && req.Side != "sell") {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	ev := map[string]any{
		"type": "trade",
		"data": map[string]any{
			"ts":     time.Now().UTC().Format(time.RFC3339),
			"side":   req.Side,
			"price":  req.Price,
			"qty":    req.Qty,
			"pnl":    0.0,
			"fee":    0.0,
			"note":   "sim",
			"symbol": s.CurSymbol,
			"tf":     s.CurTF,
		},
	}
	b, _ := json.Marshal(ev)
	s.PublishJSON(string(b))
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
