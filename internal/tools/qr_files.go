package tools

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/fileutil"
)

var qrPublicationMu sync.Mutex

func qrOutputPermission(ctx context.Context, cfg *config.Config, root string) error {
	check := func(c *config.Config) error {
		if c == nil {
			return fmt.Errorf("configuration is unavailable")
		}
		switch root {
		case "desktop":
			if !c.VirtualDesktop.Enabled || !c.VirtualDesktop.AllowAgentControl || c.VirtualDesktop.ReadOnly {
				return fmt.Errorf("desktop agent writes are disabled")
			}
		case "workspace":
			if !c.Agent.AllowFilesystemWrite {
				return fmt.Errorf("filesystem write is disabled")
			}
		default:
			return fmt.Errorf("output_root must be desktop or workspace")
		}
		return nil
	}
	if err := check(cfg); err != nil {
		return err
	}
	if cfg.AuthorizationSnapshots != nil {
		_, current := cfg.AuthorizationSnapshots()
		if err := check(current); err != nil {
			return err
		}
	}
	if root == "workspace" {
		perms, configured := EffectiveRuntimePermissions(ctx)
		return requireRuntimePermission("filesystem write", configured && perms.AllowFilesystemWrite)
	}
	return nil
}

func qrWorkspaceFile(cfg *config.Config, raw string, write bool) (*os.Root, string, error) {
	if cfg == nil || cfg.Directories.WorkspaceDir == "" || len(raw) > 1024 || !filepath.IsLocal(raw) {
		return nil, "", fmt.Errorf("a relative workspace path is required")
	}
	resolved, err := secureResolve(cfg.Directories.WorkspaceDir, raw)
	if err != nil {
		return nil, "", fmt.Errorf("workspace path is not permitted")
	}
	if err = requireUnprotectedNotesPath(resolved, write); err != nil {
		return nil, "", err
	}
	rootPath, err := filepath.EvalSymlinks(cfg.Directories.WorkspaceDir)
	if err != nil {
		return nil, "", fmt.Errorf("workspace is unavailable")
	}
	rootPath, err = filepath.Abs(rootPath)
	if err != nil {
		return nil, "", fmt.Errorf("workspace is unavailable")
	}
	name, err := filepath.Rel(rootPath, resolved)
	if err != nil || name == "." || !filepath.IsLocal(name) {
		return nil, "", fmt.Errorf("path must remain inside the agent workspace")
	}
	// Root-relative I/O keeps symlink replacement races inside the authorized root.
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, "", fmt.Errorf("workspace is unavailable")
	}
	return root, name, nil
}

func qrPublish(ctx context.Context, cfg *config.Config, ref qrFileRef, data []byte, expected string) (string, *desktop.FileEntry, error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	if err := qrOutputPermission(ctx, cfg, ref.Root); err != nil {
		return "", nil, err
	}
	if ref.Root == "desktop" {
		svc, cleanup, err := getToolDesktopService(ctx, cfg)
		if err != nil {
			return "", nil, fmt.Errorf("desktop storage is unavailable")
		}
		defer cleanup()
		entry, err := svc.WriteFileBytesConditional(ctx, ref.Path, data, desktop.SourceAgent, func(state desktop.FileWriteState) error {
			if err := qrToolPermission(cfg, true); err != nil {
				return err
			}
			if err := qrOutputPermission(ctx, cfg, "desktop"); err != nil {
				return err
			}
			if expected == "" && state.Exists {
				return fmt.Errorf("output already exists; choose another path or supply its expected_version")
			}
			if expected != "" && (!state.Exists || qrVersion(state.Data) != expected) {
				return fmt.Errorf("output version conflict")
			}
			return ctx.Err()
		})
		if err != nil {
			return "", nil, fmt.Errorf("QR output was not published: %w", err)
		}
		return qrVersion(data), &entry, nil
	}
	root, name, err := qrWorkspaceFile(cfg, ref.Path, true)
	if err != nil {
		return "", nil, err
	}
	defer root.Close()
	if err = root.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return "", nil, fmt.Errorf("cannot create QR output directory")
	}
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		return "", nil, err
	}
	tmp := filepath.Join(filepath.Dir(name), fmt.Sprintf(".qr-%x.tmp", token))
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", nil, fmt.Errorf("cannot stage QR output")
	}
	defer root.Remove(tmp)
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", nil, fmt.Errorf("cannot stage complete QR output")
	}
	qrPublicationMu.Lock()
	defer qrPublicationMu.Unlock()
	err = fileutil.PublishContext(ctx, func() error {
		if err := qrToolPermission(cfg, true); err != nil {
			return err
		}
		if err := qrOutputPermission(ctx, cfg, "workspace"); err != nil {
			return err
		}
		if expected == "" { // Link is atomic and never replaces an existing destination.
			if err := root.Link(tmp, name); err != nil {
				return fmt.Errorf("output exists or cannot be published; choose a new path")
			}
			return nil
		}
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("output version conflict")
		}
		current, err := root.Open(name)
		if err != nil {
			return fmt.Errorf("output version conflict")
		}
		original, err := qrReadBounded(current, 128<<20)
		current.Close()
		if err != nil || !strings.EqualFold(qrVersion(original), expected) {
			return fmt.Errorf("output version conflict")
		}
		if err = root.Rename(tmp, name); err != nil {
			return fmt.Errorf("cannot replace QR output")
		}
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	return qrVersion(data), nil, nil
}
