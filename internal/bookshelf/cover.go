package bookshelf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aurago/internal/fileutil"
)

const maxCoverBytes = 12 << 20
const maxCoverStorageBytes = 256 << 20

func (s *Service) SetCover(ctx context.Context, id string, expectedRevision int64, mime string, body io.Reader) (Book, error) {
	if err := s.checkOpen(); err != nil {
		return Book{}, err
	}
	if !validID(id) || body == nil {
		return Book{}, fmt.Errorf("invalid bookshelf cover")
	}
	data, err := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, r: body}, maxCoverBytes+1))
	if err != nil {
		return Book{}, fmt.Errorf("read bookshelf cover: %w", err)
	}
	if len(data) == 0 || len(data) > maxCoverBytes {
		return Book{}, ErrLimitExceeded
	}
	mime = strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || int64(config.Width)*int64(config.Height) > 16_000_000 || config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 {
		return Book{}, fmt.Errorf("invalid bookshelf cover image")
	}
	if _, decodedFormat, err := image.Decode(bytes.NewReader(data)); err != nil || decodedFormat != format {
		return Book{}, fmt.Errorf("invalid bookshelf cover image")
	}
	ext := ""
	switch {
	case mime == "image/png" && format == "png":
		ext = ".png"
	case (mime == "image/jpeg" || mime == "image/jpg") && format == "jpeg":
		ext = ".jpg"
	default:
		return Book{}, fmt.Errorf("unsupported bookshelf cover image")
	}
	hash := sha256.Sum256(data)
	filename := id + "-" + hex.EncodeToString(hash[:12]) + ext
	coverPath := filepath.Join(s.coversDir, filename)
	s.coverMu.Lock()
	defer s.coverMu.Unlock()
	if _, err := os.Stat(coverPath); errors.Is(err, os.ErrNotExist) {
		if err := s.makeCoverRoom(int64(len(data))); err != nil {
			return Book{}, err
		}
		if err := s.freeSpaceFor(int64(len(data))); err != nil {
			return Book{}, err
		}
		if err := fileutil.WriteFileContext(ctx, coverPath, data, 0o600); err != nil {
			return Book{}, fmt.Errorf("publish bookshelf cover: %w", err)
		}
	} else if err != nil {
		return Book{}, fmt.Errorf("check bookshelf cover cache: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE books SET cover_file=?,revision=revision+1 WHERE id=? AND revision=?`, filename, id, expectedRevision)
	if err != nil {
		return Book{}, fmt.Errorf("save bookshelf cover: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Book{}, checkRevision(ctx, s.db, id)
	}
	return s.GetBook(ctx, id)
}

func (s *Service) makeCoverRoom(need int64) error {
	if need > maxCoverStorageBytes {
		return ErrLimitExceeded
	}
	rows, err := s.db.Query(`SELECT cover_file FROM books WHERE cover_file<>''`)
	if err != nil {
		return fmt.Errorf("read bookshelf cover references: %w", err)
	}
	referenced := make(map[string]struct{})
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			rows.Close()
			return err
		}
		referenced[filename] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	entries, err := os.ReadDir(s.coversDir)
	if err != nil {
		return fmt.Errorf("read bookshelf cover storage: %w", err)
	}
	type candidate struct {
		path     string
		size     int64
		modified time.Time
	}
	var total int64
	var orphaned []candidate
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(s.coversDir, entry.Name())
		total += info.Size()
		if _, ok := referenced[entry.Name()]; !ok {
			orphaned = append(orphaned, candidate{path: path, size: info.Size(), modified: info.ModTime()})
		}
	}
	sort.Slice(orphaned, func(i, j int) bool { return orphaned[i].modified.Before(orphaned[j].modified) })
	for _, file := range orphaned {
		if total+need <= maxCoverStorageBytes {
			break
		}
		if err := os.Remove(file.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			continue
		}
		total -= file.size
	}
	if total+need > maxCoverStorageBytes {
		return ErrLimitExceeded
	}
	return nil
}

func (s *Service) ClearCover(ctx context.Context, id string, expectedRevision int64) (Book, error) {
	if err := s.checkOpen(); err != nil {
		return Book{}, err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE books SET cover_file='',revision=revision+1 WHERE id=? AND revision=?`, id, expectedRevision)
	if err != nil {
		return Book{}, fmt.Errorf("clear bookshelf cover: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Book{}, checkRevision(ctx, s.db, id)
	}
	return s.GetBook(ctx, id)
}

func (s *Service) OpenCover(ctx context.Context, id string) (*os.File, string, error) {
	if err := s.checkOpen(); err != nil {
		return nil, "", err
	}
	var filename string
	err := s.db.QueryRowContext(ctx, `SELECT cover_file FROM books WHERE id=?`, id).Scan(&filename)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("read bookshelf cover reference: %w", err)
	}
	if filename == "" {
		return nil, "", ErrNotFound
	}
	if filepath.Base(filename) != filename {
		return nil, "", ErrNotFound
	}
	file, err := os.Open(filepath.Join(s.coversDir, filename))
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("open bookshelf cover: %w", err)
	}
	mime := "image/png"
	if strings.EqualFold(filepath.Ext(filename), ".jpg") {
		mime = "image/jpeg"
	}
	return file, mime, nil
}
