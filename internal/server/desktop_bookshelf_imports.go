package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"aurago/internal/bookshelf"
)

type bookshelfImportHash struct {
	info     os.FileInfo
	revision string
}

// Managed imports retain the SHA-256 revision recorded at upload. Cache it
// against file identity and metadata so PDF range requests do not rehash books.
func (a *desktopBookshelfAdapter) importEntry(ctx context.Context, entry bookshelf.Entry) (bookshelf.Entry, error) {
	if _, err := a.checkBookshelfEntry(entry, nil); err != nil {
		return bookshelf.Entry{}, err
	}
	file, info, err := openRegularFileInRoot(a.importsDir, entry.Path)
	if err != nil {
		return bookshelf.Entry{}, fmt.Errorf("open bookshelf import fingerprint: %w", err)
	}
	defer file.Close()
	if info.Size() != entry.Size || localFileRevision(info.Size(), info.ModTime()) != entry.Revision {
		return bookshelf.Entry{}, bookshelf.ErrStaleRendition
	}
	a.importMu.Lock()
	cached, ok := a.importHashes[entry.Path]
	a.importMu.Unlock()
	if ok && os.SameFile(cached.info, info) && cached.info.Size() == info.Size() && cached.info.ModTime().Equal(info.ModTime()) {
		entry.Revision = cached.revision
		return entry, nil
	}
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(storageContextReader{ctx: ctx, Reader: file}, entry.Size+1))
	if err != nil {
		return bookshelf.Entry{}, fmt.Errorf("hash bookshelf import: %w", err)
	}
	after, err := file.Stat()
	if err != nil {
		return bookshelf.Entry{}, fmt.Errorf("recheck bookshelf import fingerprint: %w", err)
	}
	if written != entry.Size || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return bookshelf.Entry{}, bookshelf.ErrStaleRendition
	}
	entry.Revision = "sha256:" + hex.EncodeToString(hash.Sum(nil))
	a.importMu.Lock()
	if a.importHashes == nil {
		a.importHashes = make(map[string]bookshelfImportHash)
	} else if _, exists := a.importHashes[entry.Path]; !exists && len(a.importHashes) >= 256 {
		for name := range a.importHashes {
			delete(a.importHashes, name)
			break
		}
	}
	a.importHashes[entry.Path] = bookshelfImportHash{info: info, revision: entry.Revision}
	a.importMu.Unlock()
	return entry, nil
}
