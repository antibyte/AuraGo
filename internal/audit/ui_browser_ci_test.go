package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestUIBrowserCIShardsCoverEveryTest(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Env      map[string]string `yaml:"env"`
			Strategy struct {
				FailFast *bool `yaml:"fail-fast"`
				Matrix   struct {
					Include []struct{ Name, Run, Skip string } `yaml:"include"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
			Steps []struct{ Run string } `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(readRepoFile(t, ".github/workflows/security-gates.yml")), &workflow); err != nil {
		t.Fatal(err)
	}
	job := workflow.Jobs["ui-browser"]
	if job.Env["AURAGO_RUN_BROWSER_SMOKE"] != "1" || job.Strategy.FailFast == nil || *job.Strategy.FailFast {
		t.Fatal("browser jobs must require a browser and finish independently after another shard fails")
	}
	usesSelectors := false
	for _, step := range job.Steps {
		usesSelectors = usesSelectors || strings.Contains(step.Run, "go test ./ui ") &&
			strings.Contains(step.Run, "-run '${{ matrix.run }}'") && strings.Contains(step.Run, "-skip '${{ matrix.skip }}'")
	}
	if !usesSelectors {
		t.Fatal("UI test command does not apply both matrix selectors")
	}
	type selection struct {
		run, skip *regexp.Regexp
		count     int
	}
	var selections []selection
	for _, shard := range job.Strategy.Matrix.Include {
		if strings.ContainsAny(shard.Run+shard.Skip, "/") {
			t.Fatal("UI shard selectors must select whole top-level tests")
		}
		run, err := regexp.Compile(shard.Run)
		if err != nil {
			t.Fatal(err)
		}
		skip, err := regexp.Compile(shard.Skip)
		if err != nil {
			t.Fatal(err)
		}
		selections = append(selections, selection{run: run, skip: skip})
	}
	entries, err := os.ReadDir(repoPath("ui"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), repoPath(filepath.Join("ui", entry.Name())), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name == "TestMain" {
				continue
			}
			name := fn.Name.Name
			if !strings.HasPrefix(name, "Test") && !strings.HasPrefix(name, "Example") && !strings.HasPrefix(name, "Fuzz") {
				continue
			}
			matches := 0
			for i := range selections {
				if selections[i].run.MatchString(name) && !selections[i].skip.MatchString(name) {
					matches++
					selections[i].count++
				}
			}
			if matches != 1 {
				t.Errorf("%s belongs to %d UI shards, want exactly one", name, matches)
			}
		}
	}
	for i, selected := range selections {
		if selected.count == 0 {
			t.Errorf("UI shard %q selects no tests", job.Strategy.Matrix.Include[i].Name)
		}
		t.Logf("%s: %d top-level tests", job.Strategy.Matrix.Include[i].Name, selected.count)
	}
}
