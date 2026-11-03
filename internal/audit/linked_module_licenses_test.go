package audit

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"
)

// gpl3CompatibleLicenses are the licences a module linked into a release binary may carry, each with
// the phrases its licence file must contain (lower case, whitespace collapsed). Anything else
// (GPL-2.0-only, EPL without a secondary-licence grant, CDDL, SSPL, BUSL, Commons Clause, AGPL, an
// unknown text) blocks a release until a reviewed licence is recorded.
var gpl3CompatibleLicenses = map[string][]string{
	"GPL-3.0-only": {"gnu general public license", "version 3, 29 june 2007"},
	"MPL-2.0":      {"mozilla public license version 2.0", `"secondary license" means either the gnu general public license`},
	"Apache-2.0":   {"apache license", "version 2.0"},
	"BSD-3-Clause": {"redistribution and use in source and binary forms", "endorse or promote"},
	"BSD-2-Clause": {"redistribution and use in source and binary forms"},
	"ISC":          {"permission to use, copy, modify, and", "distribute this software for any purpose"},
	"MIT":          {"permission is hereby granted, free of charge"},
	"Zlib":         {"this software is provided 'as-is'", "altered source versions must be plainly marked"},
	"Unlicense":    {"this is free and unencumbered software released into the public domain"},
	"CC0-1.0":      {"cc0 1.0 universal"},
}

// forbiddenLicenseTexts are licence families that cannot be combined with GPL-3.0 into one work.
// A licence file listed in the manifest must not contain one of them, even next to an allowed text.
// A text matches when it has every phrase and none of the absent phrases.
var forbiddenLicenseTexts = []struct {
	ID      string
	Phrases []string
	Absent  []string
}{
	{ID: "GPL-2.0", Phrases: []string{"gnu general public license", "version 2, june 1991"}},
	{ID: "AGPL-3.0", Phrases: []string{"gnu affero general public license", "version 3, 19 november 2007"}},
	{ID: "SSPL-1.0", Phrases: []string{"server side public license"}},
	// EPL-2.0 is GPL-compatible only through a Secondary License notice that names the GPL.
	{ID: "EPL-2.0 without a GPL secondary licence", Phrases: []string{"eclipse public license"}, Absent: []string{"satisfied: gnu general public license"}},
}

// spdxAliases maps deprecated or "or later" identifiers of source file headers onto the allowlist.
var spdxAliases = map[string]string{
	"GPL-3.0":          "GPL-3.0-only",
	"GPL-3.0+":         "GPL-3.0-only",
	"GPL-3.0-or-later": "GPL-3.0-only",
}

var licenseGuessOrder = []string{"GPL-3.0-only", "MPL-2.0", "Apache-2.0", "BSD-3-Clause", "BSD-2-Clause", "ISC", "MIT", "Zlib", "Unlicense", "CC0-1.0"}

var releaseTargets = []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64"}

// linkedBinaryPackages are the binaries every release and the Docker image ship.
var linkedBinaryPackages = []string{"./cmd/aurago", "./cmd/remote", "./cmd/config-merger"}

// licenseFileCandidates is the order in which a missing manifest row's licence file is suggested.
var licenseFileCandidates = []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "LICENCE", "COPYING"}

const (
	linkedModuleManifestPath = "licences/linked-go-modules.tsv"
	// licenseHeaderBytes is how much of each compiled Go file is searched for licence headers.
	licenseHeaderBytes = 32 << 10
	// mplIncompatibleMarker is MPL-2.0 section 1.12: such a file cannot be combined with GPL code.
	mplIncompatibleMarker = "incompatible with secondary licenses"
)

var spdxHeaderLine = regexp.MustCompile(`(?m)^[ \t]*(?://|/\*+|\*|#)[ \t]*SPDX-License-Identifier:[ \t]*([^\r\n*]+)`)

type linkedModuleLicense struct {
	Path, SPDX, Note string
	Files            []string
}

// linkedModule is one module compiled into a release binary: its cache directory and the non-test Go
// files of its packages that the linked targets compile.
type linkedModule struct {
	Dir   string
	Files map[string]bool
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

// forbiddenLicense names the GPL-3.0-incompatible licence family text carries, or "" for none.
func forbiddenLicense(text string) string {
	for _, forbidden := range forbiddenLicenseTexts {
		if !licenseHasPhrases(text, forbidden.Phrases) {
			continue
		}
		excluded := false
		for _, absent := range forbidden.Absent {
			if strings.Contains(text, absent) {
				excluded = true
			}
		}
		if !excluded {
			return forbidden.ID
		}
	}
	return ""
}

// spdxExpressionProblem checks the SPDX-License-Identifier expression of a source header against the
// SPDX parts the manifest row declares. With OR one acceptable alternative is enough, otherwise every
// identifier must be allowed and declared. It returns "" when the expression is fine.
func spdxExpressionProblem(expression string, declared map[string]bool) string {
	tokens := strings.FieldsFunc(expression, func(r rune) bool { return unicode.IsSpace(r) || r == '(' || r == ')' })
	var ids []string
	hasOr := false
	for i := 0; i < len(tokens); i++ {
		switch strings.ToUpper(tokens[i]) {
		case "AND":
		case "OR":
			hasOr = true
		case "WITH":
			i++ // the exception identifier is not a licence
		default:
			id := tokens[i]
			if alias, ok := spdxAliases[id]; ok {
				id = alias
			}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return "no identifier"
	}
	var problems []string
	acceptable := 0
	for _, id := range ids {
		_, allowed := gpl3CompatibleLicenses[id]
		switch {
		case !allowed:
			problems = append(problems, id+" is not GPL-3.0 compatible")
		case !declared[id]:
			problems = append(problems, id+" is not listed in the manifest row")
		default:
			acceptable++
		}
	}
	if len(problems) == 0 || (hasOr && acceptable > 0) {
		return ""
	}
	return strings.Join(problems, "; ")
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

// linkedModules lists the modules compiled into the release binaries for one target, with the Go files
// each of their packages compiles. It reports failures with t.Errorf and returns nil, so goroutines
// can call it; the caller stops from the test goroutine.
func linkedModules(t *testing.T, target string) map[string]*linkedModule {
	t.Helper()
	goos, goarch, ok := strings.Cut(target, "/")
	if !ok {
		t.Errorf("bad release target %q", target)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	template := "{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Dir}}\t{{$.Dir}}\t{{join $.GoFiles \"|\"}}{{end}}{{end}}"
	args := append([]string{"list", "-deps", "-f", template}, linkedBinaryPackages...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = repoPath()
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0", "GOFLAGS=-mod=readonly")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("go list -deps %s for %s: %v\n%s\nrun go mod download if modules are missing", strings.Join(linkedBinaryPackages, " "), target, err, strings.TrimSpace(stderr.String()))
		return nil
	}
	modules := map[string]*linkedModule{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 {
			t.Errorf("go list printed an unexpected line for %s: %q", target, line)
			return nil
		}
		path, moduleDir, packageDir, goFiles := fields[0], fields[1], fields[2], fields[3]
		if strings.TrimSpace(moduleDir) == "" {
			t.Errorf("module %s has no directory in the module cache for %s; run go mod download", path, target)
			return nil
		}
		module := modules[path]
		if module == nil {
			module = &linkedModule{Dir: moduleDir, Files: map[string]bool{}}
			modules[path] = module
		}
		for _, name := range strings.Split(goFiles, "|") {
			if name != "" {
				module.Files[filepath.Join(packageDir, name)] = true
			}
		}
	}
	if len(modules) == 0 {
		t.Errorf("go list -deps for %s printed no modules", target)
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

// goFileLicenseHeader holds what the licence scan reads from the top of one compiled Go file.
type goFileLicenseHeader struct {
	Expressions  []string
	Incompatible bool
}

func scanGoFileLicenseHeader(path string) (goFileLicenseHeader, error) {
	var header goFileLicenseHeader
	file, err := os.Open(path)
	if err != nil {
		return header, err
	}
	defer file.Close()
	buf := make([]byte, licenseHeaderBytes)
	n, err := io.ReadFull(file, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return header, err
	}
	head := string(buf[:n])
	for _, match := range spdxHeaderLine.FindAllStringSubmatch(head, -1) {
		header.Expressions = append(header.Expressions, strings.TrimSpace(match[1]))
	}
	header.Incompatible = strings.Contains(strings.ToLower(head), mplIncompatibleMarker)
	return header, nil
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
	forbidden := map[string]string{
		"GPL-2.0-only.txt":  "GPL-2.0",
		"AGPL-3.0-only.txt": "AGPL-3.0",
		"SSPL-1.0.txt":      "SSPL-1.0",
		"EPL-2.0.txt":       "EPL-2.0 without a GPL secondary licence",
	}
	for name, id := range want {
		text := normalizedLicenseText([]byte(readRepoFile(t, "licences/texts/"+name)))
		if got := licenseGuess(text); got != id {
			t.Errorf("licences/texts/%s is classified %q, want %q", name, got, id)
		}
		if got := forbiddenLicense(text); got != forbidden[name] {
			t.Errorf("licences/texts/%s is flagged as %q, want %q", name, got, forbidden[name])
		}
	}
	// An EPL-2.0 notice that grants the GPL as Secondary License is not flagged.
	epl := normalizedLicenseText([]byte("Eclipse Public License - v 2.0 ... This Source Code may also be made available under the following Secondary Licenses when the conditions for such availability set forth in the Eclipse Public License, v. 2.0 are satisfied: GNU General Public License, Version 2.0, or any later versions"))
	if got := forbiddenLicense(epl); got != "" {
		t.Errorf("an EPL-2.0 text with a GPL secondary licence is flagged as %q", got)
	}
}

// Source headers can only narrow the manifest: each identifier must be allowed and declared.
func TestSPDXHeaderExpressionsAreCheckedAgainstTheManifestRow(t *testing.T) {
	t.Parallel()
	declared := map[string]bool{"MIT": true, "GPL-3.0-only": true}
	for expression, wantProblem := range map[string]bool{
		"MIT":                          false,
		"GPL-3.0":                      false, // deprecated identifier of GPL-3.0-only
		"MIT AND GPL-3.0-only":         false,
		"MIT OR Apache-2.0":            false, // one acceptable alternative is enough
		"Apache-2.0":                   true,  // allowed, but the row does not list it
		"MIT AND Apache-2.0":           true,
		"GPL-2.0-only":                 true, // not GPL-3.0 compatible
		"EPL-2.0 OR GPL-2.0-only":      true,
		"MIT WITH LLVM-exception":      false,
		"(MIT OR EPL-2.0) AND GPL-3.0": false,
		"":                             true,
	} {
		if got := spdxExpressionProblem(expression, declared) != ""; got != wantProblem {
			t.Errorf("spdxExpressionProblem(%q) problem = %v, want %v", expression, got, wantProblem)
		}
	}
}

// A GPL-3.0 combined work may only link modules whose licence is GPL-3.0 compatible. Every module
// linked into the release binaries (cmd/aurago, cmd/remote, cmd/config-merger) for a release target
// needs a reviewed row in licences/linked-go-modules.tsv; a licence outside the allowlist blocks the
// release (licences/AGENTS.md). The source files that the targets compile are checked too: their
// SPDX-License-Identifier headers must be allowed and listed in the row, and none may be an MPL-2.0
// file marked "Incompatible With Secondary Licenses".
func TestLinkedGoModulesHaveGPL3CompatibleLicenses(t *testing.T) {
	if testing.Short() {
		t.Skip("lists the linked modules of six release targets; skipped with -short")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("go is not on PATH, so the linked-module licence gate cannot run: %v", err)
	}
	manifest := readLinkedModuleManifest(t)
	notices := readRepoFile(t, "licences/THIRD_PARTY_NOTICES.md")

	perTarget := make([]map[string]*linkedModule, len(releaseTargets))
	var wg sync.WaitGroup
	for i, target := range releaseTargets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			perTarget[i] = linkedModules(t, target)
		}()
	}
	wg.Wait()
	linked := map[string]*linkedModule{}
	for _, modules := range perTarget {
		if modules == nil {
			t.FailNow()
		}
		for path, module := range modules {
			if known := linked[path]; known != nil {
				for file := range module.Files {
					known.Files[file] = true
				}
				continue
			}
			linked[path] = module
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
			guess, file := suggestedLicenseRow(linked[path].Dir)
			missing = append(missing, fmt.Sprintf("add: %s\t%s\t%s", path, guess, file))
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d linked module(s) have no reviewed row in %s; read each licence file, then add the line (module, SPDX, files, note):\n%s",
			len(missing), linkedModuleManifestPath, strings.Join(missing, "\n"))
	}

	for _, row := range manifest {
		module, ok := linked[row.Path]
		if !ok {
			continue
		}
		checkLinkedModuleRow(t, row, module, notices)
		checkLinkedModuleSources(t, row, module)
	}
}

// checkLinkedModuleRow checks the licence files a manifest row lists against the SPDX parts it declares.
func checkLinkedModuleRow(t *testing.T, row linkedModuleLicense, module *linkedModule, notices string) {
	t.Helper()
	parts := strings.Split(row.SPDX, " AND ")
	allowed := true
	for _, part := range parts {
		if _, known := gpl3CompatibleLicenses[part]; !known {
			allowed = false
			t.Errorf("%s: %s is not GPL-3.0 compatible; release blocked", row.Path, part)
		}
	}
	if !allowed {
		return
	}
	texts := map[string]string{}
	for _, file := range row.Files {
		// repo:licences/texts/X.txt names a verbatim SPDX text of this repository for a part whose licence
		// text the module does not ship.
		dir, name := module.Dir, file
		if rest, fromRepo := strings.CutPrefix(file, "repo:"); fromRepo {
			dir, name = repoPath("."), rest
			if !strings.HasPrefix(name, "licences/texts/") {
				t.Errorf("%s: %s must name a text below licences/texts/", row.Path, file)
				continue
			}
		}
		if !exactFileExists(dir, name) {
			t.Errorf("%s: licence file %s does not exist", row.Path, file)
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("%s: read %s: %v", row.Path, file, err)
			continue
		}
		text := normalizedLicenseText(raw)
		texts[file] = text
		if family := forbiddenLicense(text); family != "" {
			t.Errorf("%s: %s contains the %s text, which cannot be combined with GPL-3.0; release blocked", row.Path, file, family)
		}
		if strings.Contains(text, "eclipse distribution license") && !strings.Contains(text, "eclipse foundation") {
			t.Errorf("%s: %s is an Eclipse Distribution License without its Eclipse Foundation copyright holder", row.Path, file)
		}
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
		if part == "GPL-3.0-only" && !strings.Contains(notices, "`"+row.Path+"`") {
			t.Errorf("%s: GPL-3.0 module is not named in backticks in THIRD_PARTY_NOTICES.md", row.Path)
		}
	}
}

// checkLinkedModuleSources reads the licence headers of the Go files the release targets compile.
func checkLinkedModuleSources(t *testing.T, row linkedModuleLicense, module *linkedModule) {
	t.Helper()
	declared := map[string]bool{}
	for _, part := range strings.Split(row.SPDX, " AND ") {
		declared[part] = true
	}
	files := make([]string, 0, len(module.Files))
	for file := range module.Files {
		files = append(files, file)
	}
	sort.Strings(files)
	type finding struct {
		example string
		count   int
	}
	findings := map[string]*finding{}
	for _, file := range files {
		header, err := scanGoFileLicenseHeader(file)
		if err != nil {
			t.Errorf("%s: read %s: %v", row.Path, file, err)
			continue
		}
		relative, relErr := filepath.Rel(module.Dir, file)
		if relErr != nil {
			relative = file
		}
		relative = filepath.ToSlash(relative)
		var problems []string
		for _, expression := range header.Expressions {
			if problem := spdxExpressionProblem(expression, declared); problem != "" {
				problems = append(problems, fmt.Sprintf("SPDX-License-Identifier %q: %s", expression, problem))
			}
		}
		if header.Incompatible {
			problems = append(problems, `an MPL-2.0 file marked "Incompatible With Secondary Licenses" cannot be combined with GPL-3.0`)
		}
		for _, problem := range problems {
			if findings[problem] == nil {
				findings[problem] = &finding{example: relative}
			}
			findings[problem].count++
		}
	}
	var problems []string
	for problem := range findings {
		problems = append(problems, problem)
	}
	sort.Strings(problems)
	for _, problem := range problems {
		found := findings[problem]
		t.Errorf("%s: %s (%d compiled file(s), for example %s); fix the row in %s or block the module", row.Path, problem, found.count, found.example, linkedModuleManifestPath)
	}
}
