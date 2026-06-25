// Package i18n provides runtime-switchable English/Spanish translations.
// Translation maps are embedded into the binary (no external files needed).
package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed en.json
var enJSON []byte

//go:embed es.json
var esJSON []byte

//go:embed es_AR.json
var esARJSON []byte

var (
	mu   sync.RWMutex
	cur  = "en"
	maps = map[string]map[string]string{}
)

func init() {
	en := map[string]string{}
	es := map[string]string{}
	esAR := map[string]string{}
	_ = json.Unmarshal(enJSON, &en)
	_ = json.Unmarshal(esJSON, &es)
	_ = json.Unmarshal(esARJSON, &esAR)
	maps["en"] = en
	maps["es"] = es
	maps["es_AR"] = esAR
}

// SetLang switches the active language ("en" or "es"). Unknown codes are ignored.
func SetLang(code string) {
	mu.Lock()
	if _, ok := maps[code]; ok {
		cur = code
	}
	mu.Unlock()
}

// Lang returns the active language code.
func Lang() string {
	mu.RLock()
	defer mu.RUnlock()
	return cur
}

// Codes returns the supported language codes in display order.
func Codes() []string { return []string{"en", "es", "es_AR"} }

// DisplayName returns the human label for a language code.
func DisplayName(code string) string {
	switch code {
	case "es":
		return "Español"
	case "es_AR":
		return "Español (AR)"
	default:
		return "English"
	}
}

// T translates a key for the active language, falling back to English then the key.
func T(key string) string {
	mu.RLock()
	defer mu.RUnlock()
	if m, ok := maps[cur]; ok {
		if v, ok := m[key]; ok && v != "" {
			return v
		}
	}
	if v, ok := maps["en"][key]; ok && v != "" {
		return v
	}
	return key
}

// Tf is T followed by fmt.Sprintf for keys that contain format verbs.
func Tf(key string, a ...interface{}) string {
	return fmt.Sprintf(T(key), a...)
}
