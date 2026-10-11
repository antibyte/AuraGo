package dockerutil

import (
	"reflect"
	"strings"
	"testing"
)

func TestDefaultHostForGOOS(t *testing.T) {
	t.Parallel()

	if got := DefaultHostForGOOS("windows"); got != "npipe:////./pipe/docker_engine" {
		t.Fatalf("windows Docker host = %q, want npipe:////./pipe/docker_engine", got)
	}
	if got := DefaultHostForGOOS("linux"); got != "unix:///var/run/docker.sock" {
		t.Fatalf("linux Docker host = %q, want unix:///var/run/docker.sock", got)
	}
}

func TestNormalizeNamedPipeHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host string
		want string
	}{
		{
			name: "default docker desktop pipe",
			host: "npipe:////./pipe/docker_engine",
			want: `\\.\pipe\docker_engine`,
		},
		{
			name: "shorthand pipe",
			host: "npipe://./pipe/docker_engine",
			want: `\\.\pipe\docker_engine`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeNamedPipeHost(tt.host)
			if err != nil {
				t.Fatalf("NormalizeNamedPipeHost returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeNamedPipeHost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeNamedPipeHostRejectsInvalidPath(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeNamedPipeHost("npipe://docker_engine"); err == nil {
		t.Fatal("expected invalid named pipe path error")
	}
}

func TestFormatBindMountNormalizesHostSlashesAndOptions(t *testing.T) {
	t.Parallel()

	got := FormatBindMount(`C:\Users\andi\project`, "/workspace", "ro")
	want := "C:/Users/andi/project:/workspace:ro"
	if got != want {
		t.Fatalf("FormatBindMount() = %q, want %q", got, want)
	}
}

func TestManagedByRecognizesCanonicalAndLegacyLabels(t *testing.T) {
	t.Parallel()

	for _, labels := range []map[string]string{
		{"aurago.managed": "local-llm"},
		{"com.aurago.managed": "true", "com.aurago.owner": "local-llm"},
	} {
		if !ManagedBy(labels, LocalLLMOwner) {
			t.Fatalf("labels were not recognized: %#v", labels)
		}
	}
	if ManagedBy(map[string]string{"aurago.managed": "go2rtc"}, LocalLLMOwner) {
		t.Fatal("foreign managed resource was recognized as local LLM")
	}
}

func TestLocalLLMReservedNames(t *testing.T) {
	t.Parallel()

	if !IsLocalLLMContainerName("/AURAGO-LOCAL-LLM") ||
		!IsLocalLLMContainerName(LocalLLMKeySeedName) ||
		!IsLocalLLMContainerName("legacy-aurago-local-llm-1") ||
		!IsLocalLLMVolumeName("AURAGO_MODELS") ||
		!IsLocalLLMVolumeName("legacy_aurago_models") ||
		!IsLocalLLMVolumeName(LocalLLMKeyVolumeName) {
		t.Fatal("reserved local LLM resource name was not recognized")
	}
}

func TestAuraGoAppContainerName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"aurago", "/AURAGO", "homelab_aurago", "homelab-aurago-1"} {
		if !IsAuraGoAppContainerName(name) {
			t.Fatalf("AuraGo app container name %q was not recognized", name)
		}
	}
	for _, name := range []string{
		"aurago-local-llm",
		"aurago_gotenberg",
		"aurago_docker_proxy",
		"aurago-homepage",
		"aurago-homepage-web",
		"custom-caddy",
	} {
		if IsAuraGoAppContainerName(name) {
			t.Fatalf("unrelated container %q was recognized as AuraGo app", name)
		}
	}
}

func TestHomepageReservedResources(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"aurago-homepage", "/AURAGO-HOMEPAGE-WEB"} {
		if !IsHomepageContainerName(name) {
			t.Fatalf("homepage container name %q was not recognized", name)
		}
	}
	for _, image := range []string{
		"aurago-homepage",
		"aurago-homepage:latest",
		"registry.example.test/team/aurago-homepage:v1",
		"aurago-homepage@sha256:0123456789abcdef",
	} {
		if !IsHomepageImageReference(image) {
			t.Fatalf("homepage image %q was not recognized", image)
		}
	}
	for _, value := range []string{"my-aurago-homepage", "caddy:2-alpine", "example/aurago-homepage-assets:latest"} {
		if IsHomepageContainerName(value) || IsHomepageImageReference(value) {
			t.Fatalf("unrelated resource %q was recognized as managed homepage", value)
		}
	}
}

func TestParseNumericGroupIDs(t *testing.T) {
	t.Parallel()

	got := ParseNumericGroupIDs(" 109,44,109,0,-1,nope,2147483648 ")
	want := []string{"109", "44"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseNumericGroupIDs() = %#v, want %#v", got, want)
	}
}

func TestSecurityProxyReservedName(t *testing.T) {
	t.Parallel()

	if SecurityProxyOwner != "security-proxy" || SecurityProxyContainerName != "aurago-security-proxy" {
		t.Fatalf("owner = %q, name = %q", SecurityProxyOwner, SecurityProxyContainerName)
	}
	for _, name := range []string{"aurago-security-proxy", "/aurago-security-proxy", " AURAGO-SECURITY-PROXY "} {
		if !IsSecurityProxyContainerName(name) {
			t.Fatalf("security proxy container name %q was not recognized", name)
		}
	}
	for _, name := range []string{"", "aurago", "caddy", "security-proxy", "aurago-security-proxy-2", "my-aurago-security-proxy"} {
		if IsSecurityProxyContainerName(name) {
			t.Fatalf("unrelated container %q was recognized as the security proxy", name)
		}
	}
	if !ManagedBy(ManagedLabels(SecurityProxyOwner, "caddy", "proxy", ""), SecurityProxyOwner) {
		t.Fatal("the security proxy labels do not name their owner")
	}
}

// The CYD Creator builder reserves its exact container name and its two volume
// names; the test and CI volumes stay usable.
func TestCYDCreatorReservedNames(t *testing.T) {
	t.Parallel()

	if CYDCreatorOwner != "cyd-creator" || CYDCreatorBuilderContainerName != "aurago-cyd-builder" ||
		CYDCreatorWorkVolumeName != "aurago-cyd-work" || CYDCreatorCacheVolumeName != "aurago-cyd-cache" {
		t.Fatalf("owner = %q, container = %q, volumes = %q, %q", CYDCreatorOwner, CYDCreatorBuilderContainerName, CYDCreatorWorkVolumeName, CYDCreatorCacheVolumeName)
	}
	for _, name := range []string{"aurago-cyd-builder", "/aurago-cyd-builder", " AURAGO-CYD-BUILDER "} {
		if !IsCYDCreatorBuilderContainerName(name) {
			t.Fatalf("builder container name %q was not recognized", name)
		}
	}
	for _, name := range []string{"", "aurago", "cyd-creator", "aurago-cyd-builder-2", "proj-aurago-cyd-builder-1", "aurago-cyd-builder-stub", "aurago-cyd-cache"} {
		if IsCYDCreatorBuilderContainerName(name) {
			t.Fatalf("unrelated container %q was recognized as the CYD builder", name)
		}
	}
	for _, name := range []string{"aurago-cyd-work", "aurago-cyd-cache", " AURAGO-CYD-CACHE "} {
		if !IsCYDCreatorVolumeName(name) {
			t.Fatalf("builder volume %q was not recognized", name)
		}
	}
	for _, name := range []string{"", "aurago-cyd-work-it", "aurago-cyd-cache-it", "cyd-ci-work", "cyd-ci-cache-123", "proj_aurago-cyd-cache", "aurago-cyd-builder"} {
		if IsCYDCreatorVolumeName(name) {
			t.Fatalf("test or unrelated volume %q was recognized as a CYD builder volume", name)
		}
	}
	// The builder's labels name the owner, so label-based ownership needs no
	// extra rule.
	if !ManagedBy(ManagedLabels(CYDCreatorOwner, "cyd-creator", "builder", "abc"), CYDCreatorOwner) ||
		!ManagedBy(map[string]string{"aurago.managed": "cyd-creator"}, CYDCreatorOwner) {
		t.Fatal("the CYD builder labels do not name their owner")
	}
}

// The builder image is the published repository plus the configured
// builder_image repository, with any tag or digest; Docker Hub spellings of a
// configured local tag are the same repository.
func TestCYDCreatorImageReference(t *testing.T) {
	t.Parallel()

	for ref, want := range map[string]string{
		"ghcr.io/antibyte/aurago-cyd-builder:1":          "ghcr.io/antibyte/aurago-cyd-builder",
		"GHCR.IO/antibyte/aurago-cyd-builder@sha256:abc": "ghcr.io/antibyte/aurago-cyd-builder",
		"aurago-cyd-builder:dev":                         "aurago-cyd-builder",
		"docker.io/library/aurago-cyd-builder:dev":       "aurago-cyd-builder",
		"library/nginx":                                  "nginx",
		"registry.local:5000/team/img:2@sha256:abc":      "registry.local:5000/team/img",
		"ghcr.io/library/x:1":                            "ghcr.io/library/x",
		"":                                               "",
	} {
		if got := ImageRepository(ref); got != want {
			t.Fatalf("ImageRepository(%q) = %q, want %q", ref, got, want)
		}
	}
	for ref, want := range map[string]bool{
		"ghcr.io/antibyte/aurago-cyd-builder:1":    true,
		"localhost/aurago-cyd-builder:dev":         true,
		"registry:5000/x":                          true,
		"docker.io/library/aurago-cyd-builder:dev": true,
		"aurago-cyd-builder:dev":                   false,
		"antibyte/aurago-cyd-builder:1":            false,
	} {
		if got := ImageReferenceHasRegistry(ref); got != want {
			t.Fatalf("ImageReferenceHasRegistry(%q) = %v, want %v", ref, got, want)
		}
	}
	if got := ImageReferenceDigest("ghcr.io/antibyte/aurago-cyd-builder:1@SHA256:ABC"); got != "sha256:abc" || ImageReferenceDigest("x:1") != "" {
		t.Fatalf("ImageReferenceDigest = %q", got)
	}
	for _, ref := range []string{"ghcr.io/antibyte/aurago-cyd-builder:1", "ghcr.io/antibyte/aurago-cyd-builder:latest",
		"ghcr.io/antibyte/aurago-cyd-builder@sha256:" + strings.Repeat("a", 64), " GHCR.IO/ANTIBYTE/AURAGO-CYD-BUILDER "} {
		if !IsCYDCreatorImageReference(ref, "") {
			t.Fatalf("published builder image %q was not recognized", ref)
		}
	}
	for _, ref := range []string{"aurago-cyd-builder:dev", "aurago-cyd-builder-stub:test", "ghcr.io/antibyte/aurago-cyd-builder-stub:1",
		"ghcr.io/antibyte/aurago:1", "ghcr.io/other/aurago-cyd-builder:1", ""} {
		if IsCYDCreatorImageReference(ref, "") {
			t.Fatalf("unrelated image %q was recognized as the builder image", ref)
		}
	}
	if !IsCYDCreatorImageReference("docker.io/library/aurago-cyd-builder:other", "aurago-cyd-builder:dev") ||
		!IsCYDCreatorImageReference("registry.local/cyd:2", "registry.local/cyd:1") ||
		IsCYDCreatorImageReference("aurago-cyd-builder-stub:test", "aurago-cyd-builder:dev") {
		t.Fatal("the configured builder image repository is not matched exactly")
	}
}
