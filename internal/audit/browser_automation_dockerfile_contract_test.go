package audit

import (
	"path"
	"regexp"
	"strings"
	"testing"
)

func TestBrowserAutomationDockerfileCopiesSidecarRuntimeScripts(t *testing.T) {
	t.Parallel()

	dockerfile := readRepoFile(t, "Dockerfile.browser_automation")
	copied := browserAutomationSidecarJSCopied(dockerfile)

	for _, sidecarFile := range []string{
		"browser_automation_sidecar/server.js",
		"browser_automation_sidecar/egress_proxy.js",
	} {
		source := readRepoFile(t, sidecarFile)
		for _, dep := range browserAutomationSidecarLocalDeps(source) {
			if browserAutomationSidecarDepExcluded(dep) {
				continue
			}
			if !copied[dep] {
				t.Fatalf("%s depends on %s but Dockerfile.browser_automation does not COPY browser_automation_sidecar/%s", sidecarFile, dep, dep)
			}
		}
	}
}

func browserAutomationSidecarJSCopied(dockerfile string) map[string]bool {
	copied := map[string]bool{}
	for _, line := range strings.Split(dockerfile, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "COPY" {
			continue
		}
		source := fields[1]
		if !strings.HasPrefix(source, "browser_automation_sidecar/") || !strings.HasSuffix(source, ".js") {
			continue
		}
		copied[path.Base(source)] = true
	}
	return copied
}

var (
	browserAutomationLocalRequire = regexp.MustCompile(`require\(['"]\./([^'"]+)['"]\)`)
	browserAutomationSpawnJS      = regexp.MustCompile(`spawn\([^)]*['"]([^'"]+\.js)['"]`)
	browserAutomationPathJoinJS   = regexp.MustCompile(`path\.join\(__dirname,\s*['"]([^'"]+\.js)['"]\)`)
)

func browserAutomationSidecarLocalDeps(source string) []string {
	seen := map[string]bool{}
	var deps []string
	add := func(name string) {
		if !strings.HasSuffix(name, ".js") {
			name += ".js"
		}
		if seen[name] {
			return
		}
		seen[name] = true
		deps = append(deps, name)
	}
	for _, match := range browserAutomationLocalRequire.FindAllStringSubmatch(source, -1) {
		add(match[1])
	}
	for _, match := range browserAutomationSpawnJS.FindAllStringSubmatch(source, -1) {
		add(path.Base(match[1]))
	}
	for _, match := range browserAutomationPathJoinJS.FindAllStringSubmatch(source, -1) {
		add(path.Base(match[1]))
	}
	return deps
}

func browserAutomationSidecarDepExcluded(name string) bool {
	if strings.HasSuffix(name, ".test.js") {
		return true
	}
	return name == "test_auth.js"
}
