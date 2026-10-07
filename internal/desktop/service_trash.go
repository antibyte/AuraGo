package desktop

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type TrashResolution struct {
	Copy    bool   `json:"copy"`
	Version string `json:"version"`
}

type TrashMove struct {
	From string `json:"from"`
	Path string `json:"path"`
}

type PathConflict struct {
	Source    string
	Path      string
	Version   string
	Directory bool
}

func (e *PathConflict) Error() string { return "desktop destination exists or changed" }

func isDesktopTrashPath(p string) bool {
	p = strings.ToLower(cleanDesktopPathSlash(p))
	return p == "trash" || strings.HasPrefix(p, "trash/")
}

// TrashPaths preflights the whole selection under the mutation lock. Notes keep
// their protected namespace; no provenance is guessed for older ordinary trash.
func (s *Service) TrashPaths(ctx context.Context, paths []string, restore bool, resolutions map[string]TrashResolution) ([]TrashMove, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if s.Config().ReadOnly {
		return nil, fmt.Errorf("virtual desktop is read-only")
	}
	if len(paths) == 0 || len(paths) > 1000 {
		return nil, fmt.Errorf("invalid trash selection")
	}
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	defer s.invalidateListCache()
	root := s.Config().WorkspaceDir
	moves := make([]TrashMove, 0, len(paths))
	reserved := make(map[string]bool)
	for _, raw := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		from, err := s.resolveWorkspacePathNoSymlinks(raw, false)
		if err != nil {
			return nil, err
		}
		rel := s.relativePath(from)
		lower := strings.ToLower(rel)
		if lower == "." || lower == "documents" || lower == "documents/notes" || lower == "trash" || lower == "trash/notes" {
			return nil, fmt.Errorf("protected desktop directories cannot be moved to or from trash")
		}
		if _, err := os.Lstat(from); err != nil {
			return nil, err
		}
		for _, prior := range moves {
			a, b := strings.ToLower(prior.From)+"/", lower+"/"
			if strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
				return nil, fmt.Errorf("overlapping trash selection")
			}
		}
		var dest string
		if restore {
			if !strings.HasPrefix(lower, "trash/") {
				return nil, fmt.Errorf("restore requires a trash path")
			}
			dest = "Desktop/" + path.Base(rel)
			if strings.HasPrefix(lower, "trash/notes/") {
				parts := strings.SplitN(rel, "/", 4)
				if len(parts) != 4 {
					return nil, fmt.Errorf("select a note or folder within its trash entry")
				}
				if _, err := uuid.Parse(parts[2]); err != nil {
					return nil, fmt.Errorf("invalid notes trash entry")
				}
				dest = NotesDirectory + "/" + parts[3]
			}
		} else {
			if strings.HasPrefix(lower, "trash/") {
				return nil, fmt.Errorf("path is already in trash")
			}
			dest = "Trash/" + path.Base(rel)
			if strings.HasPrefix(lower, "documents/notes/") {
				dest = NotesTrashDirectory + "/" + uuid.NewString() + "/" + rel[len(NotesDirectory)+1:]
			}
		}
		resolution := resolutions[raw]
		if !restore || resolution.Copy {
			original := dest
			for i := 0; ; i++ {
				if i > 999 {
					return nil, fmt.Errorf("no free trash destination")
				}
				if i > 0 {
					ext := path.Ext(original)
					dest = strings.TrimSuffix(original, ext) + fmt.Sprintf(" (%d)", i) + ext
				}
				to, err := s.resolveWorkspacePathNoSymlinks(dest, true)
				if err != nil {
					return nil, err
				}
				_, err = os.Lstat(to)
				if os.IsNotExist(err) && !reserved[strings.ToLower(dest)] {
					break
				}
				if err != nil && !os.IsNotExist(err) {
					return nil, err
				}
			}
		}
		to, err := s.resolveWorkspacePathNoSymlinks(dest, true)
		if err != nil {
			return nil, err
		}
		if restore {
			err = s.checkPathTargetLocked(to, dest, func(current FileWriteState) error {
				if !current.Exists && resolution.Version == "" {
					return nil
				}
				version := ""
				if current.Exists && current.Entry.Type != "directory" {
					version = NoteVersion(current.Data)
				}
				if version != "" && version == resolution.Version {
					return nil
				}
				return &PathConflict{Source: raw, Path: dest, Version: version, Directory: current.Entry.Type == "directory"}
			})
			if err != nil {
				return nil, err
			}
		}
		if reserved[strings.ToLower(dest)] {
			return nil, fmt.Errorf("duplicate restore destination")
		}
		reserved[strings.ToLower(dest)] = true
		moves = append(moves, TrashMove{From: rel, Path: dest})
	}
	completed := make([]TrashMove, 0, len(moves))
	for _, move := range moves {
		if err := moveDesktopPathRoot(ctx, root, filepath.Join(root, filepath.FromSlash(move.From)), filepath.Join(root, filepath.FromSlash(move.Path))); err != nil {
			return completed, err
		}
		completed = append(completed, move)
		s.invalidateBootstrapCacheForFileMutation(move.From, move.Path)
		_ = s.Audit(ctx, "trash_move", move.From, map[string]interface{}{"new_path": move.Path, "restore": restore}, SourceUser)
	}
	return completed, nil
}
