// Package i18n provides internationalization support for AuraGo.
// It handles language normalization, loading of translation files, and
// provides lookup functions for both frontend injection and backend use.
package i18n

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"strings"
	"sync"
)

// Store holds the parsed translations, keyed by language code.
// Each language maps to a flat key-value pairs.
type Store struct {
	mu       sync.RWMutex
	langData map[string]map[string]string // lang -> key -> translation
	metaJSON string                       // raw JSON for field metadata
	langJSON map[string]string            // lang -> marshaled JSON string
}

// Global store instance used by the Load, GetJSON, GetMetaJSON functions.
// Server code should use these package-level functions rather than
// creating separate Store instances to avoid duplication.
var (
	globalStore = &Store{
		langData: make(map[string]map[string]string),
		langJSON: make(map[string]string),
	}
)

// Load reads translation files from the provided filesystem and prepares
// per-language JSON blobs for template injection. Files are organized in
// category subdirectories under the root (e.g., lang/chat/, lang/help/).
// Each <lang>.json contains a flat {key: translation} map.
// The special file meta.json in the root holds field-option metadata.
//
// The lang directory structure expected:
//
//	ui/lang/
//	  meta.json                    - field metadata
//	  chat/en.json, chat/de.json   - chat UI strings
//	  help/en.json, help/de.json   - help page strings
//	  setup/en.json, setup/de.json - setup wizard strings
//	  skills/en.json, skills/de.json
//	  truenas/en.json, truenas/de.json
//	  config/en.json, config/de.json
func Load(uiFS fs.FS, logger *slog.Logger) {
	globalStore.load(uiFS, logger)
}

// load implements the Load logic on the receiver store.
func (s *Store) load(uiFS fs.FS, logger *slog.Logger) {
	newLangJSON := make(map[string]string)
	langData := make(map[string]map[string]string)
	var newMetaJSON string

	entries, err := fs.ReadDir(uiFS, "lang")
	if err != nil {
		logger.Error("Failed to read lang/ directory", "error", err)
		s.mu.Lock()
		s.langJSON = map[string]string{"en": "{}"}
		s.metaJSON = "{}"
		s.mu.Unlock()
		return
	}

	// Read meta.json from root
	metaData, err := fs.ReadFile(uiFS, "lang/meta.json")
	if err == nil {
		newMetaJSON = string(metaData)
	} else {
		newMetaJSON = "{}"
	}

	// Process subdirectories recursively
	var processDir func(path string)
	processDir = func(dirPath string) {
		entries, err := fs.ReadDir(uiFS, dirPath)
		if err != nil {
			logger.Warn("Failed to read directory", "path", dirPath, "error", err)
			return
		}

		for _, e := range entries {
			itemPath := dirPath + "/" + e.Name()

			if e.IsDir() {
				processDir(itemPath)
			} else if strings.HasSuffix(e.Name(), ".json") {
				lang := strings.TrimSuffix(e.Name(), ".json")
				data, err := fs.ReadFile(uiFS, itemPath)
				if err != nil {
					logger.Warn("Failed to read lang file", "file", itemPath, "error", err)
					continue
				}

				var translations map[string]string
				if err := json.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &translations); err != nil {
					logger.Warn("Failed to parse lang file", "file", itemPath, "error", err)
					continue
				}

				if langData[lang] == nil {
					langData[lang] = make(map[string]string)
				}
				for key, value := range translations {
					langData[lang][key] = value
				}
			}
		}
	}

	// Process all subdirectories in lang/
	for _, e := range entries {
		if e.IsDir() {
			processDir("lang/" + e.Name())
		}
	}

	// Convert merged data to JSON strings
	for lang, translations := range langData {
		jsonBytes, err := json.Marshal(translations)
		if err != nil {
			logger.Warn("Failed to marshal translations", "lang", lang, "error", err)
			continue
		}
		newLangJSON[lang] = string(jsonBytes)
	}

	if len(newLangJSON) == 0 {
		newLangJSON = map[string]string{"en": "{}"}
	}

	s.mu.Lock()
	s.langData = langData
	s.langJSON = newLangJSON
	s.metaJSON = newMetaJSON
	s.mu.Unlock()

	logger.Info("i18n loaded", "languages", len(newLangJSON))
}

// GetJSON returns the JSON string for the given language, falling back to "en".
// The result is suitable for injection into frontend templates as a JS object.
func GetJSON(lang string) template.JS {
	return globalStore.GetJSON(lang)
}

// GetJSON returns the JSON string for the given language, falling back to "en".
func (s *Store) GetJSON(lang string) template.JS {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if j, ok := s.langJSON[lang]; ok {
		return template.JS(j)
	}
	if j, ok := s.langJSON["en"]; ok {
		return template.JS(j)
	}
	return template.JS("{}")
}

// GetMetaJSON returns the _meta section JSON for config_help metadata.
func GetMetaJSON() template.JS {
	return globalStore.GetMetaJSON()
}

// GetMetaJSON returns the _meta section JSON for config_help metadata.
func (s *Store) GetMetaJSON() template.JS {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return template.JS(s.metaJSON)
}

// NormalizeLang converts a human-readable language string to an ISO code
// used by the translation system. It handles full language names, demonyms,
// and ISO codes in both upper and lower case.
//
// Supported languages: cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh
//
// Examples:
//   - "German", "Deutsch", "german", "de" -> "de"
//   - "ENGLISH", "en" -> "en"
//   - "Japanese", "日本語", "ja" -> "ja"
//   - "unknown" -> "en" (fallback)
func NormalizeLang(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	switch {
	case strings.Contains(l, "german") || strings.Contains(l, "deutsch") || l == "de":
		return "de"
	case strings.Contains(l, "english") || l == "en":
		return "en"
	case strings.Contains(l, "spanish") || strings.Contains(l, "español") || l == "es":
		return "es"
	case strings.Contains(l, "french") || strings.Contains(l, "français") || l == "fr":
		return "fr"
	case strings.Contains(l, "polish") || strings.Contains(l, "polski") || l == "pl":
		return "pl"
	case strings.Contains(l, "chinese") || strings.Contains(l, "mandarin") || l == "zh":
		return "zh"
	case strings.Contains(l, "hindi") || l == "hi":
		return "hi"
	case strings.Contains(l, "dutch") || strings.Contains(l, "nederlands") || l == "nl":
		return "nl"
	case strings.Contains(l, "italian") || strings.Contains(l, "italiano") || l == "it":
		return "it"
	case strings.Contains(l, "portuguese") || strings.Contains(l, "português") || l == "pt":
		return "pt"
	case strings.Contains(l, "danish") || strings.Contains(l, "dansk") || l == "da":
		return "da"
	case strings.Contains(l, "japanese") || strings.Contains(l, "日本語") || l == "ja":
		return "ja"
	case strings.Contains(l, "swedish") || strings.Contains(l, "svenska") || l == "sv":
		return "sv"
	case strings.Contains(l, "norwegian") || strings.Contains(l, "norsk") || l == "no":
		return "no"
	case strings.Contains(l, "czech") || strings.Contains(l, "čeština") || l == "cs":
		return "cs"
	case strings.Contains(l, "greek") || strings.Contains(l, "ελληνικά") || l == "el":
		return "el"
	default:
		return "en" // Fallback
	}
}

// T returns the translation for the given key in the specified language.
// If the key is not found in the requested language, it falls back to English.
// If the key is not found in English either, the key itself is returned.
// Parameters can be interpolated using {name} placeholders.
func T(lang, key string, params ...any) string {
	return globalStore.T(lang, key, params...)
}

// T returns the translation for the given key, with parameter interpolation support.
func (s *Store) T(lang, key string, params ...any) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Try requested language first
	normalizedLang := NormalizeLang(lang)
	if translations, ok := s.langData[normalizedLang]; ok {
		if val, ok := translations[key]; ok {
			return interpolate(val, params)
		}
	}

	// Fall back to English
	if translations, ok := s.langData["en"]; ok {
		if val, ok := translations[key]; ok {
			return interpolate(val, params)
		}
	}

	// Last resort: return the key itself
	return key
}

// interpolate replaces {name} placeholders with the corresponding parameter values.
// Parameters can be strings or types that format with fmt.Sprint.
func interpolate(s string, params []any) string {
	if len(params) == 0 {
		return s
	}

	result := s
	for i, p := range params {
		placeholder := fmt.Sprintf("{%d}", i)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprint(p))
	}

	// Also support named parameters if first param is a map
	if len(params) > 0 {
		if m, ok := params[0].(map[string]any); ok {
			for k, v := range m {
				placeholder := fmt.Sprintf("{%s}", k)
				result = strings.ReplaceAll(result, placeholder, fmt.Sprint(v))
			}
		}
	}

	return result
}

// GetSupportedLanguages returns a list of language codes that have translations loaded.
func GetSupportedLanguages() []string {
	return globalStore.GetSupportedLanguages()
}

// GetSupportedLanguages returns the list of supported language codes.
func (s *Store) GetSupportedLanguages() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	languages := make([]string, 0, len(s.langData))
	for lang := range s.langData {
		languages = append(languages, lang)
	}
	return languages
}

// HasKey checks if a translation key exists in the given language or English.
func HasKey(lang, key string) bool {
	return globalStore.HasKey(lang, key)
}

// HasKey checks if a translation key exists in the given language or English.
func (s *Store) HasKey(lang, key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normalizedLang := NormalizeLang(lang)
	if translations, ok := s.langData[normalizedLang]; ok {
		if _, exists := translations[key]; exists {
			return true
		}
	}

	if translations, ok := s.langData["en"]; ok {
		if _, exists := translations[key]; exists {
			return true
		}
	}

	return false
}
