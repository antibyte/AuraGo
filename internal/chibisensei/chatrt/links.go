package chatrt

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"aurago/internal/llamacpp"
	"aurago/internal/onnxrt"
)

const (
	maxArchiveLinks   = 4096
	maxArchiveEntries = 200000
	maxLinkNameBytes  = 1024
	maxLinkChain      = 8
	linkMarkerName    = ".links.json"
)

// linkEntry is one symbolic-link entry of the archive. Name and Target are untrusted slash paths.
type linkEntry struct{ Name, Target string }

// plannedLink is one validated link: Name is the cleaned archive path of the link, Final the cleaned archive path
// of the regular file it resolves to.
type plannedLink struct{ Name, Final string }

type linksMarker struct {
	Asset  string `json:"asset"`
	SHA256 string `json:"sha256"`
	Links  int    `json:"links"`
}

// ensureArchiveLinks recreates, in its own directory, the link entries of a verified tar.gz runtime archive that
// the shared extractor skips, so the dynamic loader finds the library names it asks for (for example
// libllama-common.so.0). It returns "" when the platform or the archive needs none.
func ensureArchiveLinks(ctx context.Context, cacheDir string, spec llamacpp.AssetSpec, runtimeRoot string) (string, error) {
	return ensureArchiveLinksFor(ctx, runtime.GOOS, cacheDir, spec, runtimeRoot)
}

func ensureArchiveLinksFor(ctx context.Context, goos, cacheDir string, spec llamacpp.AssetSpec, runtimeRoot string) (string, error) {
	if spec.Kind != onnxrt.AssetTarGzip || goos == "windows" {
		return "", nil
	}
	if !plainName(spec.ID) {
		return "", fmt.Errorf("archive links of %q: invalid asset id", spec.ID)
	}
	linksDir := filepath.Join(cacheDir, "links")
	linkRoot := filepath.Join(linksDir, spec.ID)
	if linkRootIsCurrent(linkRoot, spec, runtimeRoot) {
		return linkRoot, nil
	}
	entries, err := readArchiveLinks(ctx, onnxrt.ArchivePath(cacheDir, spec))
	if err != nil {
		return "", fmt.Errorf("archive links of %s: %w", spec.ID, err)
	}
	plan := planArchiveLinks(entries, func(rel string) bool { return isRegularFile(filepath.Join(runtimeRoot, filepath.FromSlash(rel))) })
	if len(plan) == 0 {
		return "", nil
	}
	if err := os.MkdirAll(linksDir, 0o750); err != nil {
		return "", fmt.Errorf("archive links of %s: create link directory: %w", spec.ID, err)
	}
	tmp, err := os.MkdirTemp(linksDir, spec.ID+".tmp-")
	if err != nil {
		return "", fmt.Errorf("archive links of %s: create temporary link directory: %w", spec.ID, err)
	}
	cleanup := func() { _ = onnxrt.SafeRemoveAll(linksDir, tmp) }
	count, err := writeArchiveLinks(tmp, runtimeRoot, plan)
	if err == nil {
		err = writeLinksMarker(tmp, linksMarker{Asset: spec.ID, SHA256: spec.SHA256, Links: count})
	}
	if err != nil {
		cleanup()
		return "", fmt.Errorf("archive links of %s: %w", spec.ID, err)
	}
	if _, statErr := os.Lstat(linkRoot); statErr == nil {
		if err := onnxrt.SafeRemoveAll(linksDir, linkRoot); err != nil {
			cleanup()
			return "", fmt.Errorf("archive links of %s: replace old link directory: %w", spec.ID, err)
		}
	}
	if err := os.Rename(tmp, linkRoot); err != nil {
		cleanup()
		return "", fmt.Errorf("archive links of %s: publish link directory: %w", spec.ID, err)
	}
	return linkRoot, nil
}

// plainName reports whether s is one path element that cannot leave its directory.
func plainName(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, "/\\\x00")
}

// cleanArchivePath normalizes an untrusted archive slash path. It rejects absolute paths, NUL, backslashes and
// every path that leaves the archive root.
func cleanArchivePath(p string) (string, bool) {
	if p == "" || strings.ContainsAny(p, "\\\x00") || strings.HasPrefix(p, "/") {
		return "", false
	}
	for strings.HasPrefix(p, "./") {
		p = p[2:]
	}
	c := path.Clean(p)
	if c != p || c == "." || c == ".." || strings.HasPrefix(c, "../") {
		return "", false
	}
	return c, true
}

// resolveLinkTarget joins a link's target with the directory of the link and keeps it inside the archive.
func resolveLinkTarget(name, target string) (string, bool) {
	if target == "" || strings.ContainsAny(target, "\\\x00") || strings.HasPrefix(target, "/") {
		return "", false
	}
	return cleanArchivePath(path.Join(path.Dir(name), target))
}

// planArchiveLinks is the pure core: which links are created and where they finally point. exists reports whether
// an archive path is a regular file of the extracted runtime. Unsafe or unresolvable entries are skipped.
func planArchiveLinks(entries []linkEntry, exists func(rel string) bool) []plannedLink {
	type node struct{ name, target string }
	nodes := make(map[string]node, len(entries))
	for _, e := range entries {
		name, ok := cleanArchivePath(e.Name)
		if !ok || !plainName(path.Base(name)) || strings.HasSuffix(e.Name, "/") {
			continue
		}
		if _, ok := resolveLinkTarget(name, e.Target); !ok {
			continue
		}
		nodes[name] = node{name, e.Target}
	}
	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	var plan []plannedLink
	for _, name := range names {
		cur, _ := resolveLinkTarget(name, nodes[name].target)
		final := ""
		for step := 0; step <= maxLinkChain; step++ {
			next, isLink := nodes[cur]
			if !isLink {
				if exists(cur) {
					final = cur
				}
				break
			}
			resolved, ok := resolveLinkTarget(cur, next.target)
			if !ok {
				break
			}
			cur = resolved
		}
		if final != "" {
			plan = append(plan, plannedLink{Name: name, Final: final})
		}
	}
	return plan
}

// readArchiveLinks collects the symbolic-link entries of a tar.gz archive within fixed bounds.
func readArchiveLinks(ctx context.Context, archivePath string) ([]linkEntry, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var entries []linkEntry
	for seen := 0; ; seen++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen >= maxArchiveEntries {
			return nil, errors.New("archive has too many entries")
		}
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return entries, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read archive entry: %w", err)
		}
		if hdr.Typeflag != tar.TypeSymlink {
			continue
		}
		if len(hdr.Name) > maxLinkNameBytes || len(hdr.Linkname) > maxLinkNameBytes {
			continue
		}
		if len(entries) >= maxArchiveLinks {
			return nil, errors.New("archive has too many link entries")
		}
		entries = append(entries, linkEntry{Name: hdr.Name, Target: hdr.Linkname})
	}
}

// isRegularFile is true for a regular file and never follows a link.
func isRegularFile(p string) bool {
	info, err := os.Lstat(p)
	return err == nil && info.Mode().IsRegular()
}

// writeArchiveLinks creates the planned links below root, each pointing at the absolute path of its final file.
func writeArchiveLinks(root, runtimeRoot string, plan []plannedLink) (int, error) {
	absRuntime, err := filepath.Abs(runtimeRoot)
	if err != nil {
		return 0, fmt.Errorf("resolve runtime root: %w", err)
	}
	count := 0
	for _, l := range plan {
		dest := filepath.Join(root, filepath.FromSlash(l.Name))
		if rel, err := filepath.Rel(root, dest); err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		target := filepath.Join(absRuntime, filepath.FromSlash(l.Final))
		if rel, err := filepath.Rel(absRuntime, target); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			return count, fmt.Errorf("create link parent: %w", err)
		}
		if err := os.Symlink(target, dest); err != nil {
			return count, fmt.Errorf("create link: %w", err)
		}
		count++
	}
	if count == 0 {
		return 0, errors.New("no link could be created")
	}
	return count, nil
}

func writeLinksMarker(root string, m linksMarker) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode marker: %w", err)
	}
	if err := os.WriteFile(filepath.Join(root, linkMarkerName), data, 0o640); err != nil {
		return fmt.Errorf("write marker: %w", err)
	}
	return nil
}

// linkRootIsCurrent is true when the marker matches the asset and every recorded link still resolves to a
// regular file below runtimeRoot.
func linkRootIsCurrent(linkRoot string, spec llamacpp.AssetSpec, runtimeRoot string) bool {
	data, err := os.ReadFile(filepath.Join(linkRoot, linkMarkerName))
	if err != nil {
		return false
	}
	var m linksMarker
	if json.Unmarshal(data, &m) != nil || m.Asset != spec.ID || m.SHA256 != spec.SHA256 || m.Links <= 0 {
		return false
	}
	absRuntime, err := filepath.Abs(runtimeRoot)
	if err != nil {
		return false
	}
	found := 0
	ok := true
	_ = filepath.WalkDir(linkRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			ok = false
			return fs.SkipAll
		}
		if d.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		found++
		dest, err := os.Readlink(p)
		if err != nil {
			ok = false
			return fs.SkipAll
		}
		if rel, err := filepath.Rel(absRuntime, dest); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			ok = false
			return fs.SkipAll
		}
		if info, err := os.Stat(p); err != nil || !info.Mode().IsRegular() {
			ok = false
			return fs.SkipAll
		}
		return nil
	})
	return ok && found == m.Links
}
