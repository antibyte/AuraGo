package desktop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMovePathReceiptUndoRedoAndOccupiedTarget(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	if err := svc.WriteFile(ctx, "Desktop/a.txt", "original", SourceUser); err != nil {
		t.Fatal(err)
	}
	path, receipt, err := svc.MovePathWithReceipt(ctx, "Desktop/a.txt", "Documents/a.txt", SourceUser, nil)
	if err != nil || path != "Documents/a.txt" || receipt == "" {
		t.Fatalf("move: path=%q receipt=%q err=%v", path, receipt, err)
	}
	root := svc.Config().WorkspaceDir
	if err := os.WriteFile(filepath.Join(root, "Desktop/a.txt"), []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UndoFileMutation(ctx, receipt); err == nil {
		t.Fatal("undo replaced an occupied target")
	}
	if _, ok := svc.fileUndo[receipt]; !ok {
		t.Fatal("safe target conflict consumed receipt")
	}
	if err := os.Remove(filepath.Join(root, "Desktop/a.txt")); err != nil {
		t.Fatal(err)
	}
	undo, err := svc.UndoFileMutation(ctx, receipt)
	if err != nil || undo.From != "Documents/a.txt" || undo.Path != "Desktop/a.txt" || undo.UndoReceipt == "" {
		t.Fatalf("undo: %+v %v", undo, err)
	}
	data, err := os.ReadFile(filepath.Join(root, "Desktop/a.txt"))
	if err != nil || string(data) != "original" {
		t.Fatalf("undo data=%q err=%v", data, err)
	}
	redo, err := svc.UndoFileMutation(ctx, undo.UndoReceipt)
	if err != nil || redo.Path != "Documents/a.txt" || redo.UndoReceipt == "" {
		t.Fatalf("redo: %+v %v", redo, err)
	}
}

func TestFileUndoRejectsReplacementAndConsumesStaleReceipt(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	if err := svc.WriteFile(ctx, "Desktop/a.txt", "original", SourceUser); err != nil {
		t.Fatal(err)
	}
	_, receipt, err := svc.MovePathWithReceipt(ctx, "Desktop/a.txt", "Documents/a.txt", SourceUser, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(svc.Config().WorkspaceDir, "Documents/a.txt")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UndoFileMutation(ctx, receipt); !errors.Is(err, ErrFileUndoUnavailable) {
		t.Fatalf("replacement accepted: %v", err)
	}
	if _, ok := svc.fileUndo[receipt]; ok {
		t.Fatal("stale identity receipt was retained")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "replacement" {
		t.Fatalf("replacement changed: %q %v", data, err)
	}
}

func TestMovePathReceiptKeepsAuthorizedOverwriteIrreversible(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	for path, content := range map[string]string{"Desktop/source.txt": "source", "Documents/target.txt": "target"} {
		if err := svc.WriteFile(ctx, path, content, SourceUser); err != nil {
			t.Fatal(err)
		}
	}
	allowReplace := func(current FileWriteState) error {
		if !current.Exists {
			return errors.New("expected existing target")
		}
		return nil
	}
	path, receipt, err := svc.MovePathWithReceipt(ctx, "Desktop/source.txt", "Documents/target.txt", SourceUser, allowReplace)
	if err != nil || path != "Documents/target.txt" || receipt != "" {
		t.Fatalf("replace returned receipt: path=%q receipt=%q err=%v", path, receipt, err)
	}
	data, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, path))
	if err != nil || string(data) != "source" {
		t.Fatalf("replacement content=%q err=%v", data, err)
	}
}

func TestMovePathReceiptPreservesLegacySymlinkMoveWithoutReceipt(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	root := svc.Config().WorkspaceDir
	source := filepath.Join(root, "Desktop", "link")
	if err := os.Symlink("missing-target", source); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("creating a symlink requires Windows Developer Mode or privilege: %v", err)
		}
		t.Fatal(err)
	}
	path, receipt, err := svc.MovePathWithReceipt(ctx, "Desktop/link", "Documents/link", SourceUser, nil)
	if err != nil || path != "Documents/link" || receipt != "" {
		t.Fatalf("move symlink: path=%q receipt=%q err=%v", path, receipt, err)
	}
	info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("moved entry is not a symlink: %v %v", info, err)
	}
	target, err := os.Readlink(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil || !strings.EqualFold(target, "missing-target") {
		t.Fatalf("symlink target changed: %q %v", target, err)
	}
}

func TestFileUndoRetainsReceiptWhileReadonlyAndClosesOnExpiry(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	if err := svc.WriteFile(ctx, "Desktop/a.txt", "original", SourceUser); err != nil {
		t.Fatal(err)
	}
	_, receipt, err := svc.MovePathWithReceipt(ctx, "Desktop/a.txt", "Documents/a.txt", SourceUser, nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetReadOnly(true)
	if _, err := svc.UndoFileMutation(ctx, receipt); err == nil {
		t.Fatal("read-only undo succeeded")
	}
	if _, ok := svc.fileUndo[receipt]; !ok {
		t.Fatal("read-only check consumed a retryable receipt")
	}
	svc.SetReadOnly(false)

	svc.fileUndoMu.Lock()
	identity := svc.fileUndo[receipt].identity
	svc.fileUndo[receipt].expiresAt = time.Now().Add(-time.Second)
	svc.pruneFileUndoReceiptsLocked(time.Now())
	svc.fileUndoMu.Unlock()
	if _, err := svc.UndoFileMutation(ctx, receipt); !errors.Is(err, ErrFileUndoUnavailable) {
		t.Fatalf("expired receipt: %v", err)
	}
	if _, err := identity.Stat(); err == nil {
		t.Fatal("expired receipt identity handle was not closed")
	}
}

func TestTrashReceiptRestoresNotesOnlyThroughTrashService(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	if err := svc.WriteFile(ctx, "Documents/Notes/work/a.md", "note", SourceUser); err != nil {
		t.Fatal(err)
	}
	moves, err := svc.TrashPathsWithReceipts(ctx, []string{"Documents/Notes/work/a.md"}, false, nil)
	if err != nil || len(moves) != 1 || moves[0].UndoReceipt == "" {
		t.Fatalf("trash: %+v %v", moves, err)
	}
	if !strings.HasPrefix(moves[0].Path, NotesTrashDirectory+"/") {
		t.Fatalf("note left protected Trash namespace: %s", moves[0].Path)
	}
	result, err := svc.UndoFileMutation(ctx, moves[0].UndoReceipt)
	if err != nil || result.Path != "Documents/Notes/work/a.md" || result.UndoReceipt == "" {
		t.Fatalf("undo note: %+v %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(svc.Config().WorkspaceDir, "Documents/Notes/work/a.md")); err != nil {
		t.Fatal(err)
	}
	if err := svc.MovePath(ctx, result.Path, moves[0].Path, SourceUser); err == nil {
		t.Fatal("generic move bypassed Notes Trash service")
	}
}

func TestFileUndoBatchCapacityIsReservedBeforeMutation(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	paths := make([]string, fileUndoReceiptCapacity+1)
	for index := range paths {
		paths[index] = filepath.ToSlash(filepath.Join("Desktop", "item-"+string(rune('a'+index%26))+"-"+string(rune('A'+index/26))+".txt"))
		if err := svc.WriteFile(ctx, paths[index], "x", SourceUser); err != nil {
			t.Fatal(err)
		}
	}
	moves, err := svc.TrashPathsWithReceipts(ctx, paths, false, nil)
	if !errors.Is(err, ErrFileUndoLimit) || len(moves) != 0 {
		t.Fatalf("oversized batch moved items: moves=%d err=%v", len(moves), err)
	}
	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(svc.Config().WorkspaceDir, filepath.FromSlash(path))); err != nil {
			t.Fatalf("preflight moved %s: %v", path, err)
		}
	}
}

func TestFailedReceiptMoveDoesNotEvictExistingHistory(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	root := svc.Config().WorkspaceDir
	if err := svc.WriteFile(ctx, "Desktop/pin.txt", "pinned", SourceUser); err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFile(ctx, "Desktop/source.txt", "source", SourceUser); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	svc.fileUndoMu.Lock()
	for index := 0; index < fileUndoReceiptCapacity; index++ {
		identity, err := pinWorkspaceIdentity(root, "Desktop/pin.txt")
		if err != nil {
			svc.fileUndoMu.Unlock()
			t.Fatal(err)
		}
		token := fmt.Sprintf("preexisting-%d", index)
		svc.registerFileUndoReceiptLocked(token, &fileUndoReceipt{
			from: "Desktop/pin.txt", path: "Desktop/pin.txt", identity: identity,
			expiresAt: now.Add(fileUndoReceiptTTL), createdAt: now.Add(time.Duration(index) * time.Nanosecond),
		})
	}
	svc.fileUndoMu.Unlock()
	moveCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := svc.MovePathWithReceipt(moveCtx, "Desktop/source.txt", "Documents/source.txt", SourceUser, nil); err == nil {
		t.Fatal("cancelled move unexpectedly succeeded")
	}
	svc.fileUndoMu.Lock()
	defer svc.fileUndoMu.Unlock()
	if len(svc.fileUndo) != fileUndoReceiptCapacity {
		t.Fatalf("failed move changed receipt count: %d", len(svc.fileUndo))
	}
	for index := 0; index < fileUndoReceiptCapacity; index++ {
		if svc.fileUndo[fmt.Sprintf("preexisting-%d", index)] == nil {
			t.Fatalf("failed move evicted preexisting receipt %d", index)
		}
	}
}

func TestCloseDoesNotHoldServiceLockWhileDrainingUndoStore(t *testing.T) {
	svc := testService(t)
	desktopMutationMu.Lock()
	svc.fileUndoMu.Lock()
	closed := make(chan error, 1)
	go func() { closed <- svc.Close() }()
	desktopMutationMu.Unlock()

	configDone := make(chan struct{})
	go func() {
		defer close(configDone)
		_ = svc.Config()
	}()
	select {
	case <-configDone:
		// Config remains available while Close waits on the receipt-store lock.
	case <-time.After(2 * time.Second):
		svc.fileUndoMu.Unlock()
		<-closed
		t.Fatal("Close held Service.mu while waiting for fileUndoMu")
	}
	svc.fileUndoMu.Unlock()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not finish after receipt store was released")
	}
}

func TestReceiptMoveCannotMutateDuringClose(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	if err := svc.WriteFile(ctx, "Desktop/source.txt", "content", SourceUser); err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	closeStarted := make(chan struct{})
	closeResult := make(chan error, 1)
	go func() {
		close(closeStarted)
		closeResult <- svc.Close()
	}()
	<-closeStarted
	deadline := time.Now().Add(2 * time.Second)
	closeOwnsMutationLock := false
	for time.Now().Before(deadline) {
		if desktopMutationMu.TryLock() {
			desktopMutationMu.Unlock()
			runtime.Gosched()
			continue
		}
		closeOwnsMutationLock = true
		break
	}
	if !closeOwnsMutationLock {
		svc.mu.Unlock()
		<-closeResult
		t.Fatal("Close did not acquire desktopMutationMu before its service lock")
	}
	moveResult := make(chan error, 1)
	go func() {
		_, _, err := svc.MovePathWithReceipt(ctx, "Desktop/source.txt", "Documents/source.txt", SourceUser, nil)
		moveResult <- err
	}()
	svc.mu.Unlock()
	<-closeResult
	if err := <-moveResult; err == nil {
		t.Fatal("queued move ran after service close")
	}
	root := svc.Config().WorkspaceDir
	if _, err := os.Stat(filepath.Join(root, "Desktop/source.txt")); err != nil {
		t.Fatalf("source changed after Close: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Documents/source.txt")); !os.IsNotExist(err) {
		t.Fatalf("destination was mutated after Close: %v", err)
	}
}

func TestRenameDesktopPathNoReplacePreservesCollision(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("native atomic no-replace rename is unsupported on this platform")
	}

	ctx := context.Background()
	svc := testService(t)
	if err := svc.WriteFile(ctx, "Desktop/source.txt", "source bytes", SourceUser); err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFile(ctx, "Documents/target.txt", "target bytes", SourceUser); err != nil {
		t.Fatal(err)
	}

	workspace := svc.Config().WorkspaceDir
	root, err := os.OpenRoot(workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	identity, err := pinWorkspaceIdentity(workspace, "Desktop/source.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer identity.Close()

	type fileSnapshot struct {
		info os.FileInfo
		data string
	}
	snapshot := func(path string) fileSnapshot {
		t.Helper()
		file, err := root.Open(path)
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			t.Fatalf("stat %s: %v", path, err)
		}
		_ = file.Close()
		data, err := root.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return fileSnapshot{info: info, data: string(data)}
	}

	const sourcePath = "Desktop/source.txt"
	const targetPath = "Documents/target.txt"
	sourceBefore := snapshot(sourcePath)
	targetBefore := snapshot(targetPath)
	err = renameDesktopPathNoReplace(root, sourcePath, targetPath, identity)
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("collision error = %v, want fs.ErrExist", err)
	}
	sourceAfter := snapshot(sourcePath)
	targetAfter := snapshot(targetPath)
	if !os.SameFile(sourceBefore.info, sourceAfter.info) || sourceAfter.data != sourceBefore.data {
		t.Fatalf("source changed after collision: before=%q after=%q", sourceBefore.data, sourceAfter.data)
	}
	if !os.SameFile(targetBefore.info, targetAfter.info) || targetAfter.data != targetBefore.data {
		t.Fatalf("target changed after collision: before=%q after=%q", targetBefore.data, targetAfter.data)
	}
}
