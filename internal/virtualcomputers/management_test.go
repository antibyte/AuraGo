package virtualcomputers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestManagementContract(t *testing.T) {
	if ManagementBasePath != "/boring-computers" {
		t.Fatalf("base path = %q", ManagementBasePath)
	}
	if ManagementListenAddr != "127.0.0.1:18081" {
		t.Fatalf("listen address = %q", ManagementListenAddr)
	}
	if ManagementURL != "http://127.0.0.1:18081" {
		t.Fatalf("management URL = %q", ManagementURL)
	}
	if got := ManagementHealthURL(ManagementURL); got != "http://127.0.0.1:18081/boring-computers/" {
		t.Fatalf("health URL = %q", got)
	}
	if len(PinnedUpstreamRevision) != 40 {
		t.Fatalf("pinned revision = %q, want a full commit SHA", PinnedUpstreamRevision)
	}
}

func TestSetupStatusIncludesAdditiveComponentStates(t *testing.T) {
	status := SetupStatus{
		Configured: true,
		Healthy:    false,
		ControlPlane: ComponentStatus{
			Configured: true,
			Healthy:    true,
		},
		Management: ComponentStatus{
			Configured: true,
			Healthy:    false,
			Message:    "management service is unavailable",
		},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	for _, want := range []string{`"configured":true`, `"healthy":false`, `"control_plane"`, `"management"`} {
		if !strings.Contains(encoded, want) {
			t.Fatalf("encoded setup status %s does not contain %s", encoded, want)
		}
	}
}
