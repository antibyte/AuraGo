package tools

import (
	"aurago/internal/dockerutil"
	"net/http"
	"strings"
	"testing"
)

func TestOllamaDockerProtectionByNameLabelAndVolume(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	for _, name := range []string{"aurago_ollama_managed", "aurago_ollama_embeddings"} {
		owned, err := DockerContainerOwnership(DockerConfig{}, name, dockerutil.OllamaOwner)
		if err != nil || !owned[dockerutil.OllamaOwner] {
			t.Fatal(name, owned, err)
		}
		if got := DockerCreateContainerWithOptions(DockerConfig{}, name, "alpine", nil, nil, nil, nil, "", nil, ContainerCreateOptions{}); !strings.Contains(got, "reserved") {
			t.Fatal(got)
		}
	}
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Name":"/renamed","Config":{"Labels":{"aurago.managed":"ollama"}}}`))
	})
	if !DockerContainerManagedBy(DockerConfig{Host: host}, "abc123", dockerutil.OllamaOwner) {
		t.Fatal("renamed labeled container exposed")
	}
	for _, name := range []string{ollamaManagedVolumeName, ollamaEmbeddingsVolumeName} {
		if !DockerBindUsesOllamaVolume(name+":/models:ro") || !dockerManagedResourceExcluded(nil, []string{name}, true, []string{dockerutil.OllamaOwner}) {
			t.Fatal(name)
		}
		if !strings.Contains(DockerRemoveVolume(DockerConfig{}, name, false), "reserved") {
			t.Fatal("managed volume could be removed")
		}
	}
	if DockerBindUsesOllamaVolume("unrelated-models:/models") {
		t.Fatal("unrelated volume blocked")
	}
}

func TestOllamaComposeProtectionIncludesNamespaceAndVolume(t *testing.T) {
	for _, text := range []string{
		`{"services":{"app":{"container_name":"aurago_ollama_managed","image":"alpine"}}}`,
		`{"services":{"app":{"network_mode":"container:aurago_ollama_embeddings","image":"alpine"}}}`,
		`{"services":{"app":{"image":"alpine","volumes":[{"type":"volume","source":"aurago_ollama_managed_models","target":"/models"}]}}}`,
		`{"services":{"app":{"image":"alpine","labels":{"aurago.managed":"ollama"}}}}`,
	} {
		model, err := ParseDockerComposeModel(text)
		if err != nil {
			t.Fatal(err)
		}
		if owner := DockerComposeModelOwner(model); owner != dockerutil.OllamaOwner {
			t.Fatal(owner, text)
		}
	}
}
