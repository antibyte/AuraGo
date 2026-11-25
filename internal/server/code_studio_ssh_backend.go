package server

import (
	"errors"
	pathpkg "path"
	"sort"
	"strings"
	"time"
)

var errCodeStudioOutsideRoot = errors.New("path is outside the workspace")

// normalizeCodeStudioRemoteRoot accepts absolute POSIX paths and ~ / ~/x (home = sftp Getwd).
func normalizeCodeStudioRemoteRoot(raw, home string) (string, error) {
	p := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if strings.ContainsRune(p, 0) || p == "" {
		return "", errors.New("remote path is required")
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home == "" {
			return "", errors.New("remote home directory is unknown")
		}
		p = home + strings.TrimPrefix(p, "~")
	}
	if !strings.HasPrefix(p, "/") {
		return "", errors.New("remote path must be absolute")
	}
	return pathpkg.Clean(p), nil
}

func codeStudioRemoteInside(root, p string) bool {
	root, p = pathpkg.Clean(root), pathpkg.Clean(p)
	if root == "/" {
		return strings.HasPrefix(p, "/")
	}
	return p == root || strings.HasPrefix(p, root+"/")
}

// codeStudioRemotePath maps /workspace/<rel> to <root>/<rel>.
func codeStudioRemotePath(root, vpath string) (string, error) {
	v, err := sanitizeCodeStudioPath(vpath)
	if err != nil {
		return "", err
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(v, codeStudioWorkspaceRoot), "/")
	if rel == "" {
		return pathpkg.Clean(root), nil
	}
	return pathpkg.Join(root, rel), nil
}

// codeStudioVirtualPath maps <root>/<rel> back to /workspace/<rel>.
func codeStudioVirtualPath(root, remote string) (string, error) {
	root, remote = pathpkg.Clean(root), pathpkg.Clean(remote)
	if !codeStudioRemoteInside(root, remote) {
		return "", errCodeStudioOutsideRoot
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(remote, root), "/")
	if rel == "" {
		return codeStudioWorkspaceRoot, nil
	}
	return codeStudioWorkspaceRoot + "/" + rel, nil
}

func sortCodeStudioFileEntries(entries []codeStudioFileEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "directory"
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

// codeStudioSSHCommandLine wraps a POSIX script for the user's login shell. A fish
// login shell can alter "\\" inside the single quotes; documented limitation.
func codeStudioSSHCommandLine(script string) string { return "sh -c " + shellQuote(script) }

const codeStudioCapabilitiesScript = `printf 'os=%s\n' "$(uname -s 2>/dev/null)"
printf 'arch=%s\n' "$(uname -m 2>/dev/null)"
printf 'shell=%s\n' "${SHELL:-}"
for t in git grep gofmt gopls pyright-langserver typescript-language-server clangd python3; do
  if command -v "$t" >/dev/null 2>&1; then printf 'has=%s\n' "$t"; fi
done`

func parseCodeStudioCapabilities(output string, now time.Time) codeStudioCapabilities {
	caps := codeStudioCapabilities{Formatters: map[string]bool{"gofmt": false}, ProbedAt: now}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "os":
			caps.OS = value
		case "arch":
			caps.Arch = value
		case "shell":
			caps.Shell = value
		case "has":
			switch value {
			case "git":
				caps.Git = true
			case "grep":
				caps.Grep = true
			case "python3":
				caps.Python3 = true
			case "gofmt":
				caps.Formatters["gofmt"] = true
			case "gopls":
				caps.LSP.Go = true
			case "pyright-langserver":
				caps.LSP.Python = true
			case "typescript-language-server":
				caps.LSP.TypeScript = true
			case "clangd":
				caps.LSP.CPP = true
			}
		}
	}
	return caps
}

// codeStudioSSHGrepArgs: --null (not -Z: BSD grep reads -Z as decompress), -H for single files.
func codeStudioSSHGrepArgs(opts codeStudioSearchOptions, dir string) []string {
	args := []string{"grep", "-r", "-n", "-H", "-I", "-s", "--null"}
	if opts.Regex {
		args = append(args, "-E")
	} else {
		args = append(args, "-F")
	}
	if !opts.CaseSensitive {
		args = append(args, "-i")
	}
	if opts.WholeWord {
		args = append(args, "-w")
	}
	if inc := sanitizeCodeStudioGlob(opts.Include); inc != "" {
		args = append(args, "--include="+inc)
	}
	if exc := sanitizeCodeStudioExclude(opts.Exclude); exc != "" {
		args = append(args, "--exclude="+exc, "--exclude-dir="+exc)
	}
	return append(args, "-m", "201", "-e", opts.Query, "--", dir)
}

// rewriteCodeStudioGrepPaths maps "remote\0rest\n" records to virtual paths; records
// outside root or incomplete are dropped.
func rewriteCodeStudioGrepPaths(root, output string) string {
	var b strings.Builder
	for output != "" {
		name, rest, ok := strings.Cut(output, "\x00")
		if !ok {
			break
		}
		line, tail, ended := strings.Cut(rest, "\n")
		output = tail
		if !ended {
			break
		}
		if v, err := codeStudioVirtualPath(root, name); err == nil {
			b.WriteString(v + "\x00" + line + "\n")
		}
	}
	return b.String()
}
