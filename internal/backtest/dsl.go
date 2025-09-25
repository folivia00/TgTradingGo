package backtest

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type DSLDoc struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Body []byte
}

var (
	dslMu       sync.Mutex
	dslDocs     = map[string]DSLDoc{}
	selectedDSL string
)

func SaveDSLDoc(name string, body []byte) DSLDoc {
	if name == "" {
		name = "strategy.dsl"
	}
	id := "dsl_" + randomID(10)
	doc := DSLDoc{ID: id, Name: name, Body: append([]byte(nil), body...)}
	dslMu.Lock()
	dslDocs[id] = doc
	dslMu.Unlock()
	return doc
}

func ListDSLDocs() []DSLDoc {
	dslMu.Lock()
	defer dslMu.Unlock()
	out := make([]DSLDoc, 0, len(dslDocs))
	for _, doc := range dslDocs {
		out = append(out, doc)
	}
	return out
}

func GetDSLDoc(id string) (DSLDoc, bool) {
	dslMu.Lock()
	defer dslMu.Unlock()
	doc, ok := dslDocs[id]
	return doc, ok
}

func SelectDSL(id string) bool {
	dslMu.Lock()
	defer dslMu.Unlock()
	if _, ok := dslDocs[id]; !ok {
		return false
	}
	selectedDSL = id
	return true
}

func SelectedDSL() string {
	dslMu.Lock()
	defer dslMu.Unlock()
	return selectedDSL
}

type dslSpec struct {
	Kind string
	Args map[string]float64
}

func compileDSL(body []byte) (dslSpec, error) {
	spec := dslSpec{Kind: "ema_atr", Args: map[string]float64{}}
	root, err := parseDSLDocument(body)
	if err != nil {
		return spec, err
	}
	if v, ok := root["strategy"]; ok {
		spec.Kind = strings.ToLower(fmt.Sprint(v))
	}
	params := map[string]any{}
	if rawParams, ok := root["params"].(map[string]any); ok {
		for k, v := range rawParams {
			params[k] = v
		}
	}
	for k, v := range root {
		kl := strings.ToLower(k)
		if kl == "strategy" || kl == "name" || kl == "title" || kl == "params" {
			continue
		}
		if _, exists := params[k]; !exists {
			params[k] = v
		}
	}
	for k, v := range params {
		if f, ok := toFloat(v); ok {
			key := k
			spec.Args[key] = f
			lower := strings.ToLower(k)
			if lower != key {
				spec.Args[lower] = f
			}
		}
	}
	if v, ok := spec.Args["r"]; ok {
		spec.Args["R"] = v
	}
	return spec, nil
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint64:
		return float64(val), true
	case json.Number:
		if f, err := val.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func parseDSLDocument(body []byte) (map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err == nil {
		return root, nil
	}
	return parseNaiveYAML(body)
}

type yamlFrame struct {
	indent int
	data   map[string]any
}

func parseNaiveYAML(body []byte) (map[string]any, error) {
	root := map[string]any{}
	stack := []yamlFrame{{indent: -1, data: root}}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		for len(stack) > 1 && indent <= stack[len(stack)-1].indent {
			stack = stack[:len(stack)-1]
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		parent := stack[len(stack)-1].data
		if val == "" {
			child := map[string]any{}
			parent[key] = child
			stack = append(stack, yamlFrame{indent: indent, data: child})
			continue
		}
		parent[key] = parseScalarValue(val)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return root, nil
}

func parseScalarValue(val string) any {
	v := strings.TrimSpace(val)
	if strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
		v = strings.Trim(v, "\"")
		return v
	}
	if strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'") {
		v = strings.Trim(v, "'")
		return v
	}
	if strings.EqualFold(v, "true") {
		return true
	}
	if strings.EqualFold(v, "false") {
		return false
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return v
}

func randomID(n int) string {
	if n <= 0 {
		n = 8
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)[:n]
	}
	alphabet := "abcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, n)
	for i := range out {
		out[i] = alphabet[(i+len(alphabet))%len(alphabet)]
	}
	return string(out)
}
