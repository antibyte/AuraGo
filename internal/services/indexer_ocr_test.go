package services

import (
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/ocr"
	"aurago/internal/tools"
	"bytes"
	"context"
	"errors"
	"fmt"
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
	oldFingerprint := fi.indexFingerprintForMode(fileIndexModePDF, true)
	cfg.OCR.AutomaticIndexing = false
	if oldFingerprint == fi.indexFingerprintForMode(fileIndexModePDF, true) {
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

func TestOCRIndexFingerprintPreservesLegacyWhenDisabled(t *testing.T) {
	cfg := &config.Config{OCR: config.DefaultOCRConfig()}
	fi := NewFileIndexer(cfg, &sync.RWMutex{}, &fakeIndexerVectorDB{}, nil, slog.Default())
	for _, mode := range []string{fileIndexModePDF, fileIndexModeVision, fileIndexModeMultimodal} {
		t.Run(mode, func(t *testing.T) {
			cfg.OCR = config.DefaultOCRConfig()
			legacy := fi.indexFingerprintForMode(mode, false)
			if got := fi.indexFingerprintForMode(mode, true); got != legacy || strings.Contains(got, "ocr=") {
				t.Fatal("disabled OCR changed the legacy fingerprint", got)
			}
			cfg.OCR.Enabled = true
			if got := fi.indexFingerprintForMode(mode, true); got == legacy || !strings.Contains(got, "ocr=") {
				t.Fatal("enabled OCR missing from fingerprint", got)
			}
			if got := fi.indexFingerprintForMode(mode, false); got != legacy {
				t.Fatal("unsupported or fallback input marked as OCR", got)
			}
			cfg.OCR.AutomaticIndexing = false
			if got := fi.indexFingerprintForMode(mode, true); got != legacy {
				t.Fatal("disabled automatic OCR changed the legacy fingerprint", got)
			}
		})
	}
}

func TestOCRIndexRevisitsFallbackWhenServiceStarts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "digital.pdf")
	writeOCRIndexPDF(t, path)
	cfg := &config.Config{OCR: config.DefaultOCRConfig()}
	cfg.OCR.Enabled = true
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".pdf"}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, &sync.RWMutex{}, vdb, stm, logger)
	m, err := ocr.New(filepath.Join(t.TempDir(), "jobs"), ocr.Policy{Enabled: true}, &indexOCRRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	tools.PublishOCR(m)
	defer func() { tools.WithdrawOCR(m); _ = m.Close(context.Background()) }()
	if _, err = fi.IndexFile(context.Background(), cfg.Indexing.Directories[0], path, nil); err != nil {
		t.Fatal(err)
	}
	fallback, err := stm.GetFileIndexState(path, IndexerCollection)
	if err != nil || strings.Contains(fallback.IndexFingerprint, "ocr=") || len(vdb.stored) != 1 || !strings.Contains(vdb.stored[0], "digital") {
		t.Fatal("fallback generation has incorrect content or provenance", fallback, vdb.stored, err)
	}
	if err = m.Start(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if _, err = fi.IndexFile(context.Background(), cfg.Indexing.Directories[0], path, nil); err != nil {
		t.Fatal(err)
	}
	if len(vdb.stored) != 2 || !strings.Contains(vdb.stored[1], "scanned") {
		t.Fatal("ready OCR did not revisit fallback", vdb.stored)
	}
	complete, err := stm.GetFileIndexState(path, IndexerCollection)
	if err != nil || !strings.Contains(complete.IndexFingerprint, "ocr=") {
		t.Fatal("OCR generation missing provenance", complete, err)
	}
	if _, err = fi.IndexFile(context.Background(), cfg.Indexing.Directories[0], path, nil); err != nil || len(vdb.stored) != 2 {
		t.Fatal("unchanged OCR generation was indexed twice", err)
	}
}

func writeOCRIndexPDF(t *testing.T, path string) {
	t.Helper()
	var data bytes.Buffer
	data.WriteString("%PDF-1.4\n")
	stream := "BT /F1 18 Tf 50 750 Td (AuraGo digital fallback text) Tj ET"
	offsets := []int{}
	for index, object := range []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
	} {
		offsets = append(offsets, data.Len())
		fmt.Fprintf(&data, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := data.Len()
	data.WriteString("xref\n0 6\n0000000000 65535 f \n")
	for _, offset := range offsets {
		fmt.Fprintf(&data, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&data, "trailer << /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xref)
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}
