package cydcreator

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// importInvalid is the CYD_IMPORT_INVALID error with details.reason:
// not_zip, too_large, too_many_entries, unsafe_path, layout_missing,
// file_type, layout, board, app_files or asset.
func importInvalid(reason, message string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	details["reason"] = reason
	return NewError(CodeImportInvalid, message, details)
}

// importTooLarge refuses an archive whose kept entries unpack to more than
// MaxImportBytes.
func importTooLarge() error {
	return importInvalid("too_large", fmt.Sprintf("the archive unpacks to more than %d bytes", MaxImportBytes),
		map[string]any{"limit": MaxImportBytes})
}

// importPathDetail bounds an archive path echoed in error details.
func importPathDetail(name string) string { return clipReason(name) }

// importPathSafe rejects archive names that could leave the project
// directory (zip slip) or would not unpack to the same file everywhere:
// absolute paths, drive letters, backslashes, "." or ".." segments, double
// slashes, control characters, characters Windows forbids (<>"|?*), Windows
// device names (aux.cpp) and segments ending in a dot or a space (Windows
// drops them). Directory entries end with "/".
func importPathSafe(name string) bool {
	trimmed := strings.TrimSuffix(name, "/")
	if trimmed == "" || len(name) > 300 || strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\:<>\"|?*") {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	if path.Clean(trimmed) != trimmed {
		return false
	}
	for _, seg := range strings.Split(trimmed, "/") {
		if seg == "." || seg == ".." || strings.HasSuffix(seg, ".") || strings.HasSuffix(seg, " ") || windowsReserved(seg) {
			return false
		}
	}
	return true
}

// importEntryRegular reports a regular file or a directory entry; links,
// devices, pipes and sockets are refused.
func importEntryRegular(f *zip.File) bool {
	t := f.Mode().Type()
	return t == 0 || t == fs.ModeDir
}

// importIgnored reports files an export may contain that are not imported:
// generated and template paths (regenerated on the next build), the README
// and tool metadata in hidden folders or __MACOSX.
func importIgnored(rel string) bool {
	if IsGeneratedPath(rel) || rel == ExportReadme || strings.HasPrefix(rel, "__MACOSX/") {
		return true
	}
	for _, seg := range strings.Split(rel, "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

// parseTemplateVersion parses "major.minor.patch" (each 0..9999).
func parseTemplateVersion(v string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("template version %q is not major.minor.patch", clipEcho(v))
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n > 9999 || part != strconv.Itoa(n) {
			return out, fmt.Errorf("template version %q is not major.minor.patch", clipEcho(v))
		}
		out[i] = n
	}
	return out, nil
}

// compareTemplateVersions returns -1, 0 or 1 for a <, ==, > b.
func compareTemplateVersions(a, b string) (int, error) {
	pa, err := parseTemplateVersion(a)
	if err != nil {
		return 0, err
	}
	pb, err := parseTemplateVersion(b)
	if err != nil {
		return 0, err
	}
	for i := range pa {
		switch {
		case pa[i] < pb[i]:
			return -1, nil
		case pa[i] > pb[i]:
			return 1, nil
		}
	}
	return 0, nil
}

// readZipEntry reads at most max bytes of one entry. The read itself is
// capped, so a header that understates the size cannot unpack more (the zip
// reader also fails when an entry is longer than its header claims).
func readZipEntry(f *zip.File, max int64) ([]byte, error) {
	if max < 0 || f.UncompressedSize64 > uint64(max) {
		return nil, errFileTooLarge
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, errFileTooLarge
	}
	return data, nil
}

// errImportTooLarge: the kept entries unpacked to more than MaxImportBytes.
var errImportTooLarge = errors.New("the archive unpacks to more than the import limit")

// importBudget counts the bytes actually unpacked from kept entries against
// MaxImportBytes, independent of what the headers declare. Dropped entries
// are never opened and never count.
type importBudget struct{ used int64 }

// read reads one kept entry within max and the remaining budget.
func (b *importBudget) read(f *zip.File, max int64) ([]byte, error) {
	limit := min(max, MaxImportBytes-b.used)
	data, err := readZipEntry(f, limit)
	if errors.Is(err, errFileTooLarge) && limit < max && f.UncompressedSize64 <= uint64(max) {
		return nil, errImportTooLarge
	}
	if err != nil {
		return nil, err
	}
	b.used += int64(len(data))
	return data, nil
}

// importArchive is a checked export: the layout entry and the entries to
// import by project-relative path.
type importArchive struct {
	layout *zip.File
	app    map[string]*zip.File
	assets map[string]*zip.File
}

// scanImportArchive checks the entry count, every path and entry type, lists
// every path once and the file types, and finds cyd.project.json either at
// the root or in the single top-level folder of an export. Sizes are checked
// later, for the kept entries only (importDeclaredSize).
func scanImportArchive(zr *zip.Reader) (importArchive, error) {
	if len(zr.File) > MaxImportEntries {
		return importArchive{}, importInvalid("too_many_entries",
			fmt.Sprintf("the archive has more than %d entries", MaxImportEntries), map[string]any{"limit": MaxImportEntries})
	}
	prefix := ""
	found := false
	seen := make(map[string]bool, len(zr.File))
	for _, f := range zr.File {
		if !importPathSafe(f.Name) || !importEntryRegular(f) {
			return importArchive{}, importInvalid("unsafe_path", "the archive contains an unsafe path", map[string]any{"path": importPathDetail(f.Name)})
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if seen[f.Name] {
			return importArchive{}, importInvalid("unsafe_path", "the archive lists a path twice", map[string]any{"path": importPathDetail(f.Name)})
		}
		seen[f.Name] = true
		switch dir, file := path.Split(f.Name); {
		case file != ProjectFile, strings.HasPrefix(dir, "__MACOSX/"), strings.HasPrefix(dir, "."):
		case dir == "":
			prefix, found = "", true
		case !found && strings.Count(dir, "/") == 1:
			prefix, found = dir, true
		}
	}
	if !found {
		return importArchive{}, importInvalid("layout_missing", "this is not a CYD Creator export: cyd.project.json is missing", nil)
	}
	arc := importArchive{app: map[string]*zip.File{}, assets: map[string]*zip.File{}}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || strings.HasPrefix(f.Name, "__MACOSX/") {
			continue
		}
		rel, ok := strings.CutPrefix(f.Name, prefix)
		switch {
		case !ok:
			return importArchive{}, importInvalid("file_type", "the archive contains files outside the project folder", map[string]any{"path": importPathDetail(f.Name)})
		case rel == ProjectFile:
			arc.layout = f
		case importIgnored(rel):
		case IsEditablePath(rel):
			arc.app[rel] = f
		case assetFileRegex.MatchString(rel):
			arc.assets[rel] = f
		default:
			return importArchive{}, importInvalid("file_type", "the archive contains a file type CYD Creator does not import", map[string]any{"path": importPathDetail(rel)})
		}
	}
	if arc.layout == nil {
		return importArchive{}, importInvalid("layout_missing", "this is not a CYD Creator export: cyd.project.json is missing", nil)
	}
	return arc, nil
}

// readImportLayout parses the layout and checks the name, the template
// version and the board. Errors never carry host paths.
func readImportLayout(f *zip.File, budget *importBudget) (Project, error) {
	data, err := budget.read(f, MaxLayoutBytes)
	if err != nil {
		return Project{}, importInvalid("layout", "cyd.project.json cannot be read", nil)
	}
	p, err := ParseProject(data)
	if err != nil {
		details := map[string]any{}
		var typed *Error
		if errors.As(err, &typed) {
			details["detail"] = typed.Message
		}
		return Project{}, importInvalid("layout", "cyd.project.json is not a valid CYD Creator layout", details)
	}
	if _, err := validProjectName(p.Name); err != nil {
		return Project{}, importInvalid("layout", "the project name in cyd.project.json is not valid", nil)
	}
	cmp, err := compareTemplateVersions(p.TemplateVersion, TemplateVersion)
	if err != nil {
		return Project{}, importInvalid("layout", err.Error(), nil)
	}
	if cmp > 0 {
		return Project{}, NewError(CodeTemplateTooNew, "the project needs a newer CYD Creator template",
			map[string]any{"template_version": p.TemplateVersion, "installed": TemplateVersion})
	}
	if _, err := BoardByID(p.Board); err != nil {
		return Project{}, importInvalid("board", "the board of the project is unknown", map[string]any{"board": clipEcho(p.Board)})
	}
	return p, nil
}

// checkImportAssets applies the layout save rules to the asset entries (file
// rule, each id and file once) and refuses a file with a Windows device name
// (the rule sanitizeAssetBase applies to uploads), which would not unpack on
// Windows.
func checkImportAssets(p Project) error {
	if err := checkLayoutAssets(nil, p.Assets); err != nil {
		details := map[string]any{}
		var typed *Error
		if errors.As(err, &typed) {
			details["asset"] = typed.Details["asset"]
		}
		return importInvalid("asset", "the image list of cyd.project.json is not valid", details)
	}
	for _, a := range p.Assets {
		if windowsReserved(strings.TrimSuffix(path.Base(a.File), path.Ext(a.File))) {
			return importInvalid("asset", "an image has a reserved Windows device name (aux, con, nul, ...)", map[string]any{"path": a.File})
		}
	}
	return nil
}

// importDeclaredSize refuses an archive whose kept entries (layout, app
// sources and the images the layout lists) declare more than MaxImportBytes.
// The sum saturates, so huge header values cannot wrap around.
func importDeclaredSize(arc importArchive, p Project) error {
	var total uint64
	add := func(f *zip.File) bool {
		if f.UncompressedSize64 > uint64(MaxImportBytes)-total {
			return false
		}
		total += f.UncompressedSize64
		return true
	}
	ok := add(arc.layout)
	for _, f := range arc.app {
		ok = ok && add(f)
	}
	for _, a := range p.Assets {
		if f, listed := arc.assets[a.File]; listed {
			ok = ok && add(f)
		}
	}
	if !ok {
		return importTooLarge()
	}
	return nil
}

// readImportApp reads the app files within the file limits. Paths, and every
// folder above them, must not differ from another only in letter case or be
// a file and a folder at once: Windows and macOS would open one as the other.
func readImportApp(entries map[string]*zip.File, budget *importBudget) (map[string][]byte, error) {
	if len(entries) > MaxAppFiles {
		return nil, importInvalid("app_files", fmt.Sprintf("more than %d source files", MaxAppFiles), map[string]any{"limit": MaxAppFiles})
	}
	names := make([]string, 0, len(entries))
	for rel := range entries {
		names = append(names, rel)
	}
	sort.Strings(names)
	type seenPath struct {
		orig string
		dir  bool
	}
	folded := map[string]seenPath{}
	for _, rel := range names {
		segs := strings.Split(rel, "/")
		for n := 1; n <= len(segs); n++ {
			p := strings.Join(segs[:n], "/")
			cur := seenPath{orig: p, dir: n < len(segs)}
			prev, dup := folded[strings.ToLower(p)]
			if !dup {
				folded[strings.ToLower(p)] = cur
				continue
			}
			if prev != cur {
				return nil, importInvalid("app_files", "two source files or folders differ only in letter case",
					map[string]any{"path": rel, "existing": prev.orig})
			}
		}
	}
	out := map[string][]byte{}
	var total int64
	for _, rel := range names {
		data, err := budget.read(entries[rel], MaxAppFileBytes)
		if errors.Is(err, errImportTooLarge) {
			return nil, importTooLarge()
		}
		if err != nil || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return nil, importInvalid("app_files", "a source file is too large or not UTF-8 text", map[string]any{"path": rel})
		}
		total += int64(len(data))
		if total > MaxAppTotalBytes {
			return nil, importInvalid("app_files", fmt.Sprintf("the source files have more than %d bytes", MaxAppTotalBytes),
				map[string]any{"limit": MaxAppTotalBytes})
		}
		out[rel] = data
	}
	return out, nil
}

// readImportAssets reads the images the layout lists; other images are left out.
// Only image headers are read here: each image must have the size the layout
// declares and fit the largest board display, and all images together must fit
// AssetBudgetLarge even as RGB565, so building an import can never decode an
// unbounded number of pixels (a 37 KB PNG can claim 2048x2048 16-bit pixels).
// The originals together stay within MaxAssetTotalBytes, like uploads.
func readImportAssets(p Project, entries map[string]*zip.File, budget *importBudget) (map[string][]byte, error) {
	long, short, err := maxBoardDisplay()
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	var minBytes, total int64
	for _, a := range p.Assets {
		f, ok := entries[a.File]
		if !ok {
			continue // the generator reports ASSET_MISSING
		}
		data, err := budget.read(f, MaxAssetUploadBytes)
		if errors.Is(err, errImportTooLarge) {
			return nil, importTooLarge()
		}
		if err != nil {
			return nil, importInvalid("asset", "an image is too large or damaged", map[string]any{"path": a.File})
		}
		if total += int64(len(data)); total > MaxAssetTotalBytes {
			return nil, importInvalid("asset", fmt.Sprintf("the images have more than %d bytes in total", MaxAssetTotalBytes),
				map[string]any{"path": a.File, "limit": "asset_bytes", "max": MaxAssetTotalBytes})
		}
		ext, _, ok := assetExtension(data)
		if !ok || (ext == ".png") != strings.HasSuffix(a.File, ".png") {
			return nil, importInvalid("asset", "an image is not a PNG or JPEG file of its extension", map[string]any{"path": a.File})
		}
		cfg, err := assetHeader(data)
		if err != nil || cfg.Width != a.W || cfg.Height != a.H ||
			max(cfg.Width, cfg.Height) > long || min(cfg.Width, cfg.Height) > short {
			return nil, importInvalid("asset", "an image is damaged, has another size than the layout says or is larger than every display", map[string]any{"path": a.File})
		}
		minBytes += imageDataSize(cfg.Width, cfg.Height, false)
		if minBytes > AssetBudgetLarge {
			return nil, importInvalid("asset", "the images are larger than the largest asset budget", map[string]any{"path": a.File, "budget": int64(AssetBudgetLarge)})
		}
		out[a.File] = data
	}
	return out, nil
}

// ImportProject creates a new project from a CYD Creator export (a ZIP with
// cyd.project.json of a known format at its root or in its single top-level
// folder). Paths are checked for zip slip, the archive may have at most
// MaxImportEntries entries, and the kept entries (layout, src/app sources and
// the listed PNG/JPEG assets) may unpack to at most MaxImportBytes, counted
// from the declared sizes and again while reading; generated and template
// files, the README and hidden folders are dropped unread and regenerated. A
// newer template than installed is refused with CYD_TEMPLATE_TOO_NEW. The
// project gets a new id and a unique slug (an existing project is never
// touched); a revision "import" records its first state and a
// project_changed event (origin import) follows. Everything is checked before
// the directory is allocated, and a failed write removes it again.
func (s *Service) ImportProject(ctx context.Context, data []byte, filename string) (ProjectSummary, error) {
	if len(data) == 0 || int64(len(data)) > MaxImportBytes {
		return ProjectSummary{}, importInvalid("too_large",
			fmt.Sprintf("the archive must have 1..%d bytes", MaxImportBytes), map[string]any{"limit": MaxImportBytes})
	}
	// With GODEBUG zipinsecurepath=0 NewReader also returns a usable reader
	// with ErrInsecurePath; scanImportArchive refuses the path itself.
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return ProjectSummary{}, importInvalid("not_zip", "the file is not a ZIP archive", nil)
	}
	arc, err := scanImportArchive(zr)
	if err != nil {
		return ProjectSummary{}, err
	}
	var budget importBudget
	p, err := readImportLayout(arc.layout, &budget)
	if err != nil {
		return ProjectSummary{}, err
	}
	if p.Timezone == "" {
		p.Timezone = "UTC0"
	}
	if err := checkImportAssets(p); err != nil {
		return ProjectSummary{}, err
	}
	if err := importDeclaredSize(arc, p); err != nil {
		return ProjectSummary{}, err
	}
	app, err := readImportApp(arc.app, &budget)
	if err != nil {
		return ProjectSummary{}, err
	}
	assets, err := readImportAssets(p, arc.assets, &budget)
	if err != nil {
		return ProjectSummary{}, err
	}
	layout, err := MarshalProject(p)
	if err != nil {
		var typed *Error
		if errors.As(err, &typed) && typed.Code == CodeInvalidLayout {
			return ProjectSummary{}, importInvalid("layout", "cyd.project.json is too large in canonical form",
				map[string]any{"limit": typed.Details["limit"], "bytes": typed.Details["bytes"]})
		}
		return ProjectSummary{}, s.internalError("import project", err)
	}
	files := map[string][]byte{ProjectFile: layout}
	for rel, content := range app {
		files[rel] = content
	}
	for rel, content := range assets {
		files[rel] = content
	}
	if len(app) == 0 {
		for rel, content := range ScaffoldAppFiles(p) {
			if IsEditablePath(rel) {
				files[rel] = content
			}
		}
	}

	s.createMu.Lock()
	row, err := s.writeImportedProject(ctx, p, files)
	s.createMu.Unlock()
	if err != nil {
		return ProjectSummary{}, err
	}
	label := "Import"
	if base := strings.TrimSpace(path.Base(strings.ReplaceAll(filename, "\\", "/"))); base != "" && base != "." && base != "/" {
		label = "Import " + truncateRunes(cleanDeviceText(base), 80)
	}
	if _, err := s.RecordRevision(ctx, row.ID, label, RevisionImport); err != nil {
		s.logger.Warn("CYD Creator import revision could not be recorded", "project_id", row.ID, "error", err)
	}
	s.Emit(row.ID, EventProjectChanged, "", "", projectChange(row.Version, OriginImport, "layout"))
	return s.summary(ctx, row), nil
}

// writeImportedProject allocates a fresh project directory (unique slug),
// writes the files and inserts the row. Any failure removes the directory
// again. Callers hold createMu; nothing here emits.
func (s *Service) writeImportedProject(ctx context.Context, p Project, files map[string][]byte) (projectRow, error) {
	slug, err := s.allocateProjectDir(ctx, p.Name)
	if err != nil {
		return projectRow{}, s.internalError("import project", err)
	}
	dir := s.projectDir(slug)
	fail := func(err error) (projectRow, error) {
		if rerr := os.RemoveAll(dir); rerr != nil {
			s.logger.Warn("CYD Creator import could not remove its directory", "slug", slug, "error", rerr)
		}
		return projectRow{}, s.internalError("import project", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fail(err)
	}
	names := make([]string, 0, len(files))
	for rel := range files {
		names = append(names, rel)
	}
	sort.Strings(names)
	for _, rel := range names {
		if err := rootWriteFile(ctx, root, rel, files[rel]); err != nil {
			_ = root.Close()
			return fail(err)
		}
	}
	if err := root.Close(); err != nil {
		return fail(err)
	}
	// A theme the layout does not know keeps the default in the row, like a
	// layout save keeps the previous one (layoutRowMeta).
	name, board, theme := layoutRowMeta(projectRow{Name: p.Name, Board: p.Board, Theme: defaultProjectTheme}, p)
	now := formatTime(s.now())
	row := projectRow{ID: s.ids.NewID("ccp"), Slug: slug, Name: name, Board: board, Theme: theme, Version: 1,
		CreatedAt: now, UpdatedAt: now}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO cc_projects(id,slug,name,board,theme,version,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?)`, row.ID, row.Slug, row.Name, row.Board, row.Theme, row.Version, row.CreatedAt, row.UpdatedAt); err != nil {
		return fail(err)
	}
	return row, nil
}

// truncateRunes cuts s to at most n runes.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
