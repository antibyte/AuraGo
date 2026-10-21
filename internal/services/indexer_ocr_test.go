package services

import (
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/ocr"
	"aurago/internal/tools"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type indexOCRRuntime struct{ fail bool }

func (*indexOCRRuntime) Start(context.Context, bool) error { return nil }
func (*indexOCRRuntime) Stop(context.Context) error        { return nil }
func (r *indexOCRRuntime) Convert(_ context.Context, _ string, _ string, _ string, emit func(ocr.Page) error) error {
	if err := emit(ocr.Page{Index: 0, Text: "Digital and scanned document contents", Source: "ocr"}); err != nil {
		return err
	}
	if r.fail {
		return errors.New("page failed")
	}
	return nil
}
func TestOCRIndexRootFingerprintAndFailedReplacement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "mixed.pdf")
	if err = os.WriteFile(path, []byte("%PDF-1.7\nfirst"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{OCR: config.DefaultOCRConfig()}
	cfg.OCR.Enabled = true
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".pdf"}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, &sync.RWMutex{}, vdb, stm, logger)
	rt := &indexOCRRuntime{}
	m, err := ocr.New(filepath.Join(t.TempDir(), "jobs"), ocr.Policy{Enabled: true}, rt)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	tools.PublishOCR(m)
	defer func() { tools.WithdrawOCR(m); _ = m.Close(context.Background()) }()
	if _, _, err = fi.extractOCR(context.Background(), cfg.Directories.WorkspaceDir, path); err == nil {
		t.Fatal("index root escape accepted")
	}
	result, err := fi.IndexFile(context.Background(), cfg.Indexing.Directories[0], path, nil)
	if err != nil || !result.Indexed || !strings.Contains(vdb.stored[0], "scanned") {
		t.Fatalf("%+v %v", result, err)
	}
	before, err := stm.GetFileEmbeddingDocIDs(path, IndexerCollection)
	if err != nil {
		t.Fatal(err)
	}
	oldFingerprint := fi.indexFingerprintForMode(fileIndexModePDF)
	cfg.OCR.AutomaticIndexing = false
	if oldFingerprint == fi.indexFingerprintForMode(fileIndexModePDF) {
		t.Fatal("OCR missing from fingerprint")
	}
	cfg.OCR.AutomaticIndexing = true
	rt.fail = true
	if err = os.WriteFile(path, []byte("%PDF-1.7\nchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = fi.IndexFile(context.Background(), cfg.Indexing.Directories[0], path, nil); err == nil {
		t.Fatal("partial OCR replaced valid index")
	}
	after, err := stm.GetFileEmbeddingDocIDs(path, IndexerCollection)
	if err != nil || !reflect.DeepEqual(before, after) || len(vdb.deleted) != 0 {
		t.Fatal("valid generation lost", before, after, err)
	}
	if err = m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = fi.IndexFile(context.Background(), cfg.Indexing.Directories[0], path, nil); err == nil || !strings.Contains(err.Error(), "retaining existing index") {
		t.Fatal("unavailable OCR downgraded existing data", err)
	}
	after, err = stm.GetFileEmbeddingDocIDs(path, IndexerCollection)
	if err != nil || !reflect.DeepEqual(before, after) || len(vdb.deleted) != 0 {
		t.Fatal("valid index lost while OCR stopped", err)
	}
}
