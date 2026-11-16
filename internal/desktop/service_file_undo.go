package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	fileUndoReceiptTTL      = 30 * time.Minute
	fileUndoReceiptCapacity = 256
)

var (
	ErrFileUndoUnavailable = errors.New("file undo receipt is unavailable")
	ErrFileUndoLimit       = errors.New("file undo receipt capacity exceeded")
	ErrFileUndoUncertain   = errors.New("file operation outcome is uncertain; automatic retry is unsafe")
)

type fileUndoReceipt struct {
	from      string
	path      string
	identity  *os.File
	expiresAt time.Time
	createdAt time.Time
	trashMove bool
	timer     *time.Timer
}

// FileUndoMove describes a completed move and its one-use inverse receipt.
type FileUndoMove struct {
	From        string `json:"from"`
	Path        string `json:"path"`
	UndoReceipt string `json:"undo_receipt,omitempty"`
}

func (s *Service) fileUndoStoreLocked() {
	if s.fileUndo == nil {
		s.fileUndo = make(map[string]*fileUndoReceipt)
	}
}

func (s *Service) pruneFileUndoReceiptsLocked(now time.Time) {
	for token, receipt := range s.fileUndo {
		if receipt == nil || !receipt.expiresAt.After(now) {
			s.discardFileUndoReceiptLocked(token)
		}
	}
}

func (s *Service) makeRoomForFileUndoReceiptLocked(protected map[string]struct{}, now time.Time) error {
	s.fileUndoStoreLocked()
	s.pruneFileUndoReceiptsLocked(now)
	if len(s.fileUndo) < fileUndoReceiptCapacity {
		return nil
	}
	oldestToken := ""
	var oldest time.Time
	for token, receipt := range s.fileUndo {
		if _, keep := protected[token]; keep {
			continue
		}
		if receipt == nil || oldestToken == "" || receipt.createdAt.Before(oldest) {
			oldestToken, oldest = token, receipt.createdAt
		}
	}
	if oldestToken == "" {
		return ErrFileUndoLimit
	}
	s.discardFileUndoReceiptLocked(oldestToken)
	return nil
}

func newFileUndoToken() (string, error) {
	token, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("create file undo receipt: %w", err)
	}
	return token.String(), nil
}

func (s *Service) closeFileUndoReceipts() {
	s.fileUndoMu.Lock()
	defer s.fileUndoMu.Unlock()
	for token := range s.fileUndo {
		s.discardFileUndoReceiptLocked(token)
	}
}

func (s *Service) registerFileUndoReceiptLocked(token string, receipt *fileUndoReceipt) {
	s.fileUndoStoreLocked()
	receipt.timer = time.AfterFunc(time.Until(receipt.expiresAt), func() {
		s.expireFileUndoReceipt(token, receipt)
	})
	s.fileUndo[token] = receipt
}

func (s *Service) expireFileUndoReceipt(token string, expected *fileUndoReceipt) {
	s.fileUndoMu.Lock()
	defer s.fileUndoMu.Unlock()
	if s.fileUndo[token] != expected {
		return
	}
	if remaining := time.Until(expected.expiresAt); remaining > 0 {
		expected.timer = time.AfterFunc(remaining, func() { s.expireFileUndoReceipt(token, expected) })
		return
	}
	s.discardFileUndoReceiptLocked(token)
}

// MovePathWithReceipt moves a workspace path and returns a one-use receipt
// when the destination was free and the source identity could be pinned.
// Authorized replacement remains possible, but is deliberately not reversible.
func (s *Service) MovePathWithReceipt(ctx context.Context, oldPath, newPath, source string, target FileWritePrecondition) (string, string, error) {
	oldRel, newRel := cleanDesktopPathSlash(oldPath), cleanDesktopPathSlash(newPath)
	if source == SourceUser && (NotesPath(oldRel, true) || NotesPath(newRel, true)) && (isDesktopTrashPath(oldRel) || isDesktopTrashPath(newRel)) {
		return "", "", fmt.Errorf("notes trash transitions require the desktop trash operation")
	}
	if _, _, _, fromMount, err := s.resolveMediaMount(oldPath); fromMount || err != nil {
		if err != nil {
			return "", "", err
		}
		if err := s.MovePathTo(ctx, oldPath, newPath, source, target); err != nil {
			return "", "", err
		}
		return newRel, "", nil
	}
	if _, _, _, toMount, err := s.resolveMediaMount(newPath); err != nil {
		return "", "", err
	} else if toMount {
		return "", "", fmt.Errorf("desktop workspace paths cannot move into media mounts")
	}
	if err := s.ensureReady(ctx); err != nil {
		return "", "", err
	}
	if s.Config().ReadOnly {
		return "", "", fmt.Errorf("virtual desktop is read-only")
	}
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	defer s.invalidateListCache()
	if err := s.ensureReady(ctx); err != nil {
		return "", "", err
	}
	if s.Config().ReadOnly {
		return "", "", fmt.Errorf("virtual desktop is read-only")
	}
	from, err := s.resolveRenamePath(oldPath)
	if err != nil {
		return "", "", err
	}
	to, err := s.resolveWorkspaceRenameDestinationNoSymlinkParent(newPath)
	if err != nil {
		return "", "", err
	}
	for _, candidate := range []string{from, to} {
		if err := s.guardNoteMutation(candidate, source); err != nil {
			return "", "", err
		}
	}
	if strings.EqualFold(from, to) {
		return s.relativePath(to), "", nil
	}
	if err := s.checkPathTargetLocked(to, newPath, target); err != nil {
		return "", "", err
	}
	rootAbs, err := filepath.Abs(s.Config().WorkspaceDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve desktop root: %w", err)
	}
	toDirAbs, err := filepath.Abs(filepath.Dir(to))
	if err != nil {
		return "", "", fmt.Errorf("resolve desktop destination directory: %w", err)
	}
	if err := validateNoSymlinkComponents(rootAbs, toDirAbs, true); err != nil {
		return "", "", err
	}

	toRel := s.relativePath(to)
	fromRel := s.relativePath(from)
	sourceInfo, err := os.Lstat(from)
	if err != nil {
		return "", "", err
	}
	_, statErr := os.Lstat(to)
	freeDestination := os.IsNotExist(statErr)
	if statErr != nil && !freeDestination {
		return "", "", statErr
	}
	receiptEligible := freeDestination && (sourceInfo.IsDir() || sourceInfo.Mode().IsRegular()) && sourceInfo.Mode()&os.ModeSymlink == 0
	var token string
	var identity *os.File
	if receiptEligible {
		token, err = newFileUndoToken()
		if err != nil {
			return "", "", err
		}
		identity, err = pinWorkspaceIdentity(rootAbs, fromRel)
		if err != nil {
			return "", "", err
		}
		defer func() {
			if identity != nil {
				_ = identity.Close()
			}
		}()
	}
	if receiptEligible {
		err = moveDesktopPathRootNoReplace(ctx, rootAbs, from, to, identity)
	} else if freeDestination && sourceInfo.Mode()&os.ModeSymlink != 0 {
		err = moveDesktopSymlinkRootNoReplace(ctx, rootAbs, from, to)
	} else if freeDestination {
		return "", "", fmt.Errorf("desktop file type cannot be moved safely without a receipt")
	} else {
		err = moveDesktopPathRoot(ctx, rootAbs, from, to)
	}
	if err != nil {
		if desktopNoReplaceCollision(err) {
			return "", "", &PathConflict{Path: newPath}
		}
		if errors.Is(err, ErrFileUndoUncertain) {
			s.invalidateBootstrapCacheForFileMutation(fromRel, toRel)
		}
		return "", "", fmt.Errorf("move desktop path: %w", err)
	}
	if receiptEligible {
		s.fileUndoMu.Lock()
		if err := s.makeRoomForFileUndoReceiptLocked(nil, time.Now()); err != nil {
			s.fileUndoMu.Unlock()
			s.invalidateBootstrapCacheForFileMutation(fromRel, toRel)
			return "", "", ErrFileUndoUncertain
		}
		now := time.Now()
		s.registerFileUndoReceiptLocked(token, &fileUndoReceipt{from: fromRel, path: toRel, identity: identity, expiresAt: now.Add(fileUndoReceiptTTL), createdAt: now})
		s.fileUndoMu.Unlock()
		identity = nil
	}
	_ = s.Audit(ctx, "move_path", fromRel, map[string]interface{}{"new_path": toRel}, source)
	s.invalidateBootstrapCacheForFileMutation(fromRel, toRel)
	return toRel, token, nil
}

// UndoFileMutation consumes one receipt and returns the inverse receipt for redo.
func (s *Service) UndoFileMutation(ctx context.Context, token string) (FileUndoMove, error) {
	if token == "" || len(token) > 80 {
		return FileUndoMove{}, ErrFileUndoUnavailable
	}
	if err := s.ensureReady(ctx); err != nil {
		return FileUndoMove{}, err
	}
	if s.Config().ReadOnly {
		return FileUndoMove{}, fmt.Errorf("virtual desktop is read-only")
	}
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	defer s.invalidateListCache()
	if err := s.ensureReady(ctx); err != nil {
		return FileUndoMove{}, err
	}
	if s.Config().ReadOnly {
		return FileUndoMove{}, fmt.Errorf("virtual desktop is read-only")
	}
	s.fileUndoMu.Lock()
	defer s.fileUndoMu.Unlock()
	now := time.Now()
	s.fileUndoStoreLocked()
	s.pruneFileUndoReceiptsLocked(now)
	receipt := s.fileUndo[token]
	if receipt == nil {
		return FileUndoMove{}, ErrFileUndoUnavailable
	}
	if receipt.trashMove {
		return s.undoTrashReceiptLocked(ctx, token, receipt, now)
	}
	return s.undoWorkspaceReceiptLocked(ctx, token, receipt, now)
}

func (s *Service) undoWorkspaceReceiptLocked(ctx context.Context, token string, receipt *fileUndoReceipt, now time.Time) (FileUndoMove, error) {
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
	err = moveDesktopPathRootNoReplace(ctx, root, current, target, receipt.identity)
	from, currentPath := receipt.from, receipt.path
	if err != nil {
		if desktopNoReplaceCollision(err) {
			return FileUndoMove{}, &PathConflict{Path: receipt.from}
		}
		if workspacePathHasIdentity(root, receipt.from, receipt.identity) {
			s.rotateFileUndoReceiptLocked(token, opposite, receipt, now)
			s.invalidateBootstrapCacheForFileMutation(currentPath, from)
			_ = s.Audit(ctx, "file_undo", currentPath, map[string]interface{}{"new_path": from}, SourceUser)
			return FileUndoMove{From: currentPath, Path: from, UndoReceipt: opposite}, nil
		}
		if workspacePathHasIdentity(root, receipt.path, receipt.identity) {
			return FileUndoMove{}, err
		}
		s.discardFileUndoReceiptLocked(token)
		s.invalidateBootstrapCacheForFileMutation(currentPath, from)
		return FileUndoMove{}, ErrFileUndoUncertain
	}
	s.rotateFileUndoReceiptLocked(token, opposite, receipt, now)
	s.invalidateBootstrapCacheForFileMutation(currentPath, from)
	_ = s.Audit(ctx, "file_undo", currentPath, map[string]interface{}{"new_path": from}, SourceUser)
	return FileUndoMove{From: currentPath, Path: from, UndoReceipt: opposite}, nil
}

func (s *Service) rotateFileUndoReceiptLocked(oldToken, newToken string, receipt *fileUndoReceipt, now time.Time) {
	delete(s.fileUndo, oldToken)
	if receipt.timer != nil {
		receipt.timer.Stop()
	}
	receipt.from, receipt.path = receipt.path, receipt.from
	receipt.expiresAt = now.Add(fileUndoReceiptTTL)
	receipt.createdAt = now
	s.registerFileUndoReceiptLocked(newToken, receipt)
}

func (s *Service) discardFileUndoReceiptLocked(token string) {
	receipt := s.fileUndo[token]
	if receipt != nil {
		if receipt.timer != nil {
			receipt.timer.Stop()
		}
		if receipt.identity != nil {
			_ = receipt.identity.Close()
		}
	}
	delete(s.fileUndo, token)
}

func pinWorkspaceIdentity(rootPath, relative string) (*os.File, error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(filepath.FromSlash(relative))
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return nil, fmt.Errorf("file undo cannot pin links or special files")
	}
	file, err := root.Open(filepath.FromSlash(relative))
	if err != nil {
		return nil, err
	}
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openedInfo) {
		_ = file.Close()
		if err != nil {
			return nil, err
		}
		return nil, ErrFileUndoUnavailable
	}
	return file, nil
}

func workspacePathHasIdentity(rootPath, relative string, identity *os.File) bool {
	if identity == nil || strings.TrimSpace(relative) == "" {
		return false
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return false
	}
	defer root.Close()
	info, err := root.Lstat(filepath.FromSlash(relative))
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	file, err := root.Open(filepath.FromSlash(relative))
	if err != nil {
		return false
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return false
	}
	pinnedInfo, err := identity.Stat()
	return err == nil && os.SameFile(openedInfo, pinnedInfo)
}
