package invasion

import "testing"

func TestDockerRemotePlaintextOnlyForDockerRemote(t *testing.T) {
	for method, want := range map[string]bool{
		"docker_remote": true,
		"docker_local":  false,
		"ssh":           false,
		"":              false,
	} {
		if got := DockerRemotePlaintext(NestRecord{DeployMethod: method}); got != want {
			t.Fatalf("DockerRemotePlaintext(%q) = %v, want %v", method, got, want)
		}
	}
}
