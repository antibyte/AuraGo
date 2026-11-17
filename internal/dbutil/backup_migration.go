package dbutil

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/fileutil"
)

func migrationBackupHash(ctx context.Context, file *os.File) ([sha256.Size]byte, error) {
	hash := sha256.New()
	buffer := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return [sha256.Size]byte{}, err
		}
		n, err := file.Read(buffer)
		if n > 0 {
			_, _ = hash.Write(buffer[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return [sha256.Size]byte{}, err
		}
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest, ctx.Err()
}

func publishMigrationBackup(ctx context.Context, temporary, destination string) (string, error) {
	// shortcut: retries still prepare a consistent temporary copy; add source fingerprinting if startup I/O becomes material.
	file, err := os.Open(temporary)
	if err != nil {
		return "", err
	}
	digest, hashErr := migrationBackupHash(ctx, file)
	info, statErr := file.Stat()
	closeErr := file.Close()
	if hashErr != nil {
		return "", fmt.Errorf("hash migration backup: %w", hashErr)
	}
	if statErr != nil {
		return "", statErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	ext := filepath.Ext(destination)
	destination = fmt.Sprintf("%s-%x%s", strings.TrimSuffix(destination, ext), digest, ext)
	for {
		if _, err := os.Lstat(destination); err == nil {
			if err := verifyMigrationBackup(ctx, destination, digest, info.Size()); err != nil {
				return "", fmt.Errorf("existing migration backup is not reusable: %w", err)
			}
			if err := fileutil.PublishContext(ctx, func() error { return nil }); err != nil {
				return "", err
			}
			return destination, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		err := fileutil.PublishContext(ctx, func() error { return sqliteBackupLink(temporary, destination) })
		if err == nil {
			return destination, nil
		}
		if !os.IsExist(err) {
			return "", fmt.Errorf("publish migration backup: %w", err)
		}
	}
}

func verifyMigrationBackup(ctx context.Context, path string, digest [sha256.Size]byte, size int64) error {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	name := filepath.Base(path)
	before, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if !before.Mode().IsRegular() || before.Size() != size {
		return fmt.Errorf("migration backup is not a matching regular file")
	}
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(before, opened) {
		return fmt.Errorf("migration backup changed while opening")
	}
	if err := migrationBackupIsPrivate(file); err != nil {
		return err
	}
	actual, err := migrationBackupHash(ctx, file)
	if err != nil {
		return err
	}
	after, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if actual != digest || !os.SameFile(opened, after) || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
		return fmt.Errorf("migration backup checksum or identity changed")
	}
	return nil
}
