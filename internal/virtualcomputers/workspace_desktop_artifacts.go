package virtualcomputers

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Startup removes only captures owned by this manager's dedicated directory.
// Keeping initialization per directory prevents later config reads deleting live images.
func (m *WorkspaceManager) prepareDesktopArtifacts(dir string) error {
	if dir == "" {
		return nil
	}
	m.guiMu.Lock()
	defer m.guiMu.Unlock()
	if m.guiClosed {
		return fmt.Errorf("workspace manager is closed")
	}
	if m.desktopArtifactDirs[dir] {
		return nil
	}
	if err := removeOrphanDesktopCaptures(dir); err != nil {
		return err
	}
	if m.desktopArtifactDirs == nil {
		m.desktopArtifactDirs = make(map[string]bool)
	}
	m.desktopArtifactDirs[dir] = true
	return nil
}

func removeOrphanDesktopCaptures(dir string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("private desktop capture directory is not a directory")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	listing, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, err := listing.ReadDir(-1)
	_ = listing.Close()
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "capture-") || !strings.HasSuffix(entry.Name(), ".png") {
			continue
		}
		if err := root.Remove(entry.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *WorkspaceManager) closeDesktopGUI() error {
	m.guiMu.Lock()
	defer m.guiMu.Unlock()
	m.guiClosed = true
	var errs []error
	for _, g := range m.gui {
		g.mu.Lock()
		g.closed, g.blocked = true, true
		if g.cancel != nil {
			g.cancel()
		}
		if err := g.clearImageLocked(); err != nil {
			errs = append(errs, err)
		}
		g.mu.Unlock()
	}
	return errors.Join(errs...)
}
