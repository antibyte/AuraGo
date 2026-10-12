package cydcreator

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

// ExportReadme is the file name of the README in every export.
const ExportReadme = "README.md"

// exportBuildIDFile replaces the header cyd-build writes per build (plan 1b:
// only cyd_build_info.cpp includes it), so an export builds outside the
// container; a CYD Creator build overwrites it with the real build id.
const (
	exportBuildIDFile    = "include/cyd_build_id.h"
	exportBuildIDContent = "// Written by the CYD Creator export. A CYD Creator build replaces it with the real build id.\n#define CYD_BUILD_ID \"export\"\n"
)

// maxExportBytes caps the unpacked size of an export. The archive is built in
// memory (the HTTP handler needs its length and a typed error before the
// first byte). Derived from the project limits:
//
//	cyd.project.json  MaxLayoutBytes                        1 MiB
//	app files         2 × MaxAppTotalBytes                  4 MiB (hand-placed files skip the write limit)
//	image originals   MaxAssetTotalBytes                   16 MiB
//	ui_assets.cpp     6 × AssetBudgetLarge                 24 MiB ("0xAB, " per decoded image byte)
//	fonts             maxFontArtifactBytes (7 × 0x640000)  ~45 MiB (fonts filling the largest app slot)
//
// ≈ 90 MiB, rounded up to 128 MiB for ui.cpp, the template and fonts.json
// (TestMaxExportBytesCoversTheProjectLimits). A variable only so a test can
// lower it.
var maxExportBytes int64 = 128 << 20

// exportSlots lets two exports run at a time (each may hold maxExportBytes of
// files plus the compressed archive in memory); another one is CYD_BUSY
// (reason export). AuraGo runs one Service, so the package-wide slots are the
// service-wide ones.
var exportSlots = make(chan struct{}, 2)

// safeArchivePath reports a clean relative slash path that unpacks to the
// same files on Windows, macOS and Linux: at most 240 bytes, no "..", empty or
// hidden segments, no backslashes, colons, control characters or characters
// Windows forbids (<>"|?*), no Windows device names (aux.png) and no segment
// that ends in a dot or a space (Windows drops them).
func safeArchivePath(rel string) bool {
	if rel == "" || len(rel) > 240 || strings.ContainsAny(rel, "\\:\x00<>\"|?*") || path.Clean(rel) != rel || strings.HasPrefix(rel, "/") {
		return false
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.HasPrefix(seg, ".") ||
			strings.HasSuffix(seg, ".") || strings.HasSuffix(seg, " ") || windowsReserved(seg) {
			return false
		}
		for _, r := range seg {
			if r < 0x20 || r == 0x7f {
				return false
			}
		}
	}
	return true
}

// exportReadme explains how to build the export with PlatformIO and what it
// leaves out (skipped: project paths safeArchivePath refused). It never
// contains secrets.
func exportReadme(p Project, board Board, generated bool, fonts exportFontState, skipped []string, exported time.Time) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", p.Name)
	fmt.Fprintf(&b, "Exported from AuraGo CYD Creator on %s.\n\n", exported.UTC().Format("2006-01-02"))
	fmt.Fprintf(&b, "- Board: %s (`%s`)\n", board.Label, board.ID)
	fmt.Fprintf(&b, "- Template version: %s\n", p.TemplateVersion)
	fmt.Fprintf(&b, "- Theme: %s\n\n", p.Theme)
	if generated {
		b.WriteString("## Build and flash with PlatformIO\n\n")
		b.WriteString("1. Install PlatformIO (the VS Code extension or `pip install platformio`).\n")
		fmt.Fprintf(&b, "2. Build: `pio run -e %s`\n", board.PIOEnv())
		fmt.Fprintf(&b, "3. Flash: `pio run -e %s -t upload` (hold BOOT if the upload does not start).\n\n", board.PIOEnv())
		b.WriteString("The libraries in platformio.ini are pinned to the builder image's versions.\n\n")
		switch {
		case fonts.BuildID == "":
			b.WriteString("> Fonts: no CYD Creator build has converted the fonts of this layout yet, so `src/generated/fonts/`\n")
			b.WriteString("> is missing and PlatformIO cannot link the UI. Build the project once in CYD Creator, then export again.\n\n")
		case fonts.Current:
			fmt.Fprintf(&b, "Fonts: `src/generated/fonts/` holds the LVGL fonts of build %s (converted from\n", fonts.BuildID)
			b.WriteString("`src/generated/fonts.json` with lv_font_conv 1.5.3, 4 bpp, compressed); PlatformIO compiles them with the UI.\n\n")
		default:
			b.WriteString("> Warning: the layout's fonts changed after the newest build. `src/generated/fonts/` holds the fonts of\n")
			fmt.Fprintf(&b, "> build %s and may miss sizes or characters. Build once in CYD Creator and export again.\n\n", fonts.BuildID)
		}
	} else {
		b.WriteString("> CYD Creator could not generate the build files when this archive was exported (the layout or one\n")
		b.WriteString("> of its images has errors), so the generated files (platformio.ini, src/generated, include,\n")
		b.WriteString("> lib/cyd_template, partitions.csv) are missing and PlatformIO cannot build it. Import the archive\n")
		b.WriteString("> into CYD Creator and fix the problems it shows.\n\n")
	}
	b.WriteString("## What is in this archive\n\n")
	b.WriteString("- `cyd.project.json` is the UI layout. CYD Creator generates `src/generated`, `include`,\n")
	b.WriteString("  `lib/cyd_template`, `platformio.ini` and `partitions.csv` from it on every build: change the layout\n")
	b.WriteString("  in CYD Creator, not these files.\n")
	b.WriteString("- Your app code is in `src/app`, images are in `assets`.\n")
	b.WriteString("- Wi-Fi credentials and AuraGo device tokens are never exported. A board flashed from this archive\n")
	b.WriteString("  has no AuraGo link (no updates over Wi-Fi) until it is flashed from CYD Creator. Your code is\n")
	b.WriteString("  exported as it is: check src/app for keys or passwords before sharing the archive.\n\n")
	if len(skipped) > 0 {
		b.WriteString("## Not exported\n\n")
		b.WriteString("These project files have names that do not unpack safely on every system (Windows device names,\n")
		b.WriteString("reserved characters) and are not in the archive:\n\n")
		for _, rel := range skipped {
			fmt.Fprintf(&b, "- %q\n", rel)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Import\n\n")
	b.WriteString("Use \"Import ZIP\" on the CYD Creator start screen to open this archive as a new project.\n")
	return []byte(b.String())
}

// exportFontState says which converted fonts an export carries: BuildID is
// the build they come from ("" = none), Current that its fonts.json equals
// the exported one.
type exportFontState struct {
	BuildID string
	Current bool
}

// exportFonts adds the converted fonts of the newest succeeded build to files
// as src/generated/fonts/<symbol>.c (cyd-build converts them only inside the
// container; plan 2 keeps them with the build). The newest build whose
// fonts.json equals the exported one wins; otherwise the newest build that
// kept fonts is used and reported as not current. Only the fonts.json of each
// build is read to choose; the converted fonts only of the chosen one.
func (s *Service) exportFonts(ctx context.Context, projectID string, fontsJSON []byte, files map[string][]byte) (exportFontState, error) {
	builds, err := s.ListBuilds(ctx, projectID)
	if err != nil {
		return exportFontState{}, err
	}
	want := sha256Hex(fontsJSON)
	var chosen exportFontState
	for _, b := range builds { // newest first
		if b.Status != BuildSucceeded || !validBuildID(b.ID) {
			continue
		}
		data, err := s.readArtifact(b.ID, path.Join(ArtifactFontsDir, "fonts.json"), maxFontArtifactBytes)
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, errNotRegularFile) {
			continue
		}
		if err != nil {
			return exportFontState{}, s.internalError("export fonts", err)
		}
		if len(fontsJSON) > 0 && sha256Hex(data) == want {
			chosen = exportFontState{BuildID: b.ID, Current: true}
			break
		}
		if chosen.BuildID == "" {
			chosen = exportFontState{BuildID: b.ID}
		}
	}
	if chosen.BuildID == "" {
		return chosen, nil
	}
	sha, fontFiles, err := s.BuildFonts(ctx, chosen.BuildID)
	if err != nil {
		return exportFontState{}, err
	}
	if sha == "" { // the retention removed the build meanwhile
		return exportFontState{}, nil
	}
	chosen.Current = chosen.Current && sha == want
	addExportFonts(files, fontFiles)
	return chosen, nil
}

func addExportFonts(files, fonts map[string][]byte) {
	for name, data := range fonts {
		files["src/generated/fonts/"+name] = data
	}
}

// exportTree is what an export reads from the project under the project
// mutex: everything after it works on these copies.
type exportTree struct {
	slug      string
	project   Project
	board     Board
	generated bool
	fontsJSON []byte
	files     map[string][]byte
}

// snapshotExport reads the project for ExportProject under the project mutex,
// so a concurrent save, publication or restore never tears the archive. A
// fenced project (unresolved publication) is refused. When the generator
// refuses the project (an invalid layout, missing or unusable image data,
// CYD_BAD_REQUEST, or a generator failure) only the sources are kept.
func (s *Service) snapshotExport(ctx context.Context, projectID string) (exportTree, error) {
	unlock := s.lockProject(projectID)
	defer unlock()
	row, err := s.loadProjectRow(ctx, projectID)
	if err != nil {
		return exportTree{}, s.internalError("export project", err)
	}
	if row.PublishReview != "" {
		return exportTree{}, publicationReviewError()
	}
	root, err := s.openProjectRoot(row)
	if err != nil {
		return exportTree{}, s.internalError("export project", err)
	}
	defer root.Close()
	in, err := loadGenerateInput(row, root)
	if err != nil {
		return exportTree{}, s.internalError("export project", err)
	}
	t := exportTree{slug: row.Slug, project: in.Project, generated: true, files: map[string][]byte{}}
	out, err := s.runGenerate(in)
	switch code := ErrorCode(err); {
	case err == nil:
		for rel, data := range out.Files {
			t.files[rel] = data
		}
		if _, ok := t.files[exportBuildIDFile]; !ok {
			t.files[exportBuildIDFile] = []byte(exportBuildIDContent)
		}
		t.fontsJSON = out.Files["src/generated/fonts.json"]
	case code == CodeInvalidLayout || code == CodeBadRequest || code == CodeGeneratorError:
		// The real generator decodes the images before it validates and
		// refuses missing or unusable image data with CYD_BAD_REQUEST
		// (details.asset); a generator failure is a bug, not the user's. The
		// export keeps the sources either way, so it still works as a backup.
		t.generated = false
		for rel, data := range in.AppFiles {
			t.files[rel] = data
		}
	default:
		return exportTree{}, err
	}
	layout, err := MarshalProject(in.Project)
	if err != nil {
		return exportTree{}, s.internalError("export project", err)
	}
	t.files[ProjectFile] = layout
	for _, a := range in.Project.Assets {
		if data, ok := in.Assets[a.ID]; ok {
			t.files[a.File] = data
		}
	}
	t.board, err = BoardByID(in.Project.Board)
	if err != nil && !t.generated {
		// An unknown board is a validation error of the layout; the row keeps
		// the last valid one for the README.
		t.board, err = BoardByID(row.Board)
	}
	if err != nil {
		return exportTree{}, err
	}
	return t, nil
}

// ExportProject returns a ZIP with the complete PlatformIO project of the
// current version below "<slug>/": the generated tree (platformio.ini with the
// pinned lib_deps, src/generated, include, lib/cyd_template, partitions.csv),
// the app files, the converted fonts of the newest matching build (see
// exportFonts), cyd.project.json, the original assets and a README. When the
// generator refuses the project the generated files are left out (the README
// says so), so an export also works as a backup. The project is read under
// its mutex (snapshotExport); the fonts are read and the archive is
// compressed after it is released (build artefacts never change). Paths that
// would not unpack safely everywhere are left out and listed in the README.
// At most two exports run at a time (else CYD_BUSY, reason export) and an
// archive above maxExportBytes unpacked is CYD_FILE_LIMIT (details.limit
// export_bytes). Device tokens and Wi-Fi credentials are not part of a
// project and therefore never exported.
func (s *Service) ExportProject(ctx context.Context, projectID string) ([]byte, string, error) {
	select {
	case exportSlots <- struct{}{}:
		defer func() { <-exportSlots }()
	default:
		return nil, "", NewError(CodeBusy, "another export is running; try again in a moment", map[string]any{"reason": "export"})
	}
	t, err := s.snapshotExport(ctx, projectID)
	if err != nil {
		return nil, "", err
	}
	fonts := exportFontState{}
	if t.generated {
		if fonts, err = s.exportFonts(ctx, projectID, t.fontsJSON, t.files); err != nil {
			return nil, "", err
		}
	}
	names := make([]string, 0, len(t.files)+1)
	var skipped []string
	for rel := range t.files {
		if rel == ExportReadme {
			continue
		}
		if !safeArchivePath(rel) {
			s.logger.Warn("CYD Creator export skipped an unsafe path", "project_id", projectID, "path", rel)
			skipped = append(skipped, rel)
			continue
		}
		names = append(names, rel)
	}
	sort.Strings(skipped)
	now := s.now()
	t.files[ExportReadme] = exportReadme(t.project, t.board, t.generated, fonts, skipped, now)
	names = append(names, ExportReadme)
	sort.Strings(names)
	var total int64
	for _, rel := range names {
		if total += int64(len(t.files[rel])); total > maxExportBytes {
			return nil, "", NewError(CodeFileLimit, fmt.Sprintf("the export would unpack to more than %d bytes", maxExportBytes),
				map[string]any{"limit": "export_bytes", "max": maxExportBytes})
		}
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, rel := range names {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: t.slug + "/" + rel, Method: zip.Deflate, Modified: now})
		if err != nil {
			return nil, "", s.internalError("export project", err)
		}
		if _, err := w.Write(t.files[rel]); err != nil {
			return nil, "", s.internalError("export project", err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, "", s.internalError("export project", err)
	}
	return buf.Bytes(), t.slug + ".zip", nil
}
