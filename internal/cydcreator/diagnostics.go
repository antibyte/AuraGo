package cydcreator

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Diagnostic is one compiler or linker message mapped to the project tree.
type Diagnostic struct {
	File      string `json:"file"` // project-relative, e.g. src/app/app_main.cpp; "" for linker errors
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Severity  string `json:"severity"` // error | warning | note
	Message   string `json:"message"`
	Generated bool   `json:"generated"` // IsGeneratedPath(File)
}

var (
	diagLinePattern = regexp.MustCompile(`^(.+?):(\d+):(?:(\d+):)? (fatal error|error|warning|note): (.*)$`)
	ansiEscape      = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")
	linkerMarkers   = []string{"undefined reference to ", "multiple definition of "}
)

// ParseDiagnostics extracts gcc/g++/ld diagnostics from a build log.
//
//   - "file:line[:col]: (fatal error|error|warning|note): msg" lines are kept;
//     "In file included from" and context lines are ignored.
//   - Linker "undefined reference to" / "multiple definition of" lines become
//     errors with File "" (one per symbol).
//   - Paths under workRoot (e.g. /work/ccb_x) become project-relative.
//     Warnings and notes outside the project (toolchain, framework, .pio/
//     library sources) are dropped; errors are kept wherever they occur.
//   - ANSI colors and CR are stripped; indented lines (source excerpts, caret
//     lines, "from" continuations) are skipped because gcc and ld start every
//     diagnostic in column 0; duplicates are removed; at most 200 entries are
//     returned (notes are dropped first, then warnings).
//   - File is clipped with clipReason because a #line directive lets a source
//     file name any path; Generated is decided from the full path first.
func ParseDiagnostics(log string, workRoot string) []Diagnostic {
	root := strings.TrimSuffix(strings.ReplaceAll(workRoot, "\\", "/"), "/")
	var out []Diagnostic
	seen := map[Diagnostic]bool{}
	var collected [3]int // per severity rank; no severity can ever keep more than maxDiagnostics
	for _, raw := range strings.Split(log, "\n") {
		line := strings.TrimRight(ansiEscape.ReplaceAllString(raw, ""), "\r")
		if line == "" || line[0] == ' ' || line[0] == '\t' || strings.HasPrefix(line, "In file included from") {
			continue
		}
		d, ok := parseLinkerLine(line)
		if !ok {
			d, ok = parseCompilerLine(line, root)
		}
		if !ok {
			continue
		}
		rank := severityRank(d.Severity)
		if collected[rank] >= maxDiagnostics || seen[d] {
			continue
		}
		seen[d] = true
		collected[rank]++
		out = append(out, d)
	}
	return capDiagnostics(out)
}

func parseLinkerLine(line string) (Diagnostic, bool) {
	for _, marker := range linkerMarkers {
		idx := strings.Index(line, marker)
		if idx < 0 {
			continue
		}
		msg := line[idx:]
		if semi := strings.Index(msg, "; "); semi > 0 {
			msg = msg[:semi] // "...; <object>: first defined here"
		}
		return Diagnostic{Severity: SeverityError, Message: clipReason(msg)}, true
	}
	return Diagnostic{}, false
}

func parseCompilerLine(line, root string) (Diagnostic, bool) {
	m := diagLinePattern.FindStringSubmatch(line)
	if m == nil {
		return Diagnostic{}, false
	}
	lineNo, _ := strconv.Atoi(m[2])
	col := 0
	if m[3] != "" {
		col, _ = strconv.Atoi(m[3])
	}
	sev := m[4]
	if sev == "fatal error" {
		sev = SeverityError
	}
	file, inProject := projectRelative(m[1], root)
	if !inProject && sev != SeverityError {
		return Diagnostic{}, false
	}
	return Diagnostic{File: clipReason(file), Line: lineNo, Column: col, Severity: sev,
		Message: clipReason(strings.TrimSpace(m[5])), Generated: IsGeneratedPath(file)}, true
}

// projectRelative maps a compiler path to the project tree. inProject is false
// for absolute paths outside workRoot and for .pio/ (library) sources.
func projectRelative(file, root string) (string, bool) {
	file = strings.ReplaceAll(strings.TrimSpace(file), "\\", "/")
	if root != "" && strings.HasPrefix(file, root+"/") {
		file = strings.TrimPrefix(file, root+"/")
	}
	if isAbsPath(file) {
		return path.Clean(file), false
	}
	file = strings.TrimPrefix(path.Clean(file), "./")
	return file, !strings.HasPrefix(file, ".pio/") && !strings.HasPrefix(file, "../")
}

func isAbsPath(p string) bool {
	return strings.HasPrefix(p, "/") || (len(p) > 2 && p[1] == ':' && p[2] == '/')
}

// severityRank orders severities by importance: error 0, warning 1, note 2.
func severityRank(sev string) int {
	switch sev {
	case SeverityError:
		return 0
	case SeverityWarning:
		return 1
	}
	return 2
}

// capDiagnostics keeps at most maxDiagnostics entries in log order. The budget
// goes to errors first, then warnings, then notes; inside a severity the
// earliest entries win (later ones are dropped from the end). It counts first
// and filters once, so it is linear however large the input is.
func capDiagnostics(list []Diagnostic) []Diagnostic {
	if len(list) <= maxDiagnostics {
		return list
	}
	var total, keep, kept [3]int
	for _, d := range list {
		total[severityRank(d.Severity)]++
	}
	left := maxDiagnostics
	for rank := range keep {
		keep[rank] = min(total[rank], left)
		left -= keep[rank]
	}
	out := make([]Diagnostic, 0, maxDiagnostics)
	for _, d := range list {
		rank := severityRank(d.Severity)
		if kept[rank] < keep[rank] {
			kept[rank]++
			out = append(out, d)
		}
	}
	return out
}

// windowsReserved reports a Windows device name (CON, PRN, AUX, NUL, COM0-9,
// LPT0-9). Windows resolves it from the part of a path segment before the
// first dot and ignores case, so "con.h" or "Aux" would open a device, not a
// file, and a project checked out or built on Windows could hang or lose data.
func windowsReserved(seg string) bool {
	base := seg
	if dot := strings.IndexByte(seg, '.'); dot >= 0 {
		base = seg[:dot]
	}
	base = strings.ToLower(base)
	switch base {
	case "con", "prn", "aux", "nul":
		return true
	}
	return len(base) == 4 && (strings.HasPrefix(base, "com") || strings.HasPrefix(base, "lpt")) &&
		base[3] >= '0' && base[3] <= '9'
}

func cleanRel(rel string) string {
	rel = strings.ReplaceAll(rel, "\\", "/")
	if rel == "" {
		return ""
	}
	return strings.TrimPrefix(path.Clean(rel), "./")
}

// IsGeneratedPath reports paths produced by the generator or copied from the
// template: src/generated/**, lib/cyd_template/**, include/**, platformio.ini,
// partitions.csv. Errors there are generator bugs (CYD_GENERATOR_ERROR).
func IsGeneratedPath(rel string) bool {
	rel = cleanRel(rel)
	switch {
	case rel == "platformio.ini", rel == "partitions.csv",
		rel == "src/generated", rel == "lib/cyd_template", rel == "include":
		return true
	case strings.HasPrefix(rel, "src/generated/"), strings.HasPrefix(rel, "lib/cyd_template/"), strings.HasPrefix(rel, "include/"):
		return true
	}
	return false
}

// editableSegment is one path segment of 1..64 bytes: it starts with a letter,
// digit or underscore and does not end with a dot, because Windows drops
// trailing dots ("net." is "net") and the two spellings would alias one folder.
var editableSegment = regexp.MustCompile(`^[A-Za-z0-9_](?:[A-Za-z0-9_.-]{0,62}[A-Za-z0-9_-])?$`)

// IsEditablePath reports app source paths the user and the agent may write:
// src/app/** (matched case-sensitively) with a .cpp, .h, .hpp or .c
// extension, at most four directory levels below src/app, ASCII segments of
// [A-Za-z0-9_.-] that start with a letter, digit or underscore, do not end
// with a dot and are no Windows device name (CON, PRN, AUX, NUL, COM0-9,
// LPT0-9, with or without an extension), no "..", no backslashes, no ":", at
// most 160 bytes.
func IsEditablePath(rel string) bool {
	if rel == "" || len(rel) > 160 || strings.ContainsAny(rel, "\\\x00") || !strings.HasPrefix(rel, "src/app/") {
		return false
	}
	segs := strings.Split(rel, "/")
	if len(segs) < 3 || len(segs) > 7 {
		return false
	}
	for _, s := range segs {
		if !editableSegment.MatchString(s) || windowsReserved(s) {
			return false
		}
	}
	switch path.Ext(segs[len(segs)-1]) {
	case ".cpp", ".h", ".hpp", ".c":
		return true
	}
	return false
}
