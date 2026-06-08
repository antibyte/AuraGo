package desktopstore

import (
	"strings"
	"testing"
)

func TestStoreLogoURLUsesFirstPartyPath(t *testing.T) {
	got := StoreLogoURL("homarr")
	want := "/api/desktop/store/logos/homarr.png"
	if got != want {
		t.Fatalf("StoreLogoURL() = %q, want %q", got, want)
	}
	if strings.Contains(got, "cdn.jsdelivr.net") {
		t.Fatal("store logo URL must not expose external CDN hosts")
	}
}