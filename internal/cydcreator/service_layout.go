package cydcreator

import (
	"context"
	"fmt"
	"os"
)

// readLayout reads and strictly parses cyd.project.json from a project tree.
func readLayout(root *os.Root) (Project, error) {
	data, err := rootReadFile(root, ProjectFile, MaxLayoutBytes+1)
	if err != nil {
		return Project{}, fmt.Errorf("read %s: %w", ProjectFile, err)
	}
	return ParseProject(data)
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

// saveLayoutLocked writes the canonical layout, bumps the version and mirrors
// name, board and theme into the row. Callers hold the project lock.
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
	version := row.Version + 1
	if _, err := s.db.ExecContext(ctx, `UPDATE cc_projects SET version=?, name=?, board=?, theme=?, updated_at=? WHERE id=?`,
		version, p.Name, p.Board, p.Theme, formatTime(s.now()), row.ID); err != nil {
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
	root, err := s.openRoot(row, "")
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
// block saving; they are returned with the new version.
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
	root, err := s.openRoot(row, "")
	if err != nil {
		return 0, nil, s.internalError("save layout", err)
	}
	defer root.Close()
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
