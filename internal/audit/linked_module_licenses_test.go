package audit

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// gpl3CompatibleLicenses are the licences a module linked into cmd/aurago may carry, each with the
// phrases its licence file must contain (lower case, whitespace collapsed). Anything else
// (GPL-2.0-only, EPL without a secondary-licence grant, CDDL, SSPL, BUSL, Commons Clause, AGPL, an
// unknown text) blocks a release until a reviewed licence is recorded.
var gpl3CompatibleLicenses = map[string][]string{
	"GPL-3.0-only": {"gnu general public license", "version 3, 29 june 2007"},
	"MPL-2.0":      {"mozilla public license", "2.0"},
	"Apache-2.0":   {"apache license", "version 2.0"},
	"BSD-3-Clause": {"redistribution and use in source and binary forms", "endorse or promote"},
	"BSD-2-Clause": {"redistribution and use in source and binary forms"},
	"ISC":          {"permission to use, copy, modify, and", "distribute this software for any purpose"},
	"MIT":          {"permission is hereby granted, free of charge"},
	"Zlib":         {"this software is provided 'as-is'", "altered source versions must be plainly marked"},
	"Unlicense":    {"this is free and unencumbered software released into the public domain"},
	"CC0-1.0":      {"cc0 1.0 universal"},
}

var licenseGuessOrder = []string{"GPL-3.0-only", "MPL-2.0", "Apache-2.0", "BSD-3-Clause", "BSD-2-Clause", "ISC", "MIT", "Zlib", "Unlicense", "CC0-1.0"}

var releaseTargets = []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64"}

// licenseFileCandidates is the order in which a missing manifest row's licence file is suggested.
var licenseFileCandidates = []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "LICENCE", "COPYING"}

const linkedModuleManifestPath = "licences/linked-go-modules.tsv"

type linkedModuleLicense struct {
	Path, SPDX, Note string
	Files            []string
}

// normalizedLicenseText lower-cases a licence text, turns curly quotes into ASCII quotes and collapses
// every whitespace run, so hard-wrapped and re-flowed copies of one licence compare equal.
func normalizedLicenseText(raw []byte) string {
	text := strings.ToLower(string(raw))
	text = strings.NewReplacer("‘", "'", "’", "'", "“", `"`, "”", `"`).Replace(text)
	return strings.Join(strings.Fields(text), " ")
}

func licenseHasPhrases(text string, phrases []string) bool {
	for _, phrase := range phrases {
		if !strings.Contains(text, phrase) {
			return false
		}
	}
	return true
}

// licenseGuess returns the first licence of licenseGuessOrder whose phrases all occur in text.
func licenseGuess(text string) string {
	for _, id := range licenseGuessOrder {
		if licenseHasPhrases(text, gpl3CompatibleLicenses[id]) {
			return id
		}
	}
	return "UNKNOWN"
}

func readLinkedModuleManifest(t *testing.T) []linkedModuleLicense {
	t.Helper()
	var rows []linkedModuleLicense
	for i, line := range strings.Split(readRepoFile(t, linkedModuleManifestPath), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 3 || len(cols) > 4 {
			t.Fatalf("%s:%d: want 3 or 4 tab-separated columns (module, SPDX, files, note), got %d", linkedModuleManifestPath, i+1, len(cols))
		}
		row := linkedModuleLicense{Path: strings.TrimSpace(cols[0]), SPDX: strings.TrimSpace(cols[1])}
		for _, file := range strings.Split(cols[2], ",") {
			if file = strings.TrimSpace(file); file != "" {
				row.Files = append(row.Files, file)
			}
		}
		if len(cols) == 4 {
			row.Note = strings.TrimSpace(cols[3])
		}
		if row.Path == "" || row.SPDX == "" || len(row.Files) == 0 {
			t.Fatalf("%s:%d: module, SPDX and at least one licence file are required", linkedModuleManifestPath, i+1)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		t.Fatalf("%s has no rows", linkedModuleManifestPath)
	}
	return rows
}

// linkedModules lists the modules compiled into cmd/aurago for one release target, as module path to
// the directory in the module cache. It reports failures with t.Errorf and returns nil, so goroutines
// can call it; the caller stops from the test goroutine.
func linkedModules(t *testing.T, target string) map[string]string {
	t.Helper()
	goos, goarch, ok := strings.Cut(target, "/")
	if !ok {
		t.Errorf("bad release target %q", target)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Dir}}{{end}}{{end}}", "./cmd/aurago")
	cmd.Dir = repoPath()
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0", "GOFLAGS=-mod=readonly")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("go list -deps ./cmd/aurago for %s: %v\n%s\nrun go mod download if modules are missing", target, err, strings.TrimSpace(stderr.String()))
		return nil
	}
	modules := map[string]string{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		path, dir, _ := strings.Cut(line, "\t")
		if strings.TrimSpace(dir) == "" {
			t.Errorf("%s: module %s has no directory in the module cache for %s; run go mod download", linkedModuleManifestPath, path, target)
			return nil
		}
		modules[path] = dir
	}
	if len(modules) == 0 {
		t.Errorf("go list -deps ./cmd/aurago for %s printed no modules", target)
		return nil
	}
	return modules
}

// exactFileExists is true when rel exists below dir with exactly this letter case, so a manifest row
// that works on a case-insensitive Windows disk cannot fail on a Linux CI runner.
func exactFileExists(dir, rel string) bool {
	rel = filepath.FromSlash(rel)
	entries, err := os.ReadDir(filepath.Join(dir, filepath.Dir(rel)))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() == filepath.Base(rel) && !entry.IsDir() {
			return true
		}
	}
	return false
}

// suggestedLicenseRow returns the licence guess and file name for a module without a manifest row.
func suggestedLicenseRow(dir string) (guess, file string) {
	for _, name := range licenseFileCandidates {
		if !exactFileExists(dir, name) {
			continue
		}
		if raw, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			return licenseGuess(normalizedLicenseText(raw)), name
		}
	}
	return "UNKNOWN", "-"
}

// moduleHasIncompatibleSecondaryLicenseHeader reports whether a non-test Go file of the module marks
// itself "Incompatible With Secondary Licenses" (MPL-2.0 section 1.12), which forbids a GPL combination.
func moduleHasIncompatibleSecondaryLicenseHeader(dir string) (string, bool, error) {
	const marker = "incompatible with secondary licenses"
	var found string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(strings.ToLower(string(raw)), marker) {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	return found, found != "", err
}

// The fingerprints are the gate's only licence recognition. They must tell the shipped SPDX texts apart,
// and must not recognise a licence that is incompatible with GPL-3.0.
func TestLinkedModuleLicenseFingerprintsClassifyTheShippedTexts(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"MIT.txt":          "MIT",
		"Apache-2.0.txt":   "Apache-2.0",
		"BSD-3-Clause.txt": "BSD-3-Clause",
		"BSD-2-Clause.txt": "BSD-2-Clause",
		"ISC.txt":          "ISC",
		"MPL-2.0.txt":      "MPL-2.0",
		"GPL-3.0-only.txt": "GPL-3.0-only",
		"Unlicense.txt":    "Unlicense",
		"Zlib.txt":         "Zlib",
		// Not allowed: the gate must not recognise these as compatible.
		"GPL-2.0-only.txt":  "UNKNOWN",
		"AGPL-3.0-only.txt": "UNKNOWN",
		"EPL-2.0.txt":       "UNKNOWN",
		"SSPL-1.0.txt":      "UNKNOWN",
	}
	for name, id := range want {
		raw := []byte(readRepoFile(t, "licences/texts/"+name))
		if got := licenseGuess(normalizedLicenseText(raw)); got != id {
			t.Errorf("licences/texts/%s is classified %q, want %q", name, got, id)
		}
	}
}

// A GPL-3.0 combined work may only link modules whose licence is GPL-3.0 compatible. Every module
// linked into cmd/aurago for a release target needs a reviewed row in licences/linked-go-modules.tsv;
// a licence outside the allowlist blocks the release (licences/AGENTS.md).
func TestLinkedGoModulesHaveGPL3CompatibleLicenses(t *testing.T) {
	if testing.Short() {
		t.Skip("lists the linked modules of six release targets; skipped with -short")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on PATH")
	}
	manifest := readLinkedModuleManifest(t)
	notices := readRepoFile(t, "THIRD_PARTY_NOTICES.md")

	perTarget := make([]map[string]string, len(releaseTargets))
	var wg sync.WaitGroup
	for i, target := range releaseTargets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			perTarget[i] = linkedModules(t, target)
		}()
	}
	wg.Wait()
	linked := map[string]string{}
	for _, modules := range perTarget {
		if modules == nil {
			t.FailNow()
		}
		for path, dir := range modules {
			linked[path] = dir
		}
	}

	rows := map[string]linkedModuleLicense{}
	previous := ""
	for _, row := range manifest {
		switch {
		case row.Path == previous:
			t.Errorf("%s: duplicate row for %s", linkedModuleManifestPath, row.Path)
		case row.Path < previous:
			t.Errorf("%s: %s is not sorted after %s", linkedModuleManifestPath, row.Path, previous)
		}
		previous = row.Path
		rows[row.Path] = row
		if _, ok := linked[row.Path]; !ok {
			t.Errorf("%s: stale row %s; no release target links it", linkedModuleManifestPath, row.Path)
		}
	}

	var paths []string
	for path := range linked {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var missing []string
	for _, path := range paths {
		if _, ok := rows[path]; !ok {
			guess, file := suggestedLicenseRow(linked[path])
			missing = append(missing, fmt.Sprintf("add: %s\t%s\t%s", path, guess, file))
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d linked module(s) have no reviewed row in %s; read each licence file, then add the line (module, SPDX, files, note):\n%s",
			len(missing), linkedModuleManifestPath, strings.Join(missing, "\n"))
	}

	for _, row := range manifest {
		dir, ok := linked[row.Path]
		if !ok {
			continue
		}
		parts := strings.Split(row.SPDX, " AND ")
		allowed := true
		for _, part := range parts {
			if _, known := gpl3CompatibleLicenses[part]; !known {
				allowed = false
				t.Errorf("%s: %s is not GPL-3.0 compatible; release blocked", row.Path, part)
			}
		}
		if !allowed {
			continue
		}
		texts := map[string]string{}
		for _, file := range row.Files {
			if !exactFileExists(dir, file) {
				t.Errorf("%s: licence file %s does not exist in the module", row.Path, file)
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file)))
			if err != nil {
				t.Errorf("%s: read %s: %v", row.Path, file, err)
				continue
			}
			texts[file] = normalizedLicenseText(raw)
		}
		for _, part := range parts {
			found := false
			for _, text := range texts {
				if licenseHasPhrases(text, gpl3CompatibleLicenses[part]) {
					found = true
					break
				}
			}
			if !found && len(texts) == len(row.Files) {
				t.Errorf("%s: none of %s carries the %s text; release blocked until the licence is reviewed", row.Path, strings.Join(row.Files, ", "), part)
			}
			switch part {
			case "MPL-2.0":
				if file, bad, err := moduleHasIncompatibleSecondaryLicenseHeader(dir); err != nil {
					t.Errorf("%s: scan Go files: %v", row.Path, err)
				} else if bad {
					t.Errorf("%s: %s is marked Incompatible With Secondary Licenses (MPL-2.0); it cannot be combined with GPL-3.0", row.Path, file)
				}
			case "GPL-3.0-only":
				if !strings.Contains(notices, "`"+row.Path+"`") {
					t.Errorf("%s: GPL-3.0 module is not named in backticks in THIRD_PARTY_NOTICES.md", row.Path)
				}
			}
		}
	}
}
