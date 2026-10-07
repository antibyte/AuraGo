package audit

import (
	"path"
	"strings"
	"testing"
)

func TestRootDockerignoreKeepsSecretsAndWorktreesOutOfTheBuildContext(t *testing.T) {
	t.Parallel()
	excluded := []string{"secrets/", ".worktrees/", ".claude/", ".kilo/", ".kilocode/", ".gitnexus/"}
	entries := map[string]bool{}
	for _, line := range strings.Split(readRepoFile(t, ".dockerignore"), "\n") {
		entries[strings.TrimSpace(line)] = true
	}
	for _, want := range excluded {
		if !entries[want] {
			t.Fatalf(".dockerignore must exclude %s", want)
		}
	}
	// Every Dockerfile built with the repository root as its context
	// (docker-compose.yml, .github/workflows) must not copy from them.
	for _, dockerfile := range []string{
		"Dockerfile", "Dockerfile.ansible", "Dockerfile.browser_automation", "Dockerfile.browser_egress",
		"deploy/docker/Dockerfile.code-studio", "deploy/docker/Dockerfile.supertonic",
		"internal/acestep/runtime/Dockerfile", "internal/acestep/runtime/Dockerfile.vulkan",
		"internal/rtlsdr/runtime/Dockerfile", "internal/rtlsdr/runtime/Dockerfile.fixtures",
	} {
		for _, line := range strings.Split(readRepoFile(t, dockerfile), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 3 || (fields[0] != "COPY" && fields[0] != "ADD") {
				continue
			}
			for _, source := range fields[1 : len(fields)-1] {
				if strings.HasPrefix(source, "--") {
					continue // --from=<stage> or another option
				}
				cleaned := path.Clean(source) + "/"
				for _, dir := range excluded {
					if strings.HasPrefix(cleaned, dir) {
						t.Fatalf("%s copies %s from the excluded %s", dockerfile, source, dir)
					}
				}
			}
		}
	}
}
