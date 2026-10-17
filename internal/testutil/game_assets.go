package testutil

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/webassets"
)

// RunWithGameMakerAssets explicitly installs the small Game Maker fixtures. This fallback is
// compiled only into tests; released binaries never read repository sources.
func RunWithGameMakerAssets(run func() int) int {
	root, err := os.MkdirTemp("", "aurago-test-assets-")
	if err != nil {
		panic(err)
	}
	// This exact temporary directory is owned by this test process.
	defer os.RemoveAll(root)
	stage := filepath.Join(root, "stage")
	var manifest webassets.Manifest
	manifest.Version = 1
	for _, dir := range []string{"runtime", "asset_packs"} {
		err = filepath.WalkDir(filepath.Join("..", "gamemaker", dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "production" {
					return filepath.SkipDir
				}
				return nil
			}
			ext := filepath.Ext(p)
			if ext != ".js" && ext != ".json" && ext != ".png" && ext != ".glb" && ext != ".wav" && d.Name() != "THIRD_PARTY_NOTICES.md" && d.Name() != "LICENSE.txt" {
				return nil
			}
			rel, err := filepath.Rel(filepath.Join("..", "gamemaker"), p)
			if err != nil {
				return err
			}
			name := "gamemaker/" + filepath.ToSlash(rel)
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			target := filepath.Join(stage, filepath.FromSlash(name))
			if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err = os.WriteFile(target, b, 0644); err != nil {
				return err
			}
			manifest.Files = append(manifest.Files, webassets.Entry{Path: name, Size: int64(len(b)), SHA256: webassets.Digest(b)})
			return nil
		})
		if err != nil {
			panic(err)
		}
	}
	b, _ := json.Marshal(manifest)
	id := webassets.Digest(b)
	if err = os.WriteFile(filepath.Join(stage, webassets.ManifestName), b, 0644); err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err = fileutil.RenameContext(ctx, stage, filepath.Join(root, id)); err != nil {
		panic(err)
	}
	store := webassets.Open(root, webassets.Pin{ID: id})
	defer store.Close()
	if !store.Ready() {
		panic(store.Error)
	}
	webassets.Default = store
	return run()
}
