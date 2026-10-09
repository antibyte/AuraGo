package localwiki

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func TestLanguageTableMatchesConfiguredCodes(t *testing.T) {
	want := config.LocalWikipediaLanguageCodes()
	if len(languageTable) != len(want) {
		t.Fatalf("languageTable has %d languages, config offers %d", len(languageTable), len(want))
	}
	for i, spec := range languageTable {
		if spec.Code != want[i] {
			t.Fatalf("languageTable[%d] = %q, want %q", i, spec.Code, want[i])
		}
		if spec.Kiwix == "" || len(spec.ISO3) != 3 || spec.Name == "" {
			t.Fatalf("incomplete language spec %+v", spec)
		}
	}
	norwegian, ok := lookupLanguage("no")
	if !ok || norwegian.Kiwix != "nb" || norwegian.ISO3 != "nob" {
		t.Fatalf("Norwegian must use the Bokmål edition wikipedia_nb_all: %+v", norwegian)
	}
}

func TestResolveLanguage(t *testing.T) {
	for _, tc := range []struct{ configured, system, want string }{
		{"de", "English", "de"},
		{" NO ", "Deutsch", "no"},
		{"", "Deutsch", "de"},
		{"", "English", "en"},
		{"", "pl", "pl"},
		{"xx", "Polski", "pl"},
		{"", "Klingon", "en"},
		{"", "", "en"},
	} {
		if got := ResolveLanguage(tc.configured, tc.system); got != tc.want {
			t.Fatalf("ResolveLanguage(%q, %q) = %q, want %q", tc.configured, tc.system, got, tc.want)
		}
	}
	if !SupportedLanguage("zh") || SupportedLanguage("nb") || SupportedLanguage("") {
		t.Fatal("SupportedLanguage must accept exactly the AuraGo codes")
	}
}

func TestLanguagesListsAllSixteenWithNames(t *testing.T) {
	infos := Languages()
	if len(infos) != 16 {
		t.Fatalf("Languages() returned %d entries", len(infos))
	}
	for _, info := range infos {
		spec, _ := lookupLanguage(info.Code)
		if info.Name != spec.Name || info.Fulltext != fulltextSupported(spec) {
			t.Fatalf("language info %+v does not match spec %+v", info, spec)
		}
	}
	infos[0].Name = "changed"
	if Languages()[0].Name == "changed" {
		t.Fatal("Languages must return a copy")
	}
}

func TestSettingsFromConfig(t *testing.T) {
	dataDir := t.TempDir()
	custom := filepath.Join(t.TempDir(), "wiki")
	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Agent.SystemLanguage = "Deutsch"
	cfg.LocalWikipedia = config.LocalWikipediaConfig{Enabled: true, AgentAccess: true, Variant: "maxi", UpdateCheck: true}

	got := SettingsFromConfig(cfg)
	want := Settings{
		Enabled: true, AgentAccess: true, Language: "de", SystemLanguage: "de", Variant: VariantMaxi,
		DataDir: filepath.Join(dataDir, "wikipedia"), UpdateCheck: true,
	}
	if got != want {
		t.Fatalf("default settings = %+v, want %+v", got, want)
	}

	cfg.LocalWikipedia.Language = "fr"
	cfg.LocalWikipedia.DataDir = custom
	got = SettingsFromConfig(cfg)
	if got.Language != "fr" || got.SystemLanguage != "de" || got.DataDir != custom || got.DataDirLocked {
		t.Fatalf("explicit settings = %+v", got)
	}

	cfg.Runtime.IsDocker = true
	got = SettingsFromConfig(cfg)
	if got.DataDir != filepath.Join(dataDir, "wikipedia") || !got.DataDirLocked {
		t.Fatalf("Docker must force <data_dir>/wikipedia: %+v", got)
	}
	if SettingsFromConfig(nil).Variant != VariantNoPic {
		t.Fatal("nil config must default to nopic")
	}
}

func TestErrorCodeAndRecommendation(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{nil, ""},
		{&InsufficientSpaceError{Required: 2, Available: 1}, CodeInsufficientDiskSpace},
		{fmt.Errorf("wrapped: %w", ErrBusy), CodeBusy},
		{ErrDisabled, CodeDisabled},
		{ErrUnknownFreeSpace, CodeFreeSpaceUnknown},
		{fmt.Errorf("%w: relative", ErrDataDirInvalid), CodeDataDirInvalid},
		{fmt.Errorf("%w: timeout", ErrCatalogUnreachable), CodeCatalogUnreachable},
		{ErrAlreadyInstalled, CodeAlreadyInstalled},
		{ErrNoOperation, CodeNoOperation},
		{ErrUnknownLanguage, CodeUnknownLanguage},
		{errChecksumMismatch, CodeChecksumMismatch},
		{errZIMUnreadable, CodeZIMUnreadable},
		{fmt.Errorf("%w: all mirrors", errDownloadFailed), CodeDownloadFailed},
		{errors.New("boom"), CodeInternal},
	} {
		if got := ErrorCode(tc.err); got != tc.code {
			t.Fatalf("ErrorCode(%v) = %q, want %q", tc.err, got, tc.code)
		}
		if tc.code != "" && Recommendation(tc.code) == "" {
			t.Fatalf("Recommendation(%q) is empty", tc.code)
		}
	}
	if Recommendation("") != "" || Recommendation(CodeFulltextUnsupported) == "" {
		t.Fatal("Recommendation must be empty only for an empty code")
	}
}
