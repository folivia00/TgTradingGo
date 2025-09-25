package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type StrategyState struct {
	Type string    `json:"type"`
	I    []int     `json:"i,omitempty"`
	F    []float64 `json:"f,omitempty"`
}

type FeedState struct {
	Type   string `json:"type"`
	Symbol string `json:"symbol"`
	TF     string `json:"tf"`
}

type State struct {
	Strategy StrategyState `json:"strategy"`
	Feed     FeedState     `json:"feed"`
}

func Default() State {
	return State{
		Strategy: StrategyState{Type: "ema", I: []int{9, 21, 14}, F: []float64{1.5}},
		Feed:     FeedState{Type: "random"},
	}
}

type Store struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return State{}, errors.New("empty state path")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, nil
		}
		return State{}, err
	}
	if len(data) == 0 {
		return State{}, nil
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, err
	}
	return st, nil
}

func (s *Store) Save(st State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return errors.New("empty state path")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *Store) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return errors.New("empty state path")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, []byte{}, 0o644)
}
