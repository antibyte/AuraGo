package ocr

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func id() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("OCR random source unavailable")
	}
	return hex.EncodeToString(b[:])
}

func extensionMIME(name string) (string, bool) {
	m, ok := map[string]string{".pdf": "application/pdf", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
		".tif": "image/tiff", ".tiff": "image/tiff", ".webp": "image/webp", ".bmp": "image/bmp"}[strings.ToLower(filepath.Ext(name))]
	return m, ok
}

func sniff(name string, prefix []byte) (string, error) {
	want, ok := extensionMIME(name)
	if !ok {
		return "", ErrInvalid
	}
	got := http.DetectContentType(prefix)
	if bytes.HasPrefix(prefix, []byte("%PDF-")) {
		got = "application/pdf"
	}
	if len(prefix) >= 4 && (bytes.Equal(prefix[:4], []byte{'I', 'I', 42, 0}) || bytes.Equal(prefix[:4], []byte{'M', 'M', 0, 42})) {
		got = "image/tiff"
	}
	if got != want {
		return "", fmt.Errorf("%w: content does not match extension", ErrInvalid)
	}
	return want, nil
}

// Stage reads an already-authorized source into a private, bounded snapshot.
func (m *Manager) Stage(owner, name string, source io.Reader) (File, error) {
	return m.StageBounded(owner, name, source, MaxFileBytes)
}

// StageBounded also enforces a caller's stricter existing ingress limit.
func (m *Manager) StageBounded(owner, name string, source io.Reader, maxBytes int64) (File, error) {
	if maxBytes <= 0 {
		return File{}, ErrLimit
	}
	if owner == "" || len(owner) > 512 || !Supported(name) {
		return File{}, ErrInvalid
	}
	m.mu.Lock()
	locked := true
	defer func() {
		if locked {
			m.mu.Unlock()
		}
	}()
	m.pruneLocked()
	if m.closed || !m.policy.Enabled {
		return File{}, ErrDisabled
	}
	if !m.status.Ready {
		return File{}, ErrNotReady
	}
	if len(m.uploads) >= 100 || m.storage >= MaxStorageBytes {
		return File{}, ErrLimit
	}
	if err := os.MkdirAll(m.root, 0700); err != nil {
		return File{}, err
	}
	key := id()
	path := filepath.Join(m.root, key+strings.ToLower(filepath.Ext(name)))
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return File{}, err
	}
	limit := min(MaxFileBytes, maxBytes, MaxStorageBytes-m.storage)
	m.storage += limit
	generation := m.generation
	m.mu.Unlock()
	locked = false
	digest := sha256.New()
	source = io.TeeReader(source, digest)
	var prefix bytes.Buffer
	reader := io.TeeReader(io.LimitReader(source, min(int64(512), limit+1)), &prefix)
	_, err = io.Copy(out, reader)
	var remaining int64
	if err == nil {
		remaining, err = io.Copy(out, io.LimitReader(source, limit-int64(prefix.Len())+1))
	}
	closeErr := out.Close()
	size := int64(prefix.Len()) + remaining
	m.mu.Lock()
	locked = true
	m.storage -= limit
	if generation != m.generation || m.closed || !m.policy.Enabled {
		_ = os.Remove(path)
		return File{}, ErrDisabled
	}
	mime, validErr := sniff(name, prefix.Bytes())
	if err != nil || closeErr != nil || size > limit || size == 0 || validErr != nil {
		_ = os.Remove(path)
		if size > limit {
			return File{}, ErrLimit
		}
		if err != nil {
			return File{}, err
		}
		if closeErr != nil {
			return File{}, closeErr
		}
		return File{}, ErrInvalid
	}
	file := File{ID: key, Name: filepath.Base(strings.ReplaceAll(name, "\\", "/")), MIME: mime, Size: size, State: "uploaded", path: path}
	file.digest = hex.EncodeToString(digest.Sum(nil))
	m.uploads[key] = upload{file: file, owner: owner, created: m.now()}
	m.storage += size
	return file, nil
}
