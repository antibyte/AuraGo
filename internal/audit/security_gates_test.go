package audit

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGoSecurityChecksRunIndependently(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Needs    any    `yaml:"needs"`
			If       string `yaml:"if"`
			Continue bool   `yaml:"continue-on-error"`
			Steps    []struct {
				Run      string `yaml:"run"`
				If       string `yaml:"if"`
				Continue bool   `yaml:"continue-on-error"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(readRepoFile(t, ".github/workflows/security-gates.yml")), &workflow); err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{"tests": "go test ./...", "race": "go test -race ", "vulnerabilities": "govulncheck ./..."}
	owners := map[string]string{}
	for id, job := range workflow.Jobs {
		for _, step := range job.Steps {
			for check, command := range checks {
				if !strings.Contains(step.Run, command) {
					continue
				}
				if previous := owners[check]; previous != "" {
					t.Fatalf("%s occurs in both %s and %s", check, previous, id)
				}
				owners[check] = id
				if job.Needs != nil || job.If != "" || job.Continue || step.If != "" || step.Continue {
					t.Errorf("%s can be skipped or ignored", check)
				}
				if check == "race" {
					for _, pkg := range []string{"llm", "server", "memory", "tools", "telegram"} {
						if !strings.Contains(step.Run, "./internal/"+pkg) {
							t.Errorf("race check omits %s", pkg)
						}
					}
				}
			}
		}
	}
	seen := map[string]bool{}
	for check := range checks {
		id := owners[check]
		if id == "" || seen[id] {
			t.Fatalf("checks must have distinct jobs: %v", owners)
		}
		seen[id] = true
	}
}
