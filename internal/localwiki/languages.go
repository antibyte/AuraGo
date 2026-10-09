package localwiki

import (
	"strings"
	"sync"

	"aurago/internal/i18n"
	"aurago/internal/zim/xapian"
)

// languageSpec maps an AuraGo UI language to its Kiwix Wikipedia edition.
type languageSpec struct {
	Code  string // AuraGo UI and Wikipedia language code
	Kiwix string // code in the Kiwix name wikipedia_<kiwix>_all
	ISO3  string // ISO 639-3 code used by the catalog and the ZIM's M/Language
	Name  string // endonym shown in the language dropdown
}

// languageTable lists the 16 AuraGo UI languages. Every Kiwix name was checked
// against the OPDS catalog on 2026-10-09: all offer maxi and nopic. Norwegian
// is published as Bokmål (wikipedia_nb_all); wikipedia_no_all does not exist.
var languageTable = []languageSpec{
	{Code: "cs", Kiwix: "cs", ISO3: "ces", Name: "Čeština"},
	{Code: "da", Kiwix: "da", ISO3: "dan", Name: "Dansk"},
	{Code: "de", Kiwix: "de", ISO3: "deu", Name: "Deutsch"},
	{Code: "el", Kiwix: "el", ISO3: "ell", Name: "Ελληνικά"},
	{Code: "en", Kiwix: "en", ISO3: "eng", Name: "English"},
	{Code: "es", Kiwix: "es", ISO3: "spa", Name: "Español"},
	{Code: "fr", Kiwix: "fr", ISO3: "fra", Name: "Français"},
	{Code: "hi", Kiwix: "hi", ISO3: "hin", Name: "हिन्दी"},
	{Code: "it", Kiwix: "it", ISO3: "ita", Name: "Italiano"},
	{Code: "ja", Kiwix: "ja", ISO3: "jpn", Name: "日本語"},
	{Code: "nl", Kiwix: "nl", ISO3: "nld", Name: "Nederlands"},
	{Code: "no", Kiwix: "nb", ISO3: "nob", Name: "Norsk"},
	{Code: "pl", Kiwix: "pl", ISO3: "pol", Name: "Polski"},
	{Code: "pt", Kiwix: "pt", ISO3: "por", Name: "Português"},
	{Code: "sv", Kiwix: "sv", ISO3: "swe", Name: "Svenska"},
	{Code: "zh", Kiwix: "zh", ISO3: "zho", Name: "中文"},
}

var (
	languageInfoOnce sync.Once
	languageInfos    []LanguageInfo
)

func lookupLanguage(code string) (languageSpec, bool) {
	code = strings.ToLower(strings.TrimSpace(code))
	for _, spec := range languageTable {
		if spec.Code == code {
			return spec, true
		}
	}
	return languageSpec{}, false
}

// SupportedLanguage reports whether code is one of the offered languages.
func SupportedLanguage(code string) bool {
	_, ok := lookupLanguage(code)
	return ok
}

// ResolveLanguage returns the configured language when it is offered and
// otherwise the AuraGo system language (agent.system_language, e.g. "Deutsch"
// or "de"); English is the last fallback.
func ResolveLanguage(configured, systemLanguage string) string {
	if spec, ok := lookupLanguage(configured); ok {
		return spec.Code
	}
	if spec, ok := lookupLanguage(i18n.NormalizeLang(systemLanguage)); ok {
		return spec.Code
	}
	return "en"
}

// fulltextSupported reports whether AuraGo can search a language's full-text
// index (slice 2 analyzer); otherwise only title search is offered.
func fulltextSupported(spec languageSpec) bool {
	return xapian.NewAnalyzer(spec.ISO3).FulltextSupported()
}

// Languages lists the offered languages with their full-text support.
func Languages() []LanguageInfo {
	languageInfoOnce.Do(func() {
		for _, spec := range languageTable {
			languageInfos = append(languageInfos, LanguageInfo{Code: spec.Code, Name: spec.Name, Fulltext: fulltextSupported(spec)})
		}
	})
	return append([]LanguageInfo(nil), languageInfos...)
}
