package agent

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var dispatchCasePattern = regexp.MustCompile(`case\s+((?:"[^"]+"\s*,\s*)*"[^"]+")\s*:`)
var dispatchStringPattern = regexp.MustCompile(`"([^"]+)"`)

func TestBuiltinToolSchemasHaveDispatchCoverage(t *testing.T) {
	dispatchFiles := []string{
		"agent_dispatch_exec.go",
		"dispatch_shell.go",
		"dispatch_python.go",
		"dispatch_filesystem.go",
		"agent_dispatch_comm.go",
		"dispatch_email.go",
		"dispatch_messaging.go",
		"agent_dispatch_services.go",
		"agent_dispatch_infra.go",
		"dispatch_network.go",
		"dispatch_cloud.go",
		"dispatch_platform.go",
	}

	handled := make(map[string]struct{})
	for _, file := range dispatchFiles {
		data, err := os.ReadFile(filepath.Join(".", file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		matches := dispatchCasePattern.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			for _, literal := range dispatchStringPattern.FindAllStringSubmatch(match[1], -1) {
				if len(literal) < 2 {
					continue
				}
				handled[literal[1]] = struct{}{}
			}
		}
	}

	var missing []string
	for _, name := range builtinToolNames(allBuiltinToolFeatureFlags()) {
		if _, ok := handled[name]; !ok {
			missing = append(missing, name)
		}
	}

	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("builtin tool schemas without a matching dispatch case:\n%s", strings.Join(missing, "\n"))
	}
}
