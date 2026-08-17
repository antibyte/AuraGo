package tools

import (
	"net/http"
	"strings"
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
