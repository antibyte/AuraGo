package videostudio

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// maxWorkDirUnits is the longest working directory, in UTF-16 code units, that Windows CreateProcess accepts for a
// child process: MAX_PATH (260) minus the separator it appends and the terminating NUL. A longer directory fails with
// ERROR_DIRECTORY ("The directory name is invalid"). A \\?\ prefix counts toward the same limit, and neither
// LongPathsEnabled nor a longPathAware manifest lifts it.
const maxWorkDirUnits = 258

// ffmpegWorkDir picks the working directory of a staged FFmpeg process and tells whether the graph may name the staged
// files bare:
//   - A dir of at most maxWorkDirUnits UTF-16 units is the working directory itself (bare).
//   - A longer one cannot be a working directory, so FFmpeg runs in the volume root of dir and the graph names every
//     staged file by absolute path (absoluteStagedGraph).
//
// The rule is deliberately the same on every platform (on Linux the root is "/"), so the long-path form runs, and is
// tested, everywhere and with every FFmpeg version. stageVideoGraph (graph form) and runFFmpegIn (working directory)
// both apply it to the same path, so they always agree.
func ffmpegWorkDir(dir string) (runDir string, bare bool) {
	if utf16Units(dir) <= maxWorkDirUnits {
		return dir, true
	}
	return filepath.VolumeName(dir) + string(filepath.Separator), false
}

// utf16Units is the length of s in UTF-16 code units, the unit of the Windows path limits. Invalid UTF-8 counts one
// unit per byte, as U+FFFD.
func utf16Units(s string) int {
	units := 0
	for _, r := range s {
		units += utf16.RuneLen(r)
	}
	return units
}

// escapeChars puts a backslash in front of every byte of s that is in special (ASCII characters only).
func escapeChars(s, special string) string {
	var out strings.Builder
	out.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(special, s[i]) >= 0 {
			out.WriteByte('\\')
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

// filterPath writes a host path as the value of a filter option inside a filter-graph script. FFmpeg unescapes it twice:
//   - the filter's option string, where ':' ends a value (escaped: backslash, ':' and a quote);
//   - the filtergraph description, where one of "[],;" ends a filter (escaped: backslash, quote, '[', ']', ',' and ';').
//
// Slashes are forward slashes, which Windows accepts. A raw "C:/" or a singly escaped "C\:/" is not read as one value
// and quoting fails for a path with an apostrophe, so both levels escape. Control characters are refused: no staged
// directory has any, and a line break would split the script.
func filterPath(p string) (string, error) {
	for i := 0; i < len(p); i++ {
		if p[i] < 0x20 || p[i] == 0x7f {
			return "", fmt.Errorf("staged file path contains a control character")
		}
	}
	return escapeChars(escapeChars(filepath.ToSlash(p), `\:'`), `\'[],;`), nil
}

// absoluteStagedGraph rewrites the bare references of graph to the staged command and LUT files, sendcmd=f=<name> and
// lut3d=file=<name>, to the escaped absolute path of the file in workDir (filterPath). Other text is untouched. A bare
// staged file name that is left over, one that no entry of files covers, is an error: FFmpeg would look for it in a
// working directory that is not workDir.
func absoluteStagedGraph(graph, workDir string, files graphFiles) (string, error) {
	rewrite := make([]string, 0, 2*(len(files.commands)+len(files.luts)))
	drop := make([]string, 0, cap(rewrite))
	add := func(option, name string) error {
		path, err := filterPath(filepath.Join(workDir, name))
		if err != nil {
			return err
		}
		rewrite = append(rewrite, option+name, option+path)
		drop = append(drop, option+name, "")
		return nil
	}
	// The staged names end in .txt or .cube, so none is a prefix of another and the replacer's order cannot matter.
	for _, file := range files.commands {
		if err := add("sendcmd=f=", file.name); err != nil {
			return "", err
		}
	}
	for _, file := range files.luts {
		if err := add("lut3d=file=", file.name); err != nil {
			return "", err
		}
	}
	// A reference is left over when it survives the removal of every covered one (checked on the removal, so a
	// directory name that happens to contain "=vs_cmd_" cannot be mistaken for one).
	if rest := strings.NewReplacer(drop...).Replace(graph); strings.Contains(rest, "=vs_cmd_") || strings.Contains(rest, "=vs_lut_") {
		return "", fmt.Errorf("filter graph names a staged file that was not staged")
	}
	return strings.NewReplacer(rewrite...).Replace(graph), nil
}
