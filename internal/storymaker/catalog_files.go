package storymaker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"path"
	"strings"

	_ "golang.org/x/image/webp"
)

// CatalogFile is a bounded, content-addressed delivery dependency of catalog v2.
type CatalogFile struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

const CatalogDeliveryLimit = 256 << 20
const CatalogTextureLimit = 8 << 20

// DeliveryFiles returns a copy: a loaded catalog is shared between requests.
func (c *Catalog) DeliveryFiles() []CatalogFile { return append([]CatalogFile(nil), c.files...) }

// ManifestJSON copies the checked startup manifest, independent of later filesystem edits.
func (c *Catalog) ManifestJSON() []byte { return append([]byte(nil), c.manifest...) }

func (c *Catalog) checkFiles(bad func(string, ...any)) {
	if c.Schema != CatalogSchema {
		return
	}
	if len(c.files) == 0 || len(c.files) > 2000 {
		bad("files must list 1 to 2000 delivery dependencies")
	}
	listed := map[string]bool{}
	var total int64
	for _, f := range c.files {
		ascii := len(f.Path) <= 230 && strings.IndexFunc(f.Path, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_./-", r))
		}) < 0
		if !safeRelPath(f.Path) || !ascii || f.Path == "catalog.json" || listed[f.Path] {
			bad("invalid or duplicate dependency %q", f.Path)
		}
		listed[f.Path] = true
		hash, err := hex.DecodeString(f.SHA256)
		if err != nil || len(hash) != sha256.Size || strings.ToLower(f.SHA256) != f.SHA256 {
			bad("dependency %q needs a lowercase SHA-256", f.Path)
		}
		if f.Bytes <= 0 || f.Bytes > CatalogTextureLimit {
			bad("dependency %q exceeds its byte limit", f.Path)
		} else {
			total += f.Bytes
		}
		switch f.Type {
		case "image/png", "image/webp", "image/jpeg":
			maxWidth, maxHeight := 2048, 2048
			if strings.HasPrefix(f.Path, "textures/environments/") && !strings.HasPrefix(path.Base(f.Path), "foreground-") {
				maxWidth, maxHeight = 3840, 2160
			}
			if strings.HasPrefix(f.Path, "thumbs/") {
				maxWidth, maxHeight = 256, 256
			}
			if f.Width <= 0 || f.Height <= 0 || f.Width > maxWidth || f.Height > maxHeight {
				bad("dependency %q has invalid image dimensions", f.Path)
			}
		case "application/json", "audio/wav":
			if f.Width != 0 || f.Height != 0 {
				bad("non-image dependency %q has dimensions", f.Path)
			}
		default:
			bad("dependency %q has unsupported type %q", f.Path, f.Type)
		}
	}
	if total > CatalogDeliveryLimit {
		bad("catalog exceeds the 256 MiB delivery budget")
	}
	require := func(p string) {
		if p != "" && !listed[p] {
			bad("unlisted dependency %q", p)
		}
	}
	require(c.Rig.File)
	require(c.glyphsFile)
	for _, e := range c.entries {
		require(e.File)
		require(e.Thumb)
		for _, t := range e.Takes {
			require(t.File)
		}
		var resources struct {
			Resources []string `json:"resources"`
		}
		if err := json.Unmarshal(e.raw, &resources); err != nil {
			bad("invalid resource list for %s", e.ID)
		}
		for _, resource := range resources.Resources {
			require(resource)
		}
	}
}

func (c *Catalog) verifyDelivery(fsys fs.FS, dir string) error {
	for _, meta := range c.files {
		f, err := fsys.Open(path.Join(dir, meta.Path))
		if err != nil {
			return fmt.Errorf("catalog dependency %s: %w", meta.Path, err)
		}
		hash := sha256.New()
		n, readErr := io.Copy(hash, io.LimitReader(f, meta.Bytes+1))
		closeErr := f.Close()
		if readErr != nil || closeErr != nil || n != meta.Bytes || hex.EncodeToString(hash.Sum(nil)) != meta.SHA256 {
			return fmt.Errorf("catalog dependency %s failed size or integrity verification", meta.Path)
		}
		if strings.HasPrefix(meta.Type, "image/") {
			f, err = fsys.Open(path.Join(dir, meta.Path))
			if err != nil {
				return fmt.Errorf("catalog image %s: %w", meta.Path, err)
			}
			config, format, decodeErr := image.DecodeConfig(io.LimitReader(f, CatalogTextureLimit+1))
			closeErr = f.Close()
			if decodeErr != nil || closeErr != nil || "image/"+format != meta.Type || config.Width != meta.Width || config.Height != meta.Height {
				return fmt.Errorf("catalog image %s failed type or dimension verification", meta.Path)
			}
		}
	}
	return nil
}
