package tools

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/acestep"
)

func TestACESTepManagedResourcesAreProtected(t *testing.T) {
	for _, name := range []string{acestep.ContainerName, acestep.ContainerName + "-probe", acestep.ModelVolume, acestep.CacheVolume} {
		if !dockerManagedResourceExcluded(nil, []string{name}, strings.Contains(name, "_"), []string{acestep.Owner}) {
			t.Errorf("resource visible: %s", name)
		}
	}
	for _, volume := range []string{acestep.ModelVolume, acestep.CacheVolume} {
		result := DockerCreateVolume(DockerConfig{}, volume, "")
		if !strings.Contains(result, "protected") && !strings.Contains(result, "reserved") && !strings.Contains(result, "managed") {
			t.Errorf("volume guard not reached: %s", result)
		}
	}
	if !DockerContainerManagedBy(DockerConfig{}, acestep.ContainerName, acestep.Owner) {
		t.Fatal("reserved container not protected")
	}
}

func TestACESTepIDLookupDoesNotProtectUnrelatedContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/containers/json") {
			io.WriteString(w, `[{"Id":"abc123","Names":["/other"]},{"Id":"def456","Names":["/aurago-acestep"],"Labels":{"aurago.managed":"acestep"}}]`)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	cfg := DockerConfig{Host: strings.Replace(server.URL, "http://", "tcp://", 1)}
	if DockerContainerManagedBy(cfg, "abc123", acestep.Owner) {
		t.Fatal("unrelated container protected")
	}
	if !DockerContainerManagedBy(cfg, "def456", acestep.Owner) {
		t.Fatal("managed ID not protected")
	}
}
