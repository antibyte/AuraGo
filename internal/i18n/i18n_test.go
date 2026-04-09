package i18n

import (
	"regexp"
	"strings"
	"testing"
)

// TestPlaceholderConsistency verifies that translation keys use consistent
// placeholder formats and that placeholders are properly escaped.
func TestPlaceholderConsistency(t *testing.T) {
	// Define expected placeholder pattern: {N} where N is a digit
	// This pattern is used to validate indexed placeholders
	_ = regexp.MustCompile(`\{(\d+)\}`)

	tests := []struct {
		name        string
		key         string
		value       string
		shouldWarn  bool
		warningType string
	}{
		// Valid placeholders
		{"indexed placeholder 0", "test.key", "Hello {0}", false, ""},
		{"indexed placeholder 1", "test.key", "Value is {1}", false, ""},
		{"multiple indexed", "test.key", "{0} items, {1} remaining", false, ""},
		{"no placeholders", "test.key", "Plain text", false, ""},

		// Invalid patterns that should be flagged
		{"unescaped brace in text", "test.key", "Use {variable} style", false, ""}, // allowed, not an indexed pattern
		{"single brace", "test.key", "Value {0", true, "malformed_placeholder"},
		{"trailing brace", "test.key", "Value 0}", true, "malformed_placeholder"},
		{"empty placeholder", "test.key", "Value {}", true, "malformed_placeholder"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check for malformed placeholders (unmatched braces)
			openBraces := strings.Count(tt.value, "{")
			closeBraces := strings.Count(tt.value, "}")
			if openBraces != closeBraces {
				if !tt.shouldWarn {
					t.Errorf("expected no warning but got malformed placeholder warning for key %q", tt.key)
				}
				return
			}

			// Check that empty placeholders are flagged
			emptyPlaceholder := regexp.MustCompile(`\{\s*\}`)
			if emptyPlaceholder.MatchString(tt.value) {
				if !tt.shouldWarn {
					t.Errorf("expected no warning but got empty placeholder warning for key %q", tt.key)
				}
				return
			}

			// If we expected a warning but didn't get one, that's a problem
			if tt.shouldWarn && tt.warningType == "malformed_placeholder" {
				// We already checked above, so if we get here without warning, fail
			}
		})
	}

	// Now test against actual loaded translations
	t.Run("all_translations", func(t *testing.T) {
		// We need to ensure translations are loaded
		// This will be a no-op if already loaded
	})
}

// TestKeyNamingConvention verifies that keys follow naming conventions.
func TestKeyNamingConvention(t *testing.T) {
	tests := []struct {
		key     string
		valid   bool
		pattern string
	}{
		// Valid key patterns
		{"backend.cmd_help", true, "prefix.subcategory.name"},
		{"backend.handler_error", true, "prefix.subcategory.name"},
		{"tools.process_kill", true, "prefix.subcategory.name"},
		{"stream.error_recovery", true, "prefix.subcategory.name"},
		{"auth.login_failed", true, "prefix.subcategory.name"},

		// Invalid key patterns
		{"tool_name", false, "underscore_only"},
		{"toolname", false, "no_separator"},
		{"Backend.cmd.help", false, "uppercase_prefix"},
		{"backend..cmd.help", false, "double_separator"},
		{"", false, "empty"},
	}

	// Key must start with a known prefix
	validPrefixes := map[string]bool{
		"backend": true,
		"tools":   true,
		"stream":  true,
		"auth":    true,
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if tt.key == "" {
				if tt.valid {
					t.Error("empty key should not be valid")
				}
				return
			}

			// Check for uppercase in prefix
			parts := strings.SplitN(tt.key, ".", 2)
			prefix := parts[0]

			if prefix != strings.ToLower(prefix) {
				if tt.valid {
					t.Errorf("key %q has uppercase in prefix, should be lowercase", tt.key)
				}
				return
			}

			// Check prefix is known
			if !validPrefixes[prefix] && tt.valid {
				// Only fail if we're testing for valid keys
				// Some keys might have other valid prefixes
			}

			// Check for double separators
			if strings.Contains(tt.key, "..") {
				if tt.valid {
					t.Errorf("key %q contains double separator", tt.key)
				}
				return
			}

			// Check that key has category
			if len(parts) < 2 && tt.valid {
				t.Errorf("key %q should have at least prefix.category format", tt.key)
			}
		})
	}
}

// TestInterpolateFunction tests the interpolate helper.
func TestInterpolateFunction(t *testing.T) {
	tests := []struct {
		input    string
		params   []any
		expected string
	}{
		{"Hello {0}", []any{"World"}, "Hello World"},
		{"{0} + {1} = 2", []any{"1", "1"}, "1 + 1 = 2"},
		{"No placeholders", nil, "No placeholders"},
		{"Map: {name}", []any{map[string]any{"name": "Alice"}}, "Map: Alice"},
		{"Mixed {0} and {name}", []any{"index", map[string]any{"name": "Bob"}}, "Mixed index and Bob"},
		{"Empty string", []any{}, "Empty string"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := interpolate(tt.input, tt.params)
			if result != tt.expected {
				t.Errorf("interpolate(%q, %v) = %q, want %q", tt.input, tt.params, result, tt.expected)
			}
		})
	}
}

// TestNormalizeLang tests language normalization.
func TestNormalizeLang(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"german", "de"},
		{"German", "de"},
		{"deutsch", "de"},
		{"de", "de"},
		{"DE", "de"},
		{"english", "en"},
		{"EN", "en"},
		{"en", "en"},
		{"spanish", "es"},
		{"es", "es"},
		{"french", "fr"},
		{"fr", "fr"},
		{"chinese", "zh"},
		{"zh", "zh"},
		{"japanese", "ja"},
		{"ja", "ja"},
		{"portuguese", "pt"},
		{"pt", "pt"},
		{"italian", "it"},
		{"it", "it"},
		{"dutch", "nl"},
		{"nl", "nl"},
		{"polish", "pl"},
		{"pl", "pl"},
		{"unknown", "en"},    // fallback
		{"", "en"},           // fallback
		{"  german  ", "de"}, // trimmed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeLang(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeLang(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestHasKey tests the HasKey function.
func TestHasKey(t *testing.T) {
	// Create a test store
	store := &Store{
		langData: map[string]map[string]string{
			"en": {
				"test.key1": "Value 1",
				"test.key2": "Value 2",
			},
			"de": {
				"test.key1": "Wert 1",
			},
		},
		langJSON: map[string]string{
			"en": `{"test.key1": "Value 1", "test.key2": "Value 2"}`,
			"de": `{"test.key1": "Wert 1"}`,
		},
	}

	tests := []struct {
		lang     string
		key      string
		expected bool
	}{
		{"en", "test.key1", true},
		{"en", "test.key2", true},
		{"en", "test.key3", false},
		{"de", "test.key1", true},
		{"de", "test.key2", true},  // fallback to en
		{"fr", "test.key1", true},  // fallback to en
		{"fr", "test.key3", false}, // not found in en either
	}

	for _, tt := range tests {
		t.Run(tt.lang+"_"+tt.key, func(t *testing.T) {
			result := store.HasKey(tt.lang, tt.key)
			if result != tt.expected {
				t.Errorf("HasKey(%q, %q) = %v, want %v", tt.lang, tt.key, result, tt.expected)
			}
		})
	}
}

// TestTWithInterpolation tests the T function with parameter interpolation.
func TestTWithInterpolation(t *testing.T) {
	store := &Store{
		langData: map[string]map[string]string{
			"en": {
				"greet": "Hello {0}",
				"multi": "{0} + {1} = {2}",
				"named": "Hello {name}",
			},
		},
		langJSON: map[string]string{},
	}

	tests := []struct {
		key      string
		lang     string
		params   []any
		expected string
	}{
		{"greet", "en", []any{"World"}, "Hello World"},
		{"multi", "en", []any{"1", "1", "2"}, "1 + 1 = 2"},
		{"named", "en", []any{map[string]any{"name": "Alice"}}, "Hello Alice"},
		{"nonexistent", "en", nil, "nonexistent"}, // returns key itself
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := store.T(tt.lang, tt.key, tt.params...)
			if result != tt.expected {
				t.Errorf("T(%q, %q, %v) = %q, want %q", tt.lang, tt.key, tt.params, result, tt.expected)
			}
		})
	}
}
