package webassets

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// Import installs an unpacked resources.dat/image set using the same manifest
// pin as archive repair. The caller's source directory is never served directly.
func (s *Store) Import(ctx context.Context, source string) error {
	from := Open(source, s.Pin)
	defer from.Close()
	if !from.Ready() {
		return fmt.Errorf("verify imported assets: %w", from.Error)
	}
	if err := os.MkdirAll(s.Dir, 0750); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(s.Dir, ".install.lock"))
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return err
	}
	if !locked {
		return ctx.Err()
	}
	defer lock.Unlock()
	existing := Open(s.Dir, s.Pin)
	defer existing.Close()
	if existing.Ready() {
		return nil
	}
	stage, err := os.MkdirTemp(s.Dir, ".import-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	dest := filepath.Join(stage, s.Pin.ID)
	for name := range from.allowed {
		if err := ctx.Err(); err != nil {
			return err
		}
		f, err := from.Open(name)
		if err != nil {
			return err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return err
		}
		if info.IsDir() {
			f.Close()
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			f.Close()
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			f.Close()
			return err
		}
		_, err = io.Copy(out, io.LimitReader(f, info.Size()+1))
		f.Close()
		closeErr := out.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	m, err := from.root.ReadFile(ManifestName)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dest, ManifestName), m, 0644); err != nil {
		return err
	}
	verified := Open(stage, s.Pin)
	if !verified.Ready() {
		return verified.Error
	}
	verified.Close()
	return s.publish(ctx, dest)
}

func (s *Store) publish(ctx context.Context, staging string) error {
	target := filepath.Join(s.Dir, s.Pin.ID)
	if _, err := os.Lstat(target); err == nil {
		if err := os.Rename(target, target+fmt.Sprintf(".invalid-%d", time.Now().UnixNano())); err != nil {
			return err
		}
	}
	if err := publishDirectory(ctx, staging, target); err != nil {
		return fmt.Errorf("publish asset set: %w", err)
	}
	return nil
}
