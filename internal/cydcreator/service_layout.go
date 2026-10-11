package cydcreator

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// readLayout reads and strictly parses cyd.project.json from a project tree.
// A missing, oversized or non-regular file is CYD_INVALID_LAYOUT like
// unparsable content (details.reason missing, too_large or not_regular).
func readLayout(root *os.Root) (Project, error) {
	data, err := rootReadFile(root, ProjectFile, MaxLayoutBytes+1)
	if err != nil {
		return Project{}, layoutFileError(ProjectFile, fmt.Errorf("read %s: %w", ProjectFile, err))
	}
	return ParseProject(data)
}

// layoutFileError maps a damaged project tree to CYD_INVALID_LAYOUT; other
// errors are returned unchanged (internal).
func layoutFileError(what string, err error) error {
	var reason, problem string
	switch {
	case errors.Is(err, fs.ErrNotExist):
		reason, problem = "missing", "is missing"
	case errors.Is(err, errFileTooLarge):
		reason, problem = "too_large", fmt.Sprintf("exceeds %d bytes", MaxLayoutBytes)
	case errors.Is(err, errNotRegularFile):
		reason, problem = "not_regular", "is not a regular file"
	default:
		return err
	}
	return NewError(CodeInvalidLayout, fmt.Sprintf("invalid layout: %s %s", what, problem), map[string]any{"reason": reason})
}

// openProjectRoot opens the project directory; a missing directory is
// CYD_INVALID_LAYOUT (details.reason missing).
func (s *Service) openProjectRoot(row projectRow) (*os.Root, error) {
	root, err := s.openRoot(row, "")
	if err != nil {
		return nil, layoutFileError("the project directory", err)
	}
	return root, nil
}

// layoutAssetSizes returns the decoded LVGL size of every listed asset
// (RGB565 = w*h*2, RGB565A8 = w*h*3) for the Validate budget check.
func layoutAssetSizes(p Project) map[string]int64 {
	sizes := make(map[string]int64, len(p.Assets))
	for _, a := range p.Assets {
		bpp := int64(2)
		if a.Alpha {
			bpp = 3
		}
		sizes[a.ID] = int64(a.W) * int64(a.H) * bpp
	}
	return sizes
}

func validateLayout(p Project) []Issue {
	issues := Validate(p, ValidateOptions{AssetSizes: layoutAssetSizes(p)})
	if issues == nil {
		issues = []Issue{}
	}
	return issues
}

// checkLayoutAssets keeps asset entries pointing at their own image. PutAsset
// adds images and DeleteAsset removes them together with their files. A layout
// save may keep or drop stored entries but never change one (File, W, H,
// Alpha), and may list an entry the stored layout lacks (designer undo/redo,
// a second tab overwriting) only when its File belongs to no other stored
// entry; the file need not exist (Validate/Generate report a missing image).
// Every entry must match the asset file rule, and each ID and File is listed
// once, so no entry can point DeleteAsset, OpenAsset or the generator at the
// layout, app sources or another entry's image.
func checkLayoutAssets(stored, incoming []Asset) error {
	known := make(map[string]Asset, len(stored))
	storedFiles := make(map[string]string, len(stored))
	for _, a := range stored {
		known[a.ID] = a
		storedFiles[a.File] = a.ID
	}
	ids := make(map[string]bool, len(incoming))
	files := make(map[string]bool, len(incoming))
	for _, a := range incoming {
		details := map[string]any{"asset": clipEcho(a.ID)}
		if !assetFileRegex.MatchString(a.File) {
			return layoutError("asset file must match assets/<name>.png, .jpg or .jpeg", details)
		}
		if ids[a.ID] || files[a.File] {
			return layoutError("an asset is listed twice", details)
		}
		ids[a.ID], files[a.File] = true, true
		old, ok := known[a.ID]
		if !ok {
			if owner, used := storedFiles[a.File]; used && owner != a.ID {
				return layoutError("the asset file belongs to another image", details)
			}
			continue
		}
		if old != a {
			return layoutError("asset entries cannot be changed by saving the layout", details)
		}
	}
	return nil
}

// saveLayoutLocked writes the canonical layout, bumps the version and mirrors
// name, board and theme into the row (a value that Validate reports as
// invalid keeps the row's previous one). Callers hold the project lock.
func (s *Service) saveLayoutLocked(ctx context.Context, root *os.Root, row projectRow, p Project) (int64, error) {
	data, err := MarshalProject(p)
	if err != nil {
		return 0, err
	}
	// The round trip guarantees that the stored file parses again (ids, size).
	if _, err := ParseProject(data); err != nil {
		return 0, err
	}
	if err := rootWriteFile(ctx, root, ProjectFile, data); err != nil {
		return 0, err
	}
	name, board, theme := row.Name, row.Board, row.Theme
	if projectNameProblem(p.Name) == "" {
		name = p.Name
	}
	if _, err := BoardByID(p.Board); err == nil {
		board = p.Board
	}
	if _, err := ThemeByID(p.Theme); err == nil {
		theme = p.Theme
	}
	// The file is published: finish the version bump even if the request is
	// cancelled now. The lock makes row.Version current; the CAS only guards it.
	version := row.Version + 1
	res, err := s.db.ExecContext(context.WithoutCancel(ctx),
		`UPDATE cc_projects SET version=?, name=?, board=?, theme=?, updated_at=? WHERE id=? AND version=?`,
		version, name, board, theme, formatTime(s.now()), row.ID, row.Version)
	if err == nil {
		var n int64
		if n, err = res.RowsAffected(); err == nil && n != 1 {
			err = fmt.Errorf("project version is no longer %d", row.Version)
		}
	}
	if err != nil {
		s.logger.Warn("CYD Creator layout file was written but the project version was not bumped",
			"project_id", row.ID, "version", row.Version, "error", err)
		return 0, fmt.Errorf("update project version: %w", err)
	}
	return version, nil
}

// GetLayout returns the layout, its version and the validation issues.
func (s *Service) GetLayout(ctx context.Context, id string) (Project, int64, []Issue, error) {
	unlock := s.lockProject(id)
	defer unlock()
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return Project{}, 0, nil, s.internalError("read layout", err)
	}
	root, err := s.openProjectRoot(row)
	if err != nil {
		return Project{}, 0, nil, s.internalError("read layout", err)
	}
	defer root.Close()
	p, err := readLayout(root)
	if err != nil {
		return Project{}, 0, nil, s.internalError("read layout", err)
	}
	return p, row.Version, validateLayout(p), nil
}

// PutLayout saves a parsed layout when ifVersion equals the stored version
// (else CYD_PROJECT_CONFLICT with details.version). Semantic issues do not
// block saving; they are returned with the new version. The asset list must
// follow checkLayoutAssets (CYD_INVALID_LAYOUT with details.asset).
func (s *Service) PutLayout(ctx context.Context, id string, p Project, ifVersion int64, origin string) (int64, []Issue, error) {
	unlock := s.lockProject(id)
	defer unlock()
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return 0, nil, s.internalError("save layout", err)
	}
	if err := checkLock(row, origin); err != nil {
		return 0, nil, err
	}
	if ifVersion != row.Version {
		return 0, nil, NewError(CodeProjectConflict, "the layout was changed in the meantime",
			map[string]any{"version": row.Version})
	}
	if p.Format != ProjectFormat {
		return 0, nil, layoutError(fmt.Sprintf("unsupported format %d (want %d)", p.Format, ProjectFormat), nil)
	}
	root, err := s.openProjectRoot(row)
	if err != nil {
		return 0, nil, s.internalError("save layout", err)
	}
	defer root.Close()
	// A damaged stored layout lists no assets the save could keep, so a save
	// without assets can replace it.
	var storedAssets []Asset
	stored, err := readLayout(root)
	var typed *Error
	switch {
	case err == nil:
		storedAssets = stored.Assets
	case errors.As(err, &typed) && typed.Code == CodeInvalidLayout:
	default:
		return 0, nil, s.internalError("save layout", err)
	}
	if err := checkLayoutAssets(storedAssets, p.Assets); err != nil {
		return 0, nil, err
	}
	version, err := s.saveLayoutLocked(ctx, root, row, p)
	if err != nil {
		return 0, nil, s.internalError("save layout", err)
	}
	s.Emit(id, EventProjectChanged, jobIDOf(origin), "", projectChange(version, origin, "layout"))
	return version, validateLayout(p), nil
}

// jobIDOf returns the job id of a "job:<id>" origin, else "".
func jobIDOf(origin string) string {
	if len(origin) > len(originJobPrefix) && origin[:len(originJobPrefix)] == originJobPrefix {
		return origin[len(originJobPrefix):]
	}
	return ""
}
