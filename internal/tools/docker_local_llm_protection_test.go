package tools

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/dockerutil"
)

func TestLocalLLMNamedVolumesAreRejectedByCreateMountValidation(t *testing.T) {
	for _, name := range []string{
		dockerutil.LocalLLMModelVolumeName,
		dockerutil.LocalLLMKeyVolumeName,
	} {
		err := validateDockerBindMount(DockerConfig{}, name+":/models:ro")
		if err == nil || !strings.Contains(err.Error(), "reserved AuraGo local LLM volume") {
			t.Fatalf("validateDockerBindMount(%q) error = %v", name, err)
		}
	}
	if err := validateDockerBindMount(DockerConfig{}, "ordinary-data:/data"); err != nil {
		t.Fatalf("ordinary named volume was rejected: %v", err)
	}
}

func TestLocalLLMManagedResourceFilteringRecognizesLabelsNamesAndIDs(t *testing.T) {
	for _, test := range []struct {
		labels map[string]string
		names  []string
		volume bool
	}{
		{labels: map[string]string{"aurago.managed": "local-llm"}},
		{labels: map[string]string{"com.aurago.managed": "true", "com.aurago.owner": "local-llm"}},
		{names: []string{"/aurago-local-llm"}},
		{names: []string{"aurago_models"}, volume: true},
		{names: []string{"aurago_local_llm_runtime"}, volume: true},
	} {
		if !dockerManagedResourceExcluded(test.labels, test.names, test.volume, []string{dockerutil.LocalLLMOwner}) {
			t.Fatalf("managed resource was not filtered: %#v", test)
		}
	}
}

func TestLocalLLMManagedContainerDetectionFailsClosedForReservedNameAndInspectParseError(t *testing.T) {
	t.Run("reserved name reached by id", func(t *testing.T) {
		host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/containers/abc123/json") {
				_, _ = w.Write([]byte(`{"Name":"/aurago-local-llm","Config":{"Labels":{}}}`))
				return
			}
			t.Fatalf("unexpected Docker request %s", r.URL.Path)
		})
		if !DockerContainerManagedBy(DockerConfig{Host: host}, "abc123", dockerutil.LocalLLMOwner) {
			t.Fatal("reserved container name reached through its ID was not protected")
		}
	})

	t.Run("malformed inspect falls back to filtered list", func(t *testing.T) {
		host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/containers/deadbeef/json"):
				_, _ = w.Write([]byte(`{`))
			case strings.HasSuffix(r.URL.Path, "/containers/json"):
				_, _ = w.Write([]byte(`[{"Id":"deadbeefcafebabe","Names":["/renamed"],"Labels":{"com.aurago.managed":"true","com.aurago.owner":"local-llm"}}]`))
			default:
				t.Fatalf("unexpected Docker request %s", r.URL.Path)
			}
		})
		if !DockerContainerManagedBy(DockerConfig{Host: host}, "deadbeef", dockerutil.LocalLLMOwner) {
			t.Fatal("inspect parse error exposed a legacy protected container ID")
		}
	})
}

func TestHomepageManagedContainerDetectionRecognizesIDAndName(t *testing.T) {
	if !DockerContainerManagedBy(DockerConfig{}, dockerutil.HomepageContainerName, dockerutil.HomepageOwner) {
		t.Fatal("reserved homepage container name was not protected")
	}
	if !DockerContainerManagedBy(DockerConfig{}, dockerutil.AppContainerName, dockerutil.AppOwner) {
		t.Fatal("reserved AuraGo app container name was not protected")
	}

	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/containers/abc123/json") {
			_, _ = w.Write([]byte(`{"Name":"/aurago-homepage-web","Config":{"Labels":{}}}`))
			return
		}
		t.Fatalf("unexpected Docker request %s", r.URL.Path)
	})
	if !DockerContainerManagedBy(DockerConfig{Host: host}, "abc123", dockerutil.HomepageOwner) {
		t.Fatal("reserved homepage container reached through its ID was not protected")
	}
}

// The CYD Creator builder's work and cache volumes are refused as bind sources
// and for volume create/remove before any Docker request; the test and CI
// volumes stay usable.
func TestCYDCreatorVolumesAreRejectedByMountAndVolumeHelpers(t *testing.T) {
	for _, bind := range []string{"aurago-cyd-cache:/cache", "aurago-cyd-work:/work:ro", "AURAGO-CYD-CACHE:/x"} {
		err := validateDockerBindMount(DockerConfig{}, bind)
		if err == nil || !strings.Contains(err.Error(), "reserved AuraGo CYD Creator volume") {
			t.Fatalf("validateDockerBindMount(%q) error = %v", bind, err)
		}
	}
	for _, bind := range []string{"aurago-cyd-work-it:/work", "aurago-cyd-cache-it:/cache", "cyd-ci-cache:/cache"} {
		if err := validateDockerBindMount(DockerConfig{}, bind); err != nil {
			t.Fatalf("test volume %q was rejected: %v", bind, err)
		}
	}

	var requests atomic.Int64
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	cfg := DockerConfig{Host: host}
	for _, name := range []string{dockerutil.CYDCreatorWorkVolumeName, dockerutil.CYDCreatorCacheVolumeName} {
		if got := DockerCreateVolume(cfg, name, ""); !strings.Contains(got, "reserved AuraGo CYD Creator volume name") {
			t.Fatalf("create volume %s = %s, want a reserved-name refusal", name, got)
		}
		if got := DockerRemoveVolume(cfg, name, true); !strings.Contains(got, "reserved AuraGo CYD Creator volume name") {
			t.Fatalf("remove volume %s = %s, want a reserved-name refusal", name, got)
		}
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("Docker received %d requests for reserved volume names", got)
	}
	if got := DockerRemoveVolume(cfg, "aurago-cyd-cache-it", false); strings.Contains(got, "reserved") {
		t.Fatalf("test volume removal refused: %s", got)
	}
}

// Ownership of the builder: its reserved name decides without a Docker
// request, its label decides for a container reached by ID or renamed, and the
// agent list filters hide the builder and both volumes.
func TestCYDCreatorBuilderOwnershipByNameLabelAndID(t *testing.T) {
	owned, err := DockerContainerOwnership(DockerConfig{Host: "tcp://127.0.0.1:1"}, "aurago-cyd-builder", dockerutil.CYDCreatorOwner)
	if err != nil || !owned[dockerutil.CYDCreatorOwner] {
		t.Fatalf("ownership of aurago-cyd-builder = %v, %v; want cyd-creator without a request", owned, err)
	}
	labels := dockerutil.ManagedLabels(dockerutil.CYDCreatorOwner, "cyd-creator", "builder", "0123456789abcdef")
	for _, tc := range []struct {
		name   string
		names  []string
		labels map[string]string
		want   bool
	}{
		{"labeled builder", []string{"/aurago-cyd-builder"}, labels, true},
		{"unlabeled container under the name", []string{"/aurago-cyd-builder"}, nil, true},
		{"labels alone", []string{"/renamed"}, labels, true},
		{"integration test container", []string{"/aurago-cyd-builder-it"}, nil, false},
		{"other owner", []string{"/web"}, map[string]string{"aurago.managed": "homepage"}, false},
	} {
		got := DockerContainerOwnersFromMetadata(tc.names, tc.labels, dockerutil.CYDCreatorOwner)
		if got[dockerutil.CYDCreatorOwner] != tc.want {
			t.Fatalf("%s: owners = %v, want cyd-creator %v", tc.name, got, tc.want)
		}
	}

	t.Run("labeled container reached by id", func(t *testing.T) {
		host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/containers/abc123/json") {
				_, _ = w.Write([]byte(`{"Name":"/renamed","Config":{"Labels":{"aurago.managed":"cyd-creator","aurago.role":"builder"}}}`))
				return
			}
			t.Fatalf("unexpected Docker request %s", r.URL.Path)
		})
		if !DockerContainerManagedBy(DockerConfig{Host: host}, "abc123", dockerutil.CYDCreatorOwner) {
			t.Fatal("labeled builder reached through its ID was not protected")
		}
	})
	t.Run("malformed inspect falls back to the list", func(t *testing.T) {
		host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/containers/deadbeef/json"):
				_, _ = w.Write([]byte(`{`))
			case strings.HasSuffix(r.URL.Path, "/containers/json"):
				_, _ = w.Write([]byte(`[{"Id":"deadbeefcafebabe","Names":["/aurago-cyd-builder"],"Labels":{}}]`))
			default:
				t.Fatalf("unexpected Docker request %s", r.URL.Path)
			}
		})
		if !DockerContainerManagedBy(DockerConfig{Host: host}, "deadbeef", dockerutil.CYDCreatorOwner) {
			t.Fatal("inspect parse error exposed the builder reached by ID prefix")
		}
	})

	for _, tc := range []struct {
		labels map[string]string
		names  []string
		volume bool
		want   bool
	}{
		{labels: labels, want: true},
		{names: []string{"/aurago-cyd-builder"}, want: true},
		{names: []string{"aurago-cyd-work"}, volume: true, want: true},
		{names: []string{"aurago-cyd-cache"}, volume: true, want: true},
		{labels: map[string]string{"aurago.managed": "cyd-creator", "aurago.role": "cache"}, names: []string{"renamed"}, volume: true, want: true},
		{names: []string{"aurago-cyd-cache-it"}, volume: true},
		{names: []string{"cyd-ci-work"}, volume: true},
		{names: []string{"aurago-cyd-cache"}}, // a container under a volume name is not the builder
		{names: []string{"aurago-cyd-builder"}, volume: true},
	} {
		if got := dockerManagedResourceExcluded(tc.labels, tc.names, tc.volume, []string{dockerutil.CYDCreatorOwner}); got != tc.want {
			t.Fatalf("dockerManagedResourceExcluded(%#v) = %v, want %v", tc, got, tc.want)
		}
	}
}

// The agent's list_volumes call passes the CYD Creator owner and must not show
// the builder's volumes; the test volumes stay visible.
func TestDockerListVolumesHidesCYDCreatorVolumes(t *testing.T) {
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/volumes") {
			t.Fatalf("unexpected Docker request %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"Volumes":[
			{"Name":"aurago-cyd-work","Driver":"local","Labels":{"aurago.managed":"cyd-creator"}},
			{"Name":"aurago-cyd-cache","Driver":"local","Labels":null},
			{"Name":"aurago-cyd-cache-it","Driver":"local","Labels":null},
			{"Name":"data","Driver":"local","Labels":null}
		]}`))
	})
	got := DockerListVolumes(DockerConfig{Host: host}, dockerutil.LocalLLMOwner, dockerutil.CYDCreatorOwner)
	for _, hidden := range []string{`"aurago-cyd-work"`, `"aurago-cyd-cache"`} {
		if strings.Contains(got, hidden) {
			t.Fatalf("list = %s, want %s hidden", got, hidden)
		}
	}
	if !strings.Contains(got, `"aurago-cyd-cache-it"`) || !strings.Contains(got, `"data"`) || !strings.Contains(got, `"count":2`) {
		t.Fatalf("list = %s, want the test volume and data", got)
	}
}
