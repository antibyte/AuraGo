package hass

// Opt-in live test (decision D14). Environment variables:
//
//	HOMEDASH_HA_LIVE_URL     base URL of a real Home Assistant (required)
//	HOMEDASH_HA_LIVE_TOKEN   long-lived access token (required, never printed)
//	HOMEDASH_HA_LIVE_TOGGLE  optional native id of one switch or light to toggle twice
//
// Without the first two the test skips, so CI never needs them.

import (
	"context"
	"os"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// TestHALiveReadOnly runs against a real Home Assistant only when HOMEDASH_HA_LIVE_URL and
// HOMEDASH_HA_LIVE_TOKEN are set. It reads the catalog and never calls a service unless
// HOMEDASH_HA_LIVE_TOGGLE names one switch or light, which it toggles twice (back to its state).
func TestHALiveReadOnly(t *testing.T) {
	url, token := os.Getenv("HOMEDASH_HA_LIVE_URL"), os.Getenv("HOMEDASH_HA_LIVE_TOKEN")
	if url == "" || token == "" {
		t.Skip("set HOMEDASH_HA_LIVE_URL and HOMEDASH_HA_LIVE_TOKEN to run against a real Home Assistant")
	}
	a := New(tools.HAConfig{URL: url, AccessToken: token}, 5, FactoryOptions{})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	list, err := a.Catalog(ctx)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	withArea := 0
	for _, e := range list {
		if e.AreaID != "" {
			withArea++
		}
	}
	t.Logf("%d entities, %d with an area", len(list), withArea)
	if native := os.Getenv("HOMEDASH_HA_LIVE_TOGGLE"); native != "" {
		for i := 0; i < 2; i++ {
			res, err := a.Command(ctx, homedash.Command{Entity: homedash.MakeEntityID(SourceID, native), Native: native, Name: homedash.CapToggle, Args: map[string]any{}})
			if err != nil {
				t.Fatalf("toggle %d: %v", i+1, err)
			}
			t.Logf("toggle %d expected %+v", i+1, res.Expected)
			time.Sleep(2 * time.Second)
		}
	}
}
