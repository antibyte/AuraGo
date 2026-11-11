package tools

import (
	"strings"
	"testing"
)

func TestHAExecutableServicesNeedExplicitGrant(t *testing.T) {
	for _, domain := range []string{"shell_command", "python_script", "script", "hassio"} {
		t.Run(domain, func(t *testing.T) {
			if got := HAServiceRefusal(domain, "run", nil, nil); !strings.Contains(got, "explicit") {
				t.Fatalf("missing explicit-grant denial: %q", got)
			}
			allowed := []string{strings.ToUpper(domain) + ".RUN"}
			if got := HAServiceRefusal(" "+domain+" ", " run ", allowed, nil); got != "" {
				t.Fatal(got)
			}
			if got := HAServiceRefusal(domain, "run", allowed, []string{domain + ".run"}); !strings.Contains(got, "blocked") {
				t.Fatalf("blocklist did not win: %q", got)
			}
			if HAServiceAllowed(HAConfig{ReadOnly: true, AllowedServices: allowed}, domain, "run") {
				t.Fatal("allowlist bypassed read-only")
			}
		})
	}
	if got := HAServiceRefusal("light", "turn_on", nil, nil); got != "" {
		t.Fatalf("ordinary switch denied: %s", got)
	}
}
