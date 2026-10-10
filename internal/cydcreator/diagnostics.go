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
//   - ANSI colors and CR are stripped; duplicates are removed; at most 200
//     entries are returned (notes are dropped first, then warnings).
func ParseDiagnostics(log string, workRoot string) []Diagnostic {
	root := strings.TrimSuffix(strings.ReplaceAll(workRoot, "\\", "/"), "/")
	var out []Diagnostic
	seen := map[Diagnostic]bool{}
	for _, raw := range strings.Split(log, "\n") {
		line := strings.TrimRight(ansiEscape.ReplaceAllString(raw, ""), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "In file included from") || strings.HasPrefix(trimmed, "from ") {
			continue
		}
		d, ok := parseLinkerLine(line)
		if !ok {
			d, ok = parseCompilerLine(line, root)
		}
		if !ok || seen[d] {
			continue
		}
		seen[d] = true
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
	return Diagnostic{File: file, Line: lineNo, Column: col, Severity: sev,
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

// capDiagnostics keeps at most maxDiagnostics entries in log order, dropping
// notes from the end first, then warnings, then trailing errors.
func capDiagnostics(list []Diagnostic) []Diagnostic {
	for _, drop := range []string{SeverityNote, SeverityWarning, SeverityError} {
		for i := len(list) - 1; i >= 0 && len(list) > maxDiagnostics; i-- {
			if list[i].Severity == drop {
				list = append(list[:i], list[i+1:]...)
			}
		}
	}
	return list
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

var editableSegment = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,63}$`)

// IsEditablePath reports app source paths the user and the agent may write:
// src/app/** with a .cpp, .h, .hpp or .c extension, at most four directory
// levels below src/app, segments of [A-Za-z0-9_.-] that do not start with a
// dot, no "..", no backslashes, at most 160 bytes.
func IsEditablePath(rel string) bool {
	if rel == "" || len(rel) > 160 || strings.ContainsAny(rel, "\\\x00") || !strings.HasPrefix(rel, "src/app/") {
		return false
	}
	segs := strings.Split(rel, "/")
	if len(segs) < 3 || len(segs) > 7 {
		return false
	}
	for _, s := range segs {
		if !editableSegment.MatchString(s) {
			return false
		}
	}
	switch path.Ext(segs[len(segs)-1]) {
	case ".cpp", ".h", ".hpp", ".c":
		return true
	}
	return false
}
