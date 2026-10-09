package audit

import "testing"

func TestLocalWikipediaNetworkClientIsClassified(t *testing.T) {
	t.Parallel()

	found := false
	for _, entry := range NetworkClientInventory() {
		if entry.Path == "internal/localwiki/" {
			found = !entry.RequiresSSRF && !entry.AllowsLocalNet && !entry.Credentialed && entry.Classification != ""
		}
	}
	if !found {
		t.Fatal("internal/localwiki/ must be classified as an uncredentialed public HTTPS client")
	}
}

func TestLocalWikipediaRoutesHaveAdminContract(t *testing.T) {
	t.Parallel()

	if !routeContractExists(RouteContractManifest(), "/api/local-wikipedia/", "session-admin") {
		t.Fatal("/api/local-wikipedia/ must have an explicit session-admin route contract")
	}
}

// The Desktop API of the Wikipedia app only reads: search, suggestions,
// status and edition content for sessions or desktop:read tokens.
func TestLocalWikipediaDesktopRoutesHaveReadOnlyContract(t *testing.T) {
	t.Parallel()

	for _, c := range RouteContractManifest() {
		if c.Pattern != "/api/desktop/local-wikipedia/" {
			continue
		}
		if c.Auth != "session-or-desktop-token" || routeCanMutate(c.Methods) {
			t.Fatalf("/api/desktop/local-wikipedia/ contract = %+v, want a read-only session-or-desktop-token route", c)
		}
		return
	}
	t.Fatal("/api/desktop/local-wikipedia/ must have an explicit route contract")
}
