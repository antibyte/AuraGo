package ui

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestModalOverlayActiveStateEnablesInteraction(t *testing.T) {
	t.Parallel()

	sharedCSS := normalizeAssetText(mustReadUIFile(t, "shared-components.css"))
	activeBlock := cssBlock(t, sharedCSS, ".modal-overlay.active,")
	for _, marker := range []string{
		"display: flex;",
		"opacity: 1;",
		"pointer-events: all;",
	} {
		if !strings.Contains(activeBlock, marker) {
			t.Fatalf("shared modal active state missing %q", marker)
		}
	}

	closedBlock := cssBlock(t, sharedCSS, ".modal-overlay {")
	if !strings.Contains(closedBlock, "pointer-events: none;") {
		t.Fatalf("closed modal overlay must disable pointer events")
	}
}

func TestModalNoiseTextureStaysBelowModalLayer(t *testing.T) {
	t.Parallel()

	varsCSS := normalizeAssetText(mustReadUIFile(t, "shared-variables.css"))
	tokensCSS := normalizeAssetText(mustReadUIFile(t, "css/tokens.css"))

	noiseZ := cssZIndexFromBlock(t, cssBlock(t, varsCSS, "body::before {"))
	modalZ := cssTokenInt(t, tokensCSS, "--z-modal:")
	if noiseZ >= modalZ {
		t.Fatalf("body::before z-index %d must stay below modal z-index %d", noiseZ, modalZ)
	}
}

func TestPageModalCSSSupportsActiveClass(t *testing.T) {
	t.Parallel()

	for _, pageCSS := range []string{
		"css/containers.css",
		"css/skills.css",
		"css/invasion.css",
	} {
		t.Run(pageCSS, func(t *testing.T) {
			t.Parallel()
			css := normalizeAssetText(mustReadUIFile(t, pageCSS))
			openBlock := cssBlock(t, css, ".modal-overlay.open,")
			if !strings.Contains(openBlock, ".modal-overlay.active") {
				t.Fatalf("%s missing .modal-overlay.active companion rule", pageCSS)
			}
			if !strings.Contains(openBlock, "pointer-events: all;") {
				t.Fatalf("%s active/open modal state must enable pointer events", pageCSS)
			}
		})
	}
}

func mustReadUIFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func cssTokenInt(t *testing.T, css, token string) int {
	t.Helper()
	re := regexp.MustCompile(regexp.QuoteMeta(token) + `\s*([^;]+);`)
	match := re.FindStringSubmatch(css)
	if len(match) < 2 {
		t.Fatalf("missing token %q", token)
	}
	value, err := strconv.Atoi(strings.TrimSpace(match[1]))
	if err != nil {
		t.Fatalf("parse token %q value %q: %v", token, match[1], err)
	}
	return value
}

func cssZIndexFromBlock(t *testing.T, block string) int {
	t.Helper()
	re := regexp.MustCompile(`z-index:\s*([^;]+);`)
	match := re.FindStringSubmatch(block)
	if len(match) < 2 {
		t.Fatalf("missing z-index in block:\n%s", block)
	}
	raw := strings.TrimSpace(match[1])
	if strings.HasPrefix(raw, "var(") {
		if strings.Contains(raw, "--z-modal") {
			return 1000
		}
		t.Fatalf("unsupported z-index variable %q", raw)
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("parse z-index %q: %v", raw, err)
	}
	return value
}