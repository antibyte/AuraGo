package audit

import (
	"slices"
	"testing"
)

// The HomeDash desktop API has one dedicated contract that allows DELETE (design section 11 item 1)
// and names the event stream.
func TestHomeDashDesktopRouteContract(t *testing.T) {
	t.Parallel()
	for _, c := range RouteContractManifest() {
		if c.Pattern != "/api/desktop/homedash/" {
			continue
		}
		if c.Auth != "session-or-desktop-admin-token" || !slices.Contains(c.Methods, "DELETE") || !slices.Contains(c.ContentTypes, "text/event-stream") {
			t.Fatalf("/api/desktop/homedash/ contract = %+v", c)
		}
		return
	}
	t.Fatal("/api/desktop/homedash/ must have an explicit route contract")
}
