package cydcreator

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// itBuild is one compile of the opt-in builder integration test.
type itBuild struct {
	Case  string // golden case name, or "reference"
	Board string
}

// itBuilds are the compiles the plan 1c pre-verification ran: the all-widgets
// project on the classic CYD and on an ESP32-S3 board, its portrait variant,
// every theme sample on the classic CYD and the reference for every board.
func itBuilds(t *testing.T) []itBuild {
	t.Helper()
	builds := []itBuild{
		{"all_widgets", "esp32-2432s028r"},
		{"all_widgets", "esp32-8048s043"},
		{"all_widgets_portrait", "esp32-2432s028r"},
	}
	for _, th := range []string{"clear", "night", "terminal", "sandstorm", "dark_sun", "synthwave", "high_contrast"} {
		builds = append(builds, itBuild{"theme_" + th, "esp32-2432s028r"})
	}
	boards, err := Boards()
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range boards {
		builds = append(builds, itBuild{"reference", b.ID})
	}
	if only := os.Getenv("CYD_BUILDER_IT_ONLY"); only != "" {
		var keep []itBuild
		for _, b := range builds {
			for _, part := range strings.Split(only, ",") {
				if part != "" && strings.Contains(b.Case+"@"+b.Board, part) {
					keep = append(keep, b)
					break
				}
			}
		}
		builds = keep
	}
	return builds
}

func itEnv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func itDocker(t *testing.T, stdin []byte, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("docker", args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// itTar packs a generated tree (sorted, so the archive is deterministic).
func itTar(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, name := range names {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(files[name])), ModTime: time.Unix(0, 0), Format: tar.FormatPAX}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// itProject loads the input project of a build with the build's board.
func itProject(t *testing.T, b itBuild) Project {
	t.Helper()
	dir := filepath.Join(goldenDir, b.Case)
	if b.Case == "reference" {
		dir = referenceDir
	}
	raw, err := os.ReadFile(filepath.Join(dir, "cyd.project.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseProject(raw)
	if err != nil {
		t.Fatal(err)
	}
	p.Board = b.Board
	return p
}

var itGeneratedProblem = regexp.MustCompile(`(?m)^.*src/generated/[^:\n]*:\d+:\d+: (warning|error): .*$`)

// TestGenerateCompilesInBuilder compiles generated trees inside the builder
// image with AuraGo's container settings (plan 2): read-only root, 1 GiB
// /tmp tmpfs with exec, HOME=/tmp, uid 1000, no network, named volumes on
// /work and /cache. Opt-in, needs Docker and the image:
//
//	CYD_BUILDER_IT=1 CYD_BUILDER_IMAGE=aurago-cyd-builder:dev go test ./internal/cydcreator/ -run TestGenerateCompilesInBuilder -v -timeout 90m
//
// CYD_BUILDER_IT_ONLY=<comma-separated substrings of case@board> selects builds,
// CYD_BUILDER_IT_CPUS (default 2) speeds up local runs and CYD_BUILDER_IT_STRICT=1
// adds -Wall -Wextra for src/ (generated and app code; a warning there fails).
func TestGenerateCompilesInBuilder(t *testing.T) {
	if os.Getenv("CYD_BUILDER_IT") != "1" {
		t.Skip("set CYD_BUILDER_IT=1 to compile generated projects in the builder image")
	}
	image := itEnv("CYD_BUILDER_IMAGE", "ghcr.io/antibyte/aurago-cyd-builder:1")
	name := fmt.Sprintf("aurago-cyd-it-%d", time.Now().UnixNano())
	out, err := itDocker(t, nil, "run", "-d", "--name", name, "--label", "aurago.cyd-plan=1",
		"--network", "none", "--read-only", "--tmpfs", "/tmp:rw,exec,size=1g", "-e", "HOME=/tmp",
		"--user", "1000:1000", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
		"--memory", "2g", "--cpus", itEnv("CYD_BUILDER_IT_CPUS", "2"), "--pids-limit", "512",
		"-v", itEnv("CYD_BUILDER_WORK_VOLUME", "aurago-cyd-work")+":/work",
		"-v", itEnv("CYD_BUILDER_CACHE_VOLUME", "aurago-cyd-cache")+":/cache",
		image, "sleep", "infinity")
	if err != nil {
		t.Fatalf("docker run: %v\n%s", err, out)
	}
	t.Cleanup(func() { _, _ = itDocker(t, nil, "rm", "-f", name) })

	for _, b := range itBuilds(t) {
		t.Run(b.Case+"@"+b.Board, func(t *testing.T) {
			p := itProject(t, b)
			res, err := Generate(GenerateInput{Project: p, ProjectID: testProjectID, AppFiles: ScaffoldAppFiles(p), Assets: goldenAssets(t)})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if b.Case == "reference" {
				app, err := os.ReadFile(filepath.Join(referenceDir, "src", "app", "app_main.cpp"))
				if err != nil {
					t.Fatal(err)
				}
				res.Files["src/app/app_main.cpp"] = app
			}
			if os.Getenv("CYD_BUILDER_IT_STRICT") == "1" {
				// Test-only: compile src/ (generated + app code) with -Wall -Wextra.
				res.Files["platformio.ini"] = append(res.Files["platformio.ini"], []byte("build_src_flags =\n  -Wall\n  -Wextra\n")...)
			}
			id := strings.ReplaceAll("it-"+b.Case+"-"+b.Board, "_", "-")
			work := "/work/" + id
			if out, err := itDocker(t, itTar(t, res.Files), "exec", "-i", name, "sh", "-c",
				"rm -rf "+work+" && mkdir -p "+work+" && tar -x -C "+work); err != nil {
				t.Fatalf("upload: %v\n%s", err, out)
			}
			start := time.Now()
			log, err := itDocker(t, nil, "exec", name, "cyd-build", "--work", work, "--env", b.Board,
				"--build-id", id, "--cache-key", "it-"+b.Board)
			if err != nil {
				t.Fatalf("cyd-build failed after %s: %v\n%s", time.Since(start).Round(time.Second), err, tail(log, 80))
			}
			if probs := itGeneratedProblem.FindAllString(log, -1); len(probs) > 0 {
				t.Errorf("warnings/errors in generated code:\n%s", strings.Join(probs, "\n"))
			}
			raw, err := itDocker(t, nil, "exec", name, "cat", work+"/out/result.json")
			if err != nil {
				t.Fatalf("result.json: %v\n%s", err, raw)
			}
			var result struct {
				OK       bool `json:"ok"`
				Warnings int  `json:"warnings"`
				Flash    struct{ Used, Total int }
				RAM      struct{ Used, Total int }
			}
			if err := json.Unmarshal([]byte(raw), &result); err != nil || !result.OK {
				t.Fatalf("result.json %q: %v", raw, err)
			}
			t.Logf("%s on %s: flash %d/%d (%.1f %%), ram %d/%d, %d warnings, %s", b.Case, b.Board,
				result.Flash.Used, result.Flash.Total, 100*float64(result.Flash.Used)/float64(max(result.Flash.Total, 1)),
				result.RAM.Used, result.RAM.Total, result.Warnings, time.Since(start).Round(time.Second))
			_, _ = itDocker(t, nil, "exec", name, "rm", "-rf", work)
		})
	}
}

func tail(s string, lines int) string {
	parts := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n")
}
