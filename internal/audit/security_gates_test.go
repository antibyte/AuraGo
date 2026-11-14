package audit

import (
	"regexp"
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
			Timeout  int    `yaml:"timeout-minutes"`
			Strategy struct {
				FailFast *bool `yaml:"fail-fast"`
				Matrix   struct {
					Include []struct{ Name, Package, Run, Skip string } `yaml:"include"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
			Steps []struct {
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
				if (check == "tests" || check == "race") && !strings.Contains(step.Run, "-p 1") {
					t.Errorf("%s must serialize packages within runner resources", check)
				}
				if check == "race" {
					if job.Timeout != 60 || job.Strategy.FailFast == nil || *job.Strategy.FailFast || !strings.Contains(step.Run, "-timeout=30m") || !strings.Contains(step.Run, "./internal/${{ matrix.package }}") {
						t.Error("race packages must run independently with 30-minute tests and 60-minute jobs")
					}
					if !strings.Contains(step.Run, "-run '${{ matrix.run }}'") || !strings.Contains(step.Run, "-skip '${{ matrix.skip }}'") {
						t.Error("race command omits shard selectors")
					}
					packages := map[string]int{}
					for _, shard := range job.Strategy.Matrix.Include {
						packages[shard.Package]++
					}
					for pkg, count := range packages {
						want := 1
						if pkg == "agent" {
							want = 2
						}
						if count != want {
							t.Errorf("race package %s has %d shards, want %d", pkg, count, want)
						}
					}
					for _, name := range topLevelGoTestNames(t, "internal/agent") {
						matches := 0
						for _, shard := range job.Strategy.Matrix.Include {
							if shard.Package != "agent" {
								continue
							}
							run, err := regexp.Compile(shard.Run)
							if err != nil {
								t.Fatal(err)
							}
							skip, err := regexp.Compile(shard.Skip)
							if err != nil {
								t.Fatal(err)
							}
							if run.MatchString(name) && !skip.MatchString(name) {
								matches++
							}
						}
						if matches != 1 {
							t.Errorf("%s belongs to %d agent race shards, want exactly one", name, matches)
						}
					}
					for _, pkg := range []string{"llm", "server", "memory", "tools", "telegram", "whatsapp", "agent", "flows", "invasion", "security", "discord", "dbutil"} {
						if packages[pkg] == 0 {
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

func TestReviewGatesCheckoutExactHead(t *testing.T) {
	for _, name := range []string{"security-gates.yml", "web-assets.yml", "training-dataset.yml", "cyd-builder.yml", "rtl-sdr.yml"} {
		var workflow struct {
			Jobs map[string]struct {
				Steps []struct {
					Uses string            `yaml:"uses"`
					With map[string]string `yaml:"with"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(readRepoFile(t, ".github/workflows/"+name)), &workflow); err != nil {
			t.Fatal(err)
		}
		checkouts := 0
		for _, job := range workflow.Jobs {
			for _, step := range job.Steps {
				if strings.HasPrefix(step.Uses, "actions/checkout@") && step.With["repository"] == "" {
					checkouts++
					if step.With["ref"] != "${{ github.event.pull_request.head.sha || github.sha }}" {
						t.Errorf("%s checks out a merge ref instead of the reviewed head", name)
					}
				}
			}
		}
		if checkouts == 0 {
			t.Errorf("%s has no reviewed source checkout", name)
		}
	}
}
