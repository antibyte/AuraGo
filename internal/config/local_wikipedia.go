package config

import (
	"fmt"
	"strings"
)

// localWikipediaLanguageCodes are the AuraGo UI languages Local Wikipedia
// offers. internal/localwiki maps each code to its Kiwix edition and keeps a
// test that both lists stay identical.
var localWikipediaLanguageCodes = []string{
	"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh",
}

// LocalWikipediaLanguageCodes returns a copy of the offered language codes.
func LocalWikipediaLanguageCodes() []string {
	return append([]string(nil), localWikipediaLanguageCodes...)
}

// NormalizeLocalWikipediaConfig canonicalizes a loaded section. Unknown values
// fall back to the documented defaults (system language, without media)
// instead of failing startup; the config save path rejects them first.
func NormalizeLocalWikipediaConfig(c *LocalWikipediaConfig) {
	if c == nil {
		return
	}
	c.Language = strings.ToLower(strings.TrimSpace(c.Language))
	if c.Language != "" && !oneOf(c.Language, localWikipediaLanguageCodes...) {
		c.Language = ""
	}
	c.Variant = strings.ToLower(strings.TrimSpace(c.Variant))
	if c.Variant != "maxi" {
		c.Variant = "nopic"
	}
	c.DataDir = strings.TrimSpace(c.DataDir)
}

// ValidateLocalWikipediaConfig rejects languages and variants the integration
// does not offer. The storage directory is validated by the server, which
// knows whether AuraGo runs in Docker.
func ValidateLocalWikipediaConfig(c LocalWikipediaConfig) error {
	language := strings.ToLower(strings.TrimSpace(c.Language))
	if language != "" && !oneOf(language, localWikipediaLanguageCodes...) {
		return fmt.Errorf("local_wikipedia.language must be empty (system language) or one of %s",
			strings.Join(localWikipediaLanguageCodes, ", "))
	}
	switch strings.ToLower(strings.TrimSpace(c.Variant)) {
	case "", "nopic", "maxi":
		return nil
	default:
		return fmt.Errorf("local_wikipedia.variant must be nopic or maxi")
	}
}
