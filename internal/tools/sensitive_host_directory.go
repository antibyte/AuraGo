package tools

import (
	"strings"

	"aurago/internal/dockerutil"
)

// sensitiveHostDataTrees are system program, runtime and operating-system
// trees AuraGo never stores managed data in, in addition to the Docker bind
// denylist. All entries are lower case: IsSensitiveHostDirectory folds case
// before comparing because Windows and macOS file systems are case-insensitive.
//
// The macOS entries are /system, /library, /applications and /private (which
// holds /private/etc and /private/var). /usr is shared with Linux. /volumes
// (refused for the directory itself only) and /users/<name>/library are
// handled separately in IsSensitiveHostDirectory.
var sensitiveHostDataTrees = []string{
	"/bin", "/sbin", "/usr", "/lib", "/lib32", "/lib64", "/run", "/var/run",
	"/system", "/library", "/applications", "/private",
}

// IsSensitiveHostDirectory reports whether a directory must be refused as the
// storage location of AuraGo-managed data files (such as the Local Wikipedia
// storage directory).
//
// dir must be an absolute path (POSIX, Windows drive, UNC or Windows device
// form); a relative path is never reported as sensitive, so callers check
// filepath.IsAbs (or the platform equivalent) themselves. Matching is purely
// lexical and case-insensitive: "..", repeated and trailing separators and the
// trailing dots and spaces Windows ignores in path components are resolved
// first; symbolic links, junctions and 8.3 short names are not resolved
// (callers that create the directory check its resolved form as well).
//
// The check reuses the Docker bind denylist (sensitiveDockerHostPaths, drive
// roots and Windows system folders) and adds:
//   - Windows namespace prefixes: \\?\ and \\.\ are stripped before checking and
//     \\?\UNC\host\share is handled as \\host\share. Device paths that do not
//     name a drive or a UNC share (\\?\Volume{...}, \\.\PhysicalDrive0) cannot
//     be classified and are refused. Administrative shares (\\host\C$,
//     \\host\ADMIN$, \\host\PRINT$, \\host\IPC$) are refused; other network
//     shares are allowed.
//   - /mnt, where Linux hosts mount data disks: directories below it are
//     allowed and /mnt itself is refused, except WSL's Windows drives
//     (/mnt/<letter>/... is checked like <LETTER>:\..., so the drive root and
//     the Windows system folders are refused) and /mnt/wsl*.
//   - the system program trees in sensitiveHostDataTrees, /Volumes itself
//     (but not /Volumes/<disk>/...) and /Users/<name>/Library.
func IsSensitiveHostDirectory(dir string) bool {
	p := strings.ToLower(dockerutil.NormalizeHostPathForBind(dir))
	if strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///") {
		p = "//" + collapseHostSlashes(p[2:])
	} else {
		p = collapseHostSlashes(p)
	}
	p, unclassifiable := stripWindowsNamespacePrefix(p)
	if unclassifiable {
		return true
	}
	// Both spellings are checked: the path as written (POSIX file systems
	// keep trailing dots, so "/var/run/.../.." is /var/run there) and as
	// Windows reads it ("c:/windows./wiki" is c:/windows/wiki).
	return isSensitiveNormalizedHostPath(p) || isSensitiveNormalizedHostPath(trimWindowsComponentSuffixes(p))
}

// isSensitiveNormalizedHostPath applies the checks of IsSensitiveHostDirectory
// to a lower-cased, slash-normalized path without namespace prefix.
func isSensitiveNormalizedHostPath(p string) bool {
	if isWindowsAdminShare(p) {
		return true
	}
	cleaned := cleanDockerHostPath(p)
	if dockerHostPathIsSensitiveLocation(cleaned) {
		return true
	}
	if rest, ok := strings.CutPrefix(cleaned, "/mnt/"); ok {
		return wslMountIsSensitive(rest)
	}
	if isSensitiveDockerHostPath(cleaned) {
		return true
	}
	for _, tree := range sensitiveHostDataTrees {
		if dockerPathEqualOrWithin(cleaned, tree) {
			return true
		}
	}
	return cleaned == "/volumes" || isMacUserLibrary(cleaned)
}

// collapseHostSlashes replaces runs of slashes with a single slash.
func collapseHostSlashes(p string) string {
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return p
}

// stripWindowsNamespacePrefix removes the //?/ and //./ device prefixes from a
// lower-cased, slash-normalized path. //?/unc/host/share becomes //host/share
// and //?/c:/dir becomes c:/dir. The second result is true for a device path
// that names neither a drive nor a UNC share.
func stripWindowsNamespacePrefix(p string) (string, bool) {
	for _, prefix := range []string{"//?/", "//./"} {
		rest, ok := strings.CutPrefix(p, prefix)
		if !ok {
			continue
		}
		if unc, ok := strings.CutPrefix(rest, "unc/"); ok {
			return "//" + unc, false
		}
		if hasDriveLetter(rest) {
			return rest, false
		}
		return p, true
	}
	return p, false
}

// hasDriveLetter reports whether p starts with "<letter>:" followed by the end
// of the path or a slash.
func hasDriveLetter(p string) bool {
	return len(p) >= 2 && p[0] >= 'a' && p[0] <= 'z' && p[1] == ':' && (len(p) == 2 || p[2] == '/')
}

// trimWindowsComponentSuffixes removes the trailing dots and spaces Windows
// ignores in every path component ("c:/windows./wiki" is c:/windows/wiki). A
// component made only of dots and spaces (other than "." and "..") becomes
// ".", which the later cleaning drops. Empty components (the leading "//" of a
// UNC path) are kept. Trimming alone can make a path less sensitive (a POSIX
// directory named "..." is real, so "/var/run/.../.." is /var/run, but trimmed
// it reads /var), which is why IsSensitiveHostDirectory checks the trimmed
// and the untrimmed spelling and refuses the path when either is sensitive.
func trimWindowsComponentSuffixes(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			continue
		}
		trimmed := strings.TrimRight(part, ". ")
		if trimmed == "" {
			trimmed = "."
		}
		parts[i] = trimmed
	}
	return strings.Join(parts, "/")
}

// isWindowsAdminShare reports a UNC path (//host/share/...) whose share is a
// drive's administrative share (C$), ADMIN$ (the Windows folder), PRINT$ (the
// printer driver folder below System32) or IPC$. p is lower-cased and
// slash-normalized.
func isWindowsAdminShare(p string) bool {
	if !strings.HasPrefix(p, "//") || strings.HasPrefix(p, "///") {
		return false
	}
	_, afterHost, ok := strings.Cut(p[2:], "/")
	if !ok {
		return false
	}
	share, _, _ := strings.Cut(afterHost, "/")
	switch share {
	case "admin$", "print$", "ipc$":
		return true
	}
	return len(share) == 2 && share[0] >= 'a' && share[0] <= 'z' && share[1] == '$'
}

// wslMountIsSensitive decides a cleaned lower-case path below /mnt. rest is the
// part after "/mnt/".
func wslMountIsSensitive(rest string) bool {
	first, tail, _ := strings.Cut(rest, "/")
	if strings.HasPrefix(first, "wsl") {
		return true
	}
	if len(first) == 1 && first[0] >= 'a' && first[0] <= 'z' {
		windows := cleanDockerHostPath(first + ":/" + tail)
		return dockerHostPathIsSensitiveLocation(windows) || isSensitiveWindowsHostPath(windows)
	}
	return false
}

// isMacUserLibrary reports /users/<name>/library and everything below it
// (cleaned, lower-case path).
func isMacUserLibrary(cleaned string) bool {
	rest, ok := strings.CutPrefix(cleaned, "/users/")
	if !ok {
		return false
	}
	_, tail, ok := strings.Cut(rest, "/")
	if !ok {
		return false
	}
	first, _, _ := strings.Cut(tail, "/")
	return first == "library"
}
