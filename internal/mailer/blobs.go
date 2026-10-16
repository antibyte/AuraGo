package mailer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"aurago/internal/fileutil"
)

const (
	defaultAttachmentBytes      = int64(25 << 20)
	defaultAttachmentTotalBytes = int64(256 << 20)
)

type BlobStore struct {
	root     string
	store    *Store
	maxFile  int64
	maxTotal int64
	mu       sync.Mutex
}

func OpenBlobStore(root string, store *Store, maxFile, maxTotal int64) (*BlobStore, error) {
	if strings.TrimSpace(root) == "" || store == nil {
		return nil, errors.New("mailer attachment root and store are required")
	}
	if maxFile <= 0 {
		maxFile = defaultAttachmentBytes
	}
	if maxTotal <= 0 {
		maxTotal = defaultAttachmentTotalBytes
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create private mail attachment root: %w", err)
	}
	if info, err := os.Lstat(root); err != nil {
		return nil, err
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("mail attachment root must be a real directory")
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, fmt.Errorf("restrict mail attachment root: %w", err)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &BlobStore{root: abs, store: store, maxFile: maxFile, maxTotal: maxTotal}, nil
}

func (b *BlobStore) Put(ctx context.Context, ownerKind, ownerID, filename, contentType string, src io.Reader) (Attachment, error) {
	if ownerKind == "" || ownerID == "" {
		return Attachment{}, errors.New("attachment owner is required")
	}
	if src == nil {
		return Attachment{}, errors.New("attachment data is required")
	}
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "" || filename == "." {
		filename = "attachment"
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(filename))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	contentType = strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	data, err := io.ReadAll(io.LimitReader(src, b.maxFile+1))
	if err != nil {
		return Attachment{}, fmt.Errorf("read mail attachment: %w", err)
	}
	if int64(len(data)) > b.maxFile {
		return Attachment{}, fmt.Errorf("mail attachment exceeds %d-byte limit", b.maxFile)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var used int64
	if err := b.store.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(size),0) FROM mailer_blobs").Scan(&used); err != nil {
		return Attachment{}, fmt.Errorf("check mail attachment quota: %w", err)
	}
	if used+int64(len(data)) > b.maxTotal {
		return Attachment{}, fmt.Errorf("mail attachments exceed %d-byte storage limit", b.maxTotal)
	}
	id, err := newID("blob_")
	if err != nil {
		return Attachment{}, err
	}
	final := filepath.Join(b.root, id+".blob")
	tmp, err := os.CreateTemp(b.root, ".upload-*.tmp")
	if err != nil {
		return Attachment{}, fmt.Errorf("create mail attachment: %w", err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return Attachment{}, err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return Attachment{}, fmt.Errorf("write mail attachment: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return Attachment{}, fmt.Errorf("sync mail attachment: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return Attachment{}, fmt.Errorf("close mail attachment: %w", err)
	}
	if err := fileutil.RenameContext(ctx, tmpName, final); err != nil {
		return Attachment{}, fmt.Errorf("publish mail attachment: %w", err)
	}
	sum := sha256.Sum256(data)
	a := Attachment{ID: id, Filename: filename, ContentType: contentType, Size: int64(len(data))}
	if err := b.store.SetBlob(ctx, a, hex.EncodeToString(sum[:]), ownerKind, ownerID); err != nil {
		_ = os.Remove(final)
		return Attachment{}, err
	}
	ok = true
	return a, nil
}

func (b *BlobStore) Open(ctx context.Context, id string) (Attachment, *os.File, error) {
	if !validBlobID(id) {
		return Attachment{}, nil, ErrNotFound
	}
	a, _, err := b.store.Blob(ctx, id)
	if err != nil {
		return Attachment{}, nil, err
	}
	f, err := os.Open(filepath.Join(b.root, id+".blob"))
	if err != nil {
		return Attachment{}, nil, fmt.Errorf("open mail attachment: %w", err)
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != a.Size {
		_ = f.Close()
		if err != nil {
			return Attachment{}, nil, err
		}
		return Attachment{}, nil, errors.New("mail attachment file does not match its index")
	}
	return a, f, nil
}

func (b *BlobStore) Remove(ctx context.Context, id string) error {
	if !validBlobID(id) {
		return ErrNotFound
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	a, _, err := b.store.Blob(ctx, id)
	if err != nil {
		return err
	}
	_ = a
	path := filepath.Join(b.root, id+".blob")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove mail attachment: %w", err)
	}
	return b.store.DeleteBlob(ctx, id)
}

func validBlobID(id string) bool {
	return strings.HasPrefix(id, "blob_") && !strings.ContainsAny(id, "/\\") && filepath.Base(id) == id
}
