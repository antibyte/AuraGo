package audit

import (
	"strings"
	"testing"
)

func TestDockerInstallationDocStatesSocketProxyResidualRisk(t *testing.T) {
	t.Parallel()

	doc := readRepoFile(t, "documentation/docker_installation.md")
	start := strings.Index(doc, "## 4. Docker Socket Security")
	end := strings.Index(doc, "## 5. Upgrading")
	if start < 0 || end < start {
		t.Fatal("docker_installation.md must keep sections 4 and 5")
	}
	section := doc[start:end]
	for _, want := range []string{
		"The proxy reduces attack surface but does not contain a compromised AuraGo: container create with host binds is allowed.",
		"the agent's Docker `exec` operation",
		"the Homepage tool",
		"CommandCode Store terminal",
		"OpenSCAD",
		"security proxy reload",
		"is no security gain",
		"docker exec aurago /app/aurago --print-homepage-dockerfile | docker build -t aurago-homepage:latest -",
	} {
		if !strings.Contains(section, want) {
			t.Fatalf("docker_installation.md section 4 is missing %q", want)
		}
	}
	for _, stale := range []string{
		"Limited to allowed operations",
		"`EXEC=1` can be set to `0` when Code Studio terminals, container terminals and the security proxy reload are not used.",
	} {
		if strings.Contains(section, stale) {
			t.Fatalf("docker_installation.md section 4 still says %q", stale)
		}
	}

	compose := readRepoFile(t, "docker-compose.yml")
	for _, want := range []string{
		"does not contain a compromised AuraGo",
		":ro is no security gain",
		"Homepage tool",
		"OpenSCAD",
	} {
		if !strings.Contains(compose, want) {
			t.Fatalf("docker-compose.yml proxy comments are missing %q", want)
		}
	}

	const dockerGuideLink = "../../docker_installation.md#4-docker-socket-security"
	for _, manual := range []string{
		"documentation/manual/en/02-installation.md",
		"documentation/manual/de/02-installation.md",
	} {
		if !strings.Contains(readRepoFile(t, manual), dockerGuideLink) {
			t.Fatalf("%s must link to %s", manual, dockerGuideLink)
		}
	}
}
