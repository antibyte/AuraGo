package cydcreator

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
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
// memory (the HTTP handler needs its length and a typed error before the first
// byte), and every input is already read through a capped reader; the largest
// valid project (fonts filling a 16 MB board's app slot, a 4 MiB image budget)
// stays far below it. A variable only so a test can lower it.
var maxExportBytes int64 = 256 << 20

// safeArchivePath reports a clean relative slash path without "..", empty or
// hidden segments, backslashes, colons or control characters.
func safeArchivePath(rel string) bool {
	if rel == "" || len(rel) > 240 || strings.ContainsAny(rel, "\\:\x00") || path.Clean(rel) != rel || strings.HasPrefix(rel, "/") {
		return false
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.HasPrefix(seg, ".") {
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
// leaves out. It never contains secrets.
func exportReadme(p Project, board Board, generated bool, fonts exportFontState, exported time.Time) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", p.Name)
	fmt.Fprintf(&b, "Exported from AuraGo CYD Creator on %s.\n\n", exported.UTC().Format("2006-01-02"))
	fmt.Fprintf(&b, "- Board: %s (`%s`)\n", board.Label, board.ID)
	fmt.Fprintf(&b, "- Template version: %s\n", p.TemplateVersion)
	fmt.Fprintf(&b, "- Theme: %s\n\n", p.Theme)
	if !generated {
		b.WriteString("> The layout or one of its images had errors when this archive was exported, so the generated files\n")
		b.WriteString("> (platformio.ini, src/generated, include, lib/cyd_template, partitions.csv) are missing. Import the\n")
		b.WriteString("> archive into CYD Creator and fix the problems it shows.\n\n")
	}
	b.WriteString("## Build and flash with PlatformIO\n\n")
	b.WriteString("1. Install PlatformIO (the VS Code extension or `pip install platformio`).\n")
	fmt.Fprintf(&b, "2. Build: `pio run -e %s`\n", board.PIOEnv())
	fmt.Fprintf(&b, "3. Flash: `pio run -e %s -t upload` (hold BOOT if the upload does not start).\n\n", board.PIOEnv())
	b.WriteString("The libraries in platformio.ini are pinned to the versions of the CYD Creator build image, so the\n")
	b.WriteString("project builds the same way online.\n\n")
	switch {
	case !generated:
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
	b.WriteString("## What is in this archive\n\n")
	b.WriteString("- `cyd.project.json` is the UI layout. CYD Creator generates `src/generated`, `include`,\n")
	b.WriteString("  `lib/cyd_template`, `platformio.ini` and `partitions.csv` from it on every build: change the layout\n")
	b.WriteString("  in CYD Creator, not these files.\n")
	b.WriteString("- Your app code is in `src/app`, images are in `assets`.\n")
	b.WriteString("- Wi-Fi credentials and AuraGo device tokens are never exported. A board flashed from this archive\n")
	b.WriteString("  has no AuraGo link (no updates over Wi-Fi) until it is flashed from CYD Creator.\n\n")
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
// kept fonts is used and reported as not current.
func (s *Service) exportFonts(ctx context.Context, projectID string, fontsJSON []byte, files map[string][]byte) (exportFontState, error) {
	builds, err := s.ListBuilds(ctx, projectID)
	if err != nil {
		return exportFontState{}, err
	}
	want := sha256Hex(fontsJSON)
	var fallback exportFontState
	var fallbackFiles map[string][]byte
	for _, b := range builds { // newest first
		if b.Status != BuildSucceeded {
			continue
		}
		sha, fontFiles, err := s.BuildFonts(ctx, b.ID)
		if err != nil {
			return exportFontState{}, err
		}
		if sha == "" {
			continue
		}
		if len(fontsJSON) > 0 && sha == want {
			addExportFonts(files, fontFiles)
			return exportFontState{BuildID: b.ID, Current: true}, nil
		}
		if fallbackFiles == nil {
			fallback, fallbackFiles = exportFontState{BuildID: b.ID}, fontFiles
		}
	}
	addExportFonts(files, fallbackFiles)
	return fallback, nil
}

func addExportFonts(files, fonts map[string][]byte) {
	for name, data := range fonts {
		files["src/generated/fonts/"+name] = data
	}
}

// ExportProject returns a ZIP with the complete PlatformIO project of the
// current version below "<slug>/": the generated tree (platformio.ini with the
// pinned lib_deps, src/generated, include, lib/cyd_template, partitions.csv),
// the app files, the converted fonts of the newest matching build (see
// exportFonts), cyd.project.json, the original assets and a README. When the
// layout has errors, or the generator refuses an image (missing or unusable
// data, CYD_BAD_REQUEST), the generated files are left out (the README says
// so), so an export also works as a backup. Everything is read under the
// project mutex, so a concurrent save, publication or restore never tears the
// archive; a fenced project (unresolved publication) is refused. Device tokens
// and Wi-Fi credentials are not part of a project and therefore never exported.
func (s *Service) ExportProject(ctx context.Context, projectID string) ([]byte, string, error) {
	unlock := s.lockProject(projectID)
	defer unlock()
	row, err := s.loadProjectRow(ctx, projectID)
	if err != nil {
		return nil, "", s.internalError("export project", err)
	}
	if row.PublishReview != "" {
		return nil, "", publicationReviewError()
	}
	root, err := s.openRoot(row, "")
	if err != nil {
		return nil, "", s.internalError("export project", err)
	}
	defer root.Close()
	in, err := loadGenerateInput(row, root)
	if err != nil {
		return nil, "", s.internalError("export project", err)
	}
	files := map[string][]byte{}
	generated := true
	out, err := s.runGenerate(in)
	switch code := ErrorCode(err); {
	case err == nil:
		for rel, data := range out.Files {
			files[rel] = data
		}
		if _, ok := files[exportBuildIDFile]; !ok {
			files[exportBuildIDFile] = []byte(exportBuildIDContent)
		}
	case code == CodeInvalidLayout || code == CodeBadRequest:
		// The real generator decodes the images before it validates and
		// refuses missing or unusable image data with CYD_BAD_REQUEST
		// (details.asset): export the sources like for an invalid layout.
		generated = false
		for rel, data := range in.AppFiles {
			files[rel] = data
		}
	default:
		return nil, "", err
	}
	layout, err := MarshalProject(in.Project)
	if err != nil {
		return nil, "", s.internalError("export project", err)
	}
	files[ProjectFile] = layout
	for _, a := range in.Project.Assets {
		if data, ok := in.Assets[a.ID]; ok {
			files[a.File] = data
		}
	}
	board, err := BoardByID(in.Project.Board)
	if err != nil && !generated {
		// An unknown board is a validation error of the layout; the row keeps
		// the last valid one for the README.
		board, err = BoardByID(row.Board)
	}
	if err != nil {
		return nil, "", err
	}
	fonts := exportFontState{}
	if generated {
		if fonts, err = s.exportFonts(ctx, projectID, out.Files["src/generated/fonts.json"], files); err != nil {
			return nil, "", err
		}
	}
	now := s.now()
	files[ExportReadme] = exportReadme(in.Project, board, generated, fonts, now)

	names := make([]string, 0, len(files))
	var total int64
	for rel, data := range files {
		if !safeArchivePath(rel) {
			s.logger.Warn("CYD Creator export skipped an unsafe path", "project_id", projectID, "path", rel)
			continue
		}
		if total += int64(len(data)); total > maxExportBytes {
			return nil, "", NewError(CodeFileLimit, fmt.Sprintf("the export would unpack to more than %d bytes", maxExportBytes),
				map[string]any{"limit": "export_bytes", "max": maxExportBytes})
		}
		names = append(names, rel)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, rel := range names {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: row.Slug + "/" + rel, Method: zip.Deflate, Modified: now})
		if err != nil {
			return nil, "", s.internalError("export project", err)
		}
		if _, err := w.Write(files[rel]); err != nil {
			return nil, "", s.internalError("export project", err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, "", s.internalError("export project", err)
	}
	return buf.Bytes(), row.Slug + ".zip", nil
}
