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
