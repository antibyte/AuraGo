package memory

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/fileutil"
	chromem "github.com/philippgille/chromem-go"
)

func readLegacyGob(path string, dest any, expectedHash string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("legacy vector must be regular: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return fmt.Errorf("legacy vector changed while opening")
	}
	hash := sha256.New()
	raw := io.TeeReader(file, hash)
	var reader io.Reader = raw
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(raw)
		if err != nil {
			return err
		}
		defer gz.Close()
		reader = gz
	}
	data, err := io.ReadAll(io.LimitReader(reader, legacyImportReadLimit+1))
	if err != nil {
		return err
	}
	if len(data) > legacyImportReadLimit {
		return fmt.Errorf("legacy GOB %s exceeds 64 MiB decompressed read limit", filepath.Base(path))
	}
	if _, err := io.Copy(io.Discard, raw); err != nil {
		return err
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != expectedHash {
		return fmt.Errorf("legacy source changed during decoding")
	}
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(dest); err != nil {
		return fmt.Errorf("decode legacy vector %s: %w", filepath.Base(path), err)
	}
	return nil
}

func copyLegacyFile(from, to string, expectedSize int64) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err := fileutil.RestrictToOwner(to); err != nil {
		dst.Close()
		return err
	}
	n, copyErr := io.Copy(dst, io.LimitReader(src, expectedSize+1))
	if copyErr == nil && n != expectedSize {
		copyErr = fmt.Errorf("legacy source size changed during backup")
	}
	return errors.Join(copyErr, dst.Sync(), dst.Close())
}

type legacyCollection struct {
	name, metadataFile string
	documents          []string
}

// Inspect one document at a time; the retained inventory contains paths and hashes, not vectors.
func inspectLegacyCollections(dir string, inventory []legacyFile) ([]legacyCollection, int64, error) {
	var collections []legacyCollection
	var required int64
	for i := 0; i < len(inventory); {
		folder := filepath.Dir(inventory[i].Path)
		col := legacyCollection{}
		for i < len(inventory) && filepath.Dir(inventory[i].Path) == folder {
			entry := inventory[i]
			name := filepath.Base(entry.Path)
			if name == "00000000.gob" || name == "00000000.gob.gz" {
				if col.metadataFile != "" {
					return nil, 0, fmt.Errorf("duplicate legacy collection metadata")
				}
				var meta struct {
					Name     string
					Metadata map[string]string
				}
				if err := readLegacyGob(filepath.Join(dir, entry.Path), &meta, entry.SHA256); err != nil {
					return nil, 0, err
				}
				if meta.Name == "" {
					return nil, 0, fmt.Errorf("legacy collection metadata has no name")
				}
				col.name, col.metadataFile = meta.Name, entry.Path
				encoded, err := json.Marshal(meta.Metadata)
				if err != nil {
					return nil, 0, err
				}
				required += int64(len(encoded) + len(meta.Name) + 4096)
			} else {
				var doc chromem.Document
				if err := readLegacyGob(filepath.Join(dir, entry.Path), &doc, entry.SHA256); err != nil {
					return nil, 0, err
				}
				if doc.ID == "" || doc.Content == "" {
					return nil, 0, fmt.Errorf("legacy document identity/content is empty")
				}
				hash := sha256.Sum256([]byte(doc.ID))
				want := fmt.Sprintf("%x", hash[:4]) + ".gob"
				if strings.HasSuffix(name, ".gz") {
					want += ".gz"
				}
				if name != want {
					return nil, 0, fmt.Errorf("legacy document filename does not match raw ID")
				}
				required += estimatedCorpusBytes([]chromem.Document{doc}, 0)
				col.documents = append(col.documents, entry.Path)
			}
			i++
		}
		if col.metadataFile == "" {
			return nil, 0, fmt.Errorf("legacy collection metadata is missing")
		}
		collections = append(collections, col)
	}
	return collections, required, nil
}

func (c *documentCorpus) migrateLegacy(dir string) error {
	dir = filepath.Clean(dir)
	inventory, err := inventoryLegacyFiles(dir)
	if err != nil {
		return err
	}
	collections, required, err := inspectLegacyCollections(dir, inventory)
	if err != nil {
		return err
	}
	manifest, err := makeLegacyManifest(inventory)
	if err != nil {
		return err
	}
	backup, err := findLegacyBackup(dir, manifest)
	if err != nil {
		return err
	}
	if err := c.checkCapacity(context.Background(), required); err != nil {
		return err
	}
	if len(inventory) > 0 && backup == "" {
		var backupBytes int64
		for _, file := range inventory {
			backupBytes += file.Size + 4096
		}
		free, err := c.freeDisk(filepath.Dir(dir))
		if err != nil {
			return corpusCapacityError("corpus_disk_status_unavailable")
		}
		if free < CorpusDiskReserveBytes+required*3+backupBytes+65536 {
			return corpusCapacityError("legacy_backup_disk_reserve")
		}
		backup, err = publishLegacyBackup(dir, manifest)
		if err != nil {
			return err
		}
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	expected := 0
	hashes := make(map[string]string, len(inventory))
	for _, file := range inventory {
		hashes[file.Path] = file.SHA256
	}
	for _, col := range collections {
		var meta struct {
			Name     string
			Metadata map[string]string
		}
		if err := readLegacyGob(filepath.Join(backup, col.metadataFile), &meta, hashes[col.metadataFile]); err != nil {
			return err
		}
		if meta.Name != col.name {
			return fmt.Errorf("legacy backup collection changed")
		}
		metadata, err := json.Marshal(meta.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO collections(name,metadata) VALUES(?,?) ON CONFLICT(name) DO UPDATE SET metadata=excluded.metadata`, col.name, string(metadata)); err != nil {
			return err
		}
		for _, name := range col.documents {
			var doc chromem.Document
			if err := readLegacyGob(filepath.Join(backup, name), &doc, hashes[name]); err != nil {
				return err
			}
			if err := writeCorpusDocument(context.Background(), tx, col.name, doc); err != nil {
				return err
			}
			metadata, err := json.Marshal(doc.Metadata)
			if err != nil {
				return err
			}
			vector, err := json.Marshal(doc.Embedding)
			if err != nil {
				return err
			}
			var content, storedMeta string
			var storedVector []byte
			if err := tx.QueryRow(`SELECT content,metadata,vector FROM documents WHERE collection=? AND id=?`, col.name, doc.ID).Scan(&content, &storedMeta, &storedVector); err != nil {
				return err
			}
			if content != doc.Content || storedMeta != string(metadata) || !bytes.Equal(storedVector, vector) {
				return fmt.Errorf("legacy corpus content verification failed")
			}
			expected++
		}
	}
	var count int
	if err := tx.QueryRow(`SELECT count(*) FROM documents`).Scan(&count); err != nil {
		return err
	}
	if count != expected {
		return fmt.Errorf("legacy corpus count verification failed: got %d want %d", count, expected)
	}
	if err := verifyLegacyInventory(dir, manifest.Files); err != nil {
		return fmt.Errorf("legacy source changed before migration commit: %w", err)
	}
	if len(inventory) > 0 {
		if err := verifyLegacyInventory(backup, manifest.Files); err != nil {
			return fmt.Errorf("legacy backup changed before migration commit: %w", err)
		}
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO corpus_meta(key,value) VALUES('verified','1')`); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit verified corpus migration: %w", err)
	}
	return nil
}
