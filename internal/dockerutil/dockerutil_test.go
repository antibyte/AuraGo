package dockerutil

import "testing"

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
