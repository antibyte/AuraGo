package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type TrashResolution struct {
	Copy    bool   `json:"copy"`
	Version string `json:"version"`
}

type TrashMove struct {
	From        string `json:"from"`
	Path        string `json:"path"`
	UndoReceipt string `json:"undo_receipt,omitempty"`
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
	return s.trashPaths(ctx, paths, restore, resolutions, false)
}

// TrashPathsWithReceipts creates bounded, identity-pinned receipts for safe inverses.
func (s *Service) TrashPathsWithReceipts(ctx context.Context, paths []string, restore bool, resolutions map[string]TrashResolution) ([]TrashMove, error) {
	return s.trashPaths(ctx, paths, restore, resolutions, true)
}

func (s *Service) trashPaths(ctx context.Context, paths []string, restore bool, resolutions map[string]TrashResolution, withReceipts bool) ([]TrashMove, error) {
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
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if s.Config().ReadOnly {
		return nil, fmt.Errorf("virtual desktop is read-only")
	}
	root := s.Config().WorkspaceDir
	moves := make([]TrashMove, 0, len(paths))
	receiptNeeded := make([]bool, 0, len(paths))
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
		sourceInfo, err := os.Lstat(from)
		if err != nil {
			return nil, err
		}
		if withReceipts && !sourceInfo.IsDir() && !sourceInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("file type cannot be moved safely in a receipt-backed trash operation")
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
		_, targetErr := os.Lstat(to)
		targetExists := targetErr == nil
		if targetErr != nil && !os.IsNotExist(targetErr) {
			return nil, targetErr
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
		canPin := sourceInfo.IsDir() || sourceInfo.Mode().IsRegular()
		receiptNeeded = append(receiptNeeded, withReceipts && canPin && (!restore || resolution.Copy || !targetExists))
	}
	if withReceipts {
		receiptCount := 0
		for _, needed := range receiptNeeded {
			if needed {
				receiptCount++
			}
		}
		if receiptCount > fileUndoReceiptCapacity {
			return nil, ErrFileUndoLimit
		}
		identities := make([]*os.File, len(moves))
		tokens := make([]string, len(moves))
		for index, move := range moves {
			if !receiptNeeded[index] {
				continue
			}
			identity, err := pinWorkspaceIdentity(root, move.From)
			if err != nil {
				for _, opened := range identities {
					if opened != nil {
						_ = opened.Close()
					}
				}
				return nil, err
			}
			token, err := newFileUndoToken()
			if err != nil {
				_ = identity.Close()
				for _, opened := range identities {
					if opened != nil {
						_ = opened.Close()
					}
				}
				return nil, err
			}
			identities[index] = identity
			tokens[index] = token
		}
		completed := make([]TrashMove, 0, len(moves))
		committed := make(map[string]struct{}, receiptCount)
		for index, move := range moves {
			from := filepath.Join(root, filepath.FromSlash(move.From))
			to := filepath.Join(root, filepath.FromSlash(move.Path))
			var moveErr error
			if identities[index] != nil {
				moveErr = moveDesktopPathRootNoReplace(ctx, root, from, to, identities[index])
			} else {
				moveErr = moveDesktopPathRoot(ctx, root, from, to)
			}
			if moveErr != nil {
				if errors.Is(moveErr, ErrFileUndoUncertain) {
					s.invalidateBootstrapCacheForFileMutation(move.From, move.Path)
				}
				for _, remaining := range identities[index:] {
					if remaining != nil {
						_ = remaining.Close()
					}
				}
				if desktopNoReplaceCollision(moveErr) {
					return completed, &PathConflict{Path: move.Path}
				}
				return completed, moveErr
			}
			if identities[index] != nil {
				now := time.Now()
				s.fileUndoMu.Lock()
				roomErr := s.makeRoomForFileUndoReceiptLocked(committed, now)
				if roomErr == nil {
					s.registerFileUndoReceiptLocked(tokens[index], &fileUndoReceipt{
						from: move.From, path: move.Path, identity: identities[index],
						trashMove: true, expiresAt: now.Add(fileUndoReceiptTTL), createdAt: now,
					})
					move.UndoReceipt = tokens[index]
					committed[tokens[index]] = struct{}{}
					identities[index] = nil
				}
				s.fileUndoMu.Unlock()
				if roomErr != nil {
					s.invalidateBootstrapCacheForFileMutation(move.From, move.Path)
					for _, remaining := range identities[index:] {
						if remaining != nil {
							_ = remaining.Close()
						}
					}
					completed = append(completed, move)
					return completed, ErrFileUndoUncertain
				}
			}
			completed = append(completed, move)
			s.invalidateBootstrapCacheForFileMutation(move.From, move.Path)
			_ = s.Audit(ctx, "trash_move", move.From, map[string]interface{}{"new_path": move.Path, "restore": restore}, SourceUser)
		}
		return completed, nil
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

// undoTrashReceiptLocked keeps Notes Trash transitions inside the Trash service
// path and moves ordinary entries back to their recorded original locations.
// desktopMutationMu and fileUndoMu are held by UndoFileMutation.
func (s *Service) undoTrashReceiptLocked(ctx context.Context, token string, receipt *fileUndoReceipt, now time.Time) (FileUndoMove, error) {
	root := s.Config().WorkspaceDir
	current, err := s.resolveWorkspacePathNoSymlinks(receipt.path, false)
	if err != nil {
		s.discardFileUndoReceiptLocked(token)
		return FileUndoMove{}, ErrFileUndoUnavailable
	}
	target, err := s.resolveWorkspacePathNoSymlinks(receipt.from, true)
	if err != nil {
		s.discardFileUndoReceiptLocked(token)
		return FileUndoMove{}, ErrFileUndoUnavailable
	}
	fromInTrash, toInTrash := isDesktopTrashPath(receipt.path), isDesktopTrashPath(receipt.from)
	if fromInTrash == toInTrash {
		s.discardFileUndoReceiptLocked(token)
		return FileUndoMove{}, ErrFileUndoUnavailable
	}
	isNotesDocuments := func(value string) bool {
		lower := strings.ToLower(cleanDesktopPathSlash(value))
		return lower == "documents/notes" || strings.HasPrefix(lower, "documents/notes/")
	}
	isNotesTrash := func(value string) bool {
		lower := strings.ToLower(cleanDesktopPathSlash(value))
		return lower == "trash/notes" || strings.HasPrefix(lower, "trash/notes/")
	}
	currentNotes := isNotesDocuments(receipt.path) || isNotesTrash(receipt.path)
	targetNotes := isNotesDocuments(receipt.from) || isNotesTrash(receipt.from)
	if currentNotes || targetNotes {
		if !(isNotesDocuments(receipt.path) && isNotesTrash(receipt.from)) && !(isNotesTrash(receipt.path) && isNotesDocuments(receipt.from)) {
			s.discardFileUndoReceiptLocked(token)
			return FileUndoMove{}, ErrFileUndoUnavailable
		}
	}
	for _, candidate := range []string{current, target} {
		if err := s.guardNoteMutation(candidate, SourceUser); err != nil {
			return FileUndoMove{}, err
		}
	}
	if !workspacePathHasIdentity(root, receipt.path, receipt.identity) {
		s.discardFileUndoReceiptLocked(token)
		return FileUndoMove{}, ErrFileUndoUnavailable
	}
	if _, err := os.Lstat(target); err == nil {
		return FileUndoMove{}, &PathConflict{Path: receipt.from}
	} else if !os.IsNotExist(err) {
		return FileUndoMove{}, err
	}
	opposite, err := newFileUndoToken()
	if err != nil {
		return FileUndoMove{}, err
	}
	from, currentPath := receipt.from, receipt.path
	err = moveDesktopPathRootNoReplace(ctx, root, current, target, receipt.identity)
	if err != nil {
		if desktopNoReplaceCollision(err) {
			return FileUndoMove{}, &PathConflict{Path: receipt.from}
		}
		if workspacePathHasIdentity(root, from, receipt.identity) {
			s.rotateFileUndoReceiptLocked(token, opposite, receipt, now)
			s.invalidateBootstrapCacheForFileMutation(currentPath, from)
			_ = s.Audit(ctx, "trash_undo", currentPath, map[string]interface{}{"new_path": from}, SourceUser)
			return FileUndoMove{From: currentPath, Path: from, UndoReceipt: opposite}, nil
		}
		if workspacePathHasIdentity(root, currentPath, receipt.identity) {
			return FileUndoMove{}, err
		}
		s.discardFileUndoReceiptLocked(token)
		s.invalidateBootstrapCacheForFileMutation(currentPath, from)
		return FileUndoMove{}, ErrFileUndoUncertain
	}
	s.rotateFileUndoReceiptLocked(token, opposite, receipt, now)
	s.invalidateBootstrapCacheForFileMutation(currentPath, from)
	_ = s.Audit(ctx, "trash_undo", currentPath, map[string]interface{}{"new_path": from}, SourceUser)
	return FileUndoMove{From: currentPath, Path: from, UndoReceipt: opposite}, nil
}
