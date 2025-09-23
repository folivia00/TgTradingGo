package logx

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

var (
	currentLevel = levelInfo
	mu           sync.Mutex
)

const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
)

type leveledWriter struct {
	minLevel int
	target   io.Writer
}

func (w *leveledWriter) Write(p []byte) (int, error) {
	lvl := levelFromMessage(string(p))
	if lvl < w.minLevel {
		return len(p), nil
	}
	return w.target.Write(p)
}

func Setup(level string) {
	mu.Lock()
	defer mu.Unlock()
	switch strings.ToLower(level) {
	case "debug":
		currentLevel = levelDebug
	case "warn":
		currentLevel = levelWarn
	case "error":
		currentLevel = levelError
	default:
		currentLevel = levelInfo
	}
	log.SetOutput(&leveledWriter{minLevel: currentLevel, target: os.Stdout})
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func levelFromMessage(msg string) int {
	msg = strings.ToLower(msg)
	switch {
	case strings.Contains(msg, "error"):
		return levelError
	case strings.Contains(msg, "warn"):
		return levelWarn
	case strings.Contains(msg, "debug"):
		return levelDebug
	default:
		return levelInfo
	}
}
