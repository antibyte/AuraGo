package rtlsdr

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestRuntimeImageIsDigestPinned(t *testing.T) {
	name, digest, pinned := strings.Cut(RuntimeImage, "@sha256:")
	decoded, err := hex.DecodeString(digest)
	if !pinned || !strings.HasPrefix(name, "ghcr.io/antibyte/aurago-rtl-sdr:") || err != nil || len(decoded) != 32 {
		t.Fatal("RTL-SDR runtime must use a versioned multiarch SHA-256 reference")
	}
}
