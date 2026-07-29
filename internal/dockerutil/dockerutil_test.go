package dockerutil

import (
	"reflect"
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

func TestParseNumericGroupIDs(t *testing.T) {
	t.Parallel()

	got := ParseNumericGroupIDs(" 109,44,109,0,-1,nope,2147483648 ")
	want := []string{"109", "44"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseNumericGroupIDs() = %#v, want %#v", got, want)
	}
}
