package cydcreator

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// GenerateInput is everything the deterministic generator reads.
type GenerateInput struct {
	Project   Project
	ProjectID string            // server project id (ccp_…), written to board_config.h
	AppFiles  map[string][]byte // project-relative src/app/** as stored
	Assets    map[string][]byte // asset id → original PNG/JPEG bytes
}

// GenerateOutput is the complete PlatformIO tree plus the validation result.
type GenerateOutput struct {
	Files  map[string][]byte // template lib + generated files + app files + config files
	Issues []Issue
}

// Generate turns a project into the full PlatformIO tree cyd-build compiles:
// lib/cyd_template/** (embedded template), src/app/** (as given),
// include/board_config.h, platformio.ini, partitions.csv and
// src/generated/{ui.h,ui.cpp,ui_theme.cpp,ui_assets.cpp,ui_hooks_weak.cpp,fonts.json}.
// The same input always gives byte-identical files. include/cyd_build_id.h is
// never generated (cyd-build writes it). A layout with validation errors is
// refused with *Error CYD_INVALID_LAYOUT (details.issues); unreadable or
// missing asset data and app paths outside src/app are CYD_BAD_REQUEST.
func Generate(in GenerateInput) (GenerateOutput, error) {
	p := in.Project
	if p.TemplateVersion != TemplateVersion {
		return GenerateOutput{}, layoutError(fmt.Sprintf("template_version %q differs from the embedded template %s", p.TemplateVersion, TemplateVersion),
			map[string]any{"template_version": p.TemplateVersion, "embedded": TemplateVersion})
	}
	images, sizes, err := decodeProjectAssets(p, in.Assets)
	if err != nil {
		return GenerateOutput{}, err
	}
	out := GenerateOutput{Issues: Validate(p, ValidateOptions{AssetSizes: sizes})}
	if HasErrors(out.Issues) {
		n := 0
		for _, is := range out.Issues {
			if is.Severity == SeverityError {
				n++
			}
		}
		return out, NewError(CodeInvalidLayout, fmt.Sprintf("the layout has %d validation errors", n),
			map[string]any{"issues": out.Issues})
	}
	board, err := BoardByID(p.Board)
	if err != nil {
		return out, err
	}
	theme, err := ThemeByID(p.Theme)
	if err != nil {
		return out, err
	}
	files := map[string][]byte{}
	if err := copyTemplateLib(files); err != nil {
		return out, err
	}
	for rel, data := range in.AppFiles {
		if !IsEditablePath(rel) {
			return out, NewError(CodeBadRequest, fmt.Sprintf("app file %q is not below src/app", rel), map[string]any{"path": rel})
		}
		files[rel] = append([]byte(nil), data...)
	}
	csv, err := PartitionCSV(board.FlashMB)
	if err != nil {
		return out, err
	}
	fonts, err := renderFontsJSON(p, theme)
	if err != nil {
		return out, err
	}
	g := newUIGen(p, theme, images)
	files["platformio.ini"] = []byte(renderPlatformIOIni(p, board))
	files["partitions.csv"] = []byte(csv)
	files["include/board_config.h"] = []byte(renderBoardConfig(p, board, in.ProjectID))
	files["src/generated/ui.h"] = []byte(g.renderHeader())
	files["src/generated/ui.cpp"] = []byte(g.renderSource())
	files["src/generated/ui_theme.cpp"] = []byte(renderTheme(theme))
	files["src/generated/ui_assets.cpp"] = []byte(renderAssets(images))
	files["src/generated/ui_hooks_weak.cpp"] = []byte(g.renderHooks())
	files["src/generated/fonts.json"] = fonts
	out.Files = files
	return out, nil
}

// copyTemplateLib copies TemplateFS() lib/cyd_template/** into files.
func copyTemplateLib(files map[string][]byte) error {
	return fs.WalkDir(TemplateFS(), "lib/cyd_template", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(TemplateFS(), p)
		if err != nil {
			return err
		}
		files[p] = data
		return nil
	})
}

// ScaffoldAppFiles returns the initial src/app files of a new project (the
// embedded template/scaffold tree). They are created once and then belong to
// the user and the agent; Generate never overwrites them.
func ScaffoldAppFiles(p Project) map[string][]byte {
	_ = p // reserved: future scaffolds may depend on the board or settings
	out := map[string][]byte{}
	_ = fs.WalkDir(TemplateFS(), "scaffold", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(TemplateFS(), name)
		if err != nil {
			return err
		}
		out[path.Clean(strings.TrimPrefix(name, "scaffold/"))] = data
		return nil
	})
	return out
}
