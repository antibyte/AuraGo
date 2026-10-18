package memory

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	chromem "github.com/philippgille/chromem-go"
	"io"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/fileutil"
)

func readLegacyGob(path string, dest any) error {
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
	var reader io.Reader = file
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gz.Close()
		reader = gz
	}
	if err := gob.NewDecoder(reader).Decode(dest); err != nil {
		return fmt.Errorf("decode legacy vector %s: %w", filepath.Base(path), err)
	}
	return nil
}

func copyLegacyFile(from, to string) error {
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
	_, copyErr := io.Copy(dst, src)
	return errors.Join(copyErr, dst.Sync(), dst.Close())
}

func (c *documentCorpus) migrateLegacy(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type legacyCollection struct {
		name     string
		metadata map[string]string
		docs     []chromem.Document
	}
	var collections []legacyCollection
	var legacyFiles []string
	legacyHashes := map[string][32]byte{}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("legacy vector symlink is unsupported")
		}
		if !entry.IsDir() {
			if strings.HasSuffix(entry.Name(), ".gob") || strings.HasSuffix(entry.Name(), ".gob.gz") {
				return fmt.Errorf("orphan legacy document outside a collection")
			}
			continue
		}
		path := filepath.Join(dir, entry.Name())
		files, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		var meta struct {
			Name     string
			Metadata map[string]string
		}
		var docs []chromem.Document
		hasGob := false
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if !strings.HasSuffix(file.Name(), ".gob") && !strings.HasSuffix(file.Name(), ".gob.gz") {
				continue
			}
			full := filepath.Join(path, file.Name())
			hasGob = true
			before, err := os.ReadFile(full)
			if err != nil {
				return err
			}
			legacyHashes[full] = sha256.Sum256(before)
			legacyFiles = append(legacyFiles, full)
			if file.Name() == "00000000.gob" || file.Name() == "00000000.gob.gz" {
				if err := readLegacyGob(full, &meta); err != nil {
					return err
				}
			} else {
				var doc chromem.Document
				if err := readLegacyGob(full, &doc); err != nil {
					return err
				}
				if doc.ID == "" || doc.Content == "" {
					return fmt.Errorf("legacy document identity/content is empty")
				}
				hash := sha256.Sum256([]byte(doc.ID))
				want := fmt.Sprintf("%x", hash[:4]) + ".gob"
				if strings.HasSuffix(file.Name(), ".gz") {
					want += ".gz"
				}
				if file.Name() != want {
					return fmt.Errorf("legacy document filename does not match raw ID")
				}
				docs = append(docs, doc)
			}
			after, err := os.ReadFile(full)
			if err != nil {
				return err
			}
			if sha256.Sum256(after) != legacyHashes[full] {
				return fmt.Errorf("legacy source changed during decoding")
			}
		}
		if len(docs) == 0 && meta.Name == "" {
			if hasGob {
				return fmt.Errorf("legacy collection metadata has no name")
			}
			continue
		}
		if meta.Name == "" {
			return fmt.Errorf("legacy collection metadata is missing")
		}
		collections = append(collections, legacyCollection{meta.Name, meta.Metadata, docs})
	}
	// Original files remain in place; the synced copy is retained before migration.
	if len(legacyFiles) > 0 {
		backup, err := os.MkdirTemp(filepath.Dir(dir), filepath.Base(dir)+".legacy-backup-")
		if err != nil {
			return err
		}
		if err := fileutil.RestrictToOwner(backup); err != nil {
			return err
		}
		for _, from := range legacyFiles {
			rel, err := filepath.Rel(dir, from)
			if err != nil {
				return err
			}
			to := filepath.Join(backup, rel)
			if err := os.MkdirAll(filepath.Dir(to), 0700); err != nil {
				return err
			}
			if err := fileutil.RestrictToOwner(filepath.Dir(to)); err != nil {
				return err
			}
			if err := copyLegacyFile(from, to); err != nil {
				return fmt.Errorf("backup legacy vectors: %w", err)
			}
			copied, err := os.ReadFile(to)
			if err != nil {
				return err
			}
			if sha256.Sum256(copied) != legacyHashes[from] {
				return fmt.Errorf("legacy backup differs from decoded source")
			}
		}
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	expected := 0
	for _, col := range collections {
		metadata, err := json.Marshal(col.metadata)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO collections(name,metadata) VALUES(?,?) ON CONFLICT(name) DO UPDATE SET metadata=excluded.metadata`, col.name, string(metadata)); err != nil {
			return err
		}
		for _, doc := range col.docs {
			if err := writeCorpusDocument(context.Background(), tx, col.name, doc); err != nil {
				return err
			}
			metadata, _ := json.Marshal(doc.Metadata)
			vector, _ := json.Marshal(doc.Embedding)
			var content, storedMeta string
			var storedVector []byte
			if err := tx.QueryRow(`SELECT content,metadata,vector FROM documents WHERE collection=? AND id=?`, col.name, doc.ID).Scan(&content, &storedMeta, &storedVector); err != nil {
				return err
			}
			if content != doc.Content || storedMeta != string(metadata) || string(storedVector) != string(vector) {
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
	for path, want := range legacyHashes {
		actual, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if sha256.Sum256(actual) != want {
			return fmt.Errorf("legacy source changed before migration commit")
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
