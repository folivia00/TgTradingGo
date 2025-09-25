package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"tradebot/internal/backtest"
)

type strategyListItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Server) handleStrategyUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	name := r.URL.Query().Get("name")
	var body []byte
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "text/plain") {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read", http.StatusBadRequest)
			return
		}
		body = b
	} else {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			http.Error(w, "multipart", http.StatusBadRequest)
			return
		}
		file, hdr, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "file required", http.StatusBadRequest)
			return
		}
		defer file.Close()
		b, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "read", http.StatusBadRequest)
			return
		}
		body = b
		if name == "" && hdr != nil {
			name = hdr.Filename
		}
	}
	doc := backtest.SaveDSLDoc(name, body)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "id": doc.ID})
}

func (s *Server) handleStrategyList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	docs := backtest.ListDSLDocs()
	out := make([]strategyListItem, 0, len(docs))
	for _, d := range docs {
		out = append(out, strategyListItem{ID: d.ID, Name: d.Name})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) handleStrategySelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if !backtest.SelectDSL(req.ID) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	s.setSelectedDSL(req.ID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
