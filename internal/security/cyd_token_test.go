package security

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCYDTokenIsAuraPrefixPlusNineChars(t *testing.T) {
	t.Parallel()
	raw, err := generateCYDToken()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, "aura_") {
		t.Fatalf("prefix: %q", raw)
	}
	body := strings.TrimPrefix(raw, "aura_")
	if len(body) != CYDTokenBodyLen {
		t.Fatalf("body len %d want %d in %q", len(body), CYDTokenBodyLen, raw)
	}
	for _, r := range body {
		if !strings.ContainsRune(cydTokenAlphabet, r) {
			t.Fatalf("invalid char %q in %q", r, raw)
		}
	}
}

func TestFormatCYDTokenDisplayGroupsOfThree(t *testing.T) {
	t.Parallel()
	got := FormatCYDTokenDisplay("aura_K7M2PQ9XH")
	if got != "K7M 2PQ 9XH" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeAPITokenAcceptsGroupedCodeWithoutPrefix(t *testing.T) {
	t.Parallel()
	got := NormalizeAPIToken("k7m 2pq 9xh")
	if got != "aura_K7M2PQ9XH" {
		t.Fatalf("got %q", got)
	}
	if NormalizeAPIToken("aura_k7m-2pq-9xh") != "aura_K7M2PQ9XH" {
		t.Fatalf("dashed: %q", NormalizeAPIToken("aura_k7m-2pq-9xh"))
	}
}

func TestNormalizeAPITokenLeavesLegacyHexTokens(t *testing.T) {
	t.Parallel()
	legacy := "aura_0123456789abcdef0123456789abcdef"
	if got := NormalizeAPIToken(legacy); got != legacy {
		t.Fatalf("got %q", got)
	}
}

func TestCreateCYDScopeUsesShortToken(t *testing.T) {
	vault, err := NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	tm, err := NewTokenManager(vault, filepath.Join(t.TempDir(), "tokens.bin"))
	if err != nil {
		t.Fatal(err)
	}
	raw, meta, err := tm.Create("Cheap Yellow Display", []string{"cyd"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != len("aura_")+CYDTokenBodyLen {
		t.Fatalf("cyd token %q len %d", raw, len(raw))
	}
	_, ok := tm.Validate(raw, "cyd")
	if !ok {
		t.Fatal("short token should validate")
	}
	grouped := FormatCYDTokenDisplay(raw)
	if _, ok := tm.Validate(grouped, "cyd"); !ok {
		t.Fatalf("grouped %q should validate", grouped)
	}
	if !strings.HasPrefix(meta.Prefix, "aura_") {
		t.Fatalf("meta prefix %q", meta.Prefix)
	}

	long, _, err := tm.Create("webhook", []string{"webhook"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(long) != 37 {
		t.Fatalf("webhook token should stay long, got %q", long)
	}
}
