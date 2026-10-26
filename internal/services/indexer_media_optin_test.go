package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"aurago/internal/chunking"
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/testutil"
)

// mediaOptInFixture indexes one folder whose Vision and multimodal embedding
// providers are local HTTP servers that count the requests they receive. Any
// request means media left the indexer.
type mediaOptInFixture struct {
	fi          *FileIndexer
	cfg         *config.Config
	cfgMu       *sync.RWMutex
	vdb         *fakeIndexerVectorDB
	stm         *memory.SQLiteMemory
	dir         string
	visionCalls *atomic.Int32
	embedCalls  *atomic.Int32
	logs        *bytes.Buffer
}

func newMediaOptInFixture(t *testing.T, allow, indexImages, multimodal bool, extensions []string) *mediaOptInFixture {
	t.Helper()
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	visionCalls, embedCalls := &atomic.Int32{}, &atomic.Int32{}
	visionSrv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visionCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"a described image"}}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`))
	}))
	t.Cleanup(visionSrv.Close)
	embedSrv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		embedCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]}]}`))
	}))
	t.Cleanup(embedSrv.Close)

	dir := filepath.Join(t.TempDir(), "mediafolder")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = dir
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir, AllowMediaAnalysis: allow}}
	cfg.Indexing.Extensions = extensions
	cfg.Indexing.IndexImages = indexImages
	cfg.Embeddings.Multimodal = multimodal
	cfg.Embeddings.BaseURL = embedSrv.URL
	cfg.Embeddings.Model = "embed-model"
	cfg.Embeddings.ProviderType = "openai"
	cfg.Embeddings.MultimodalFormat = "openai"
	cfg.Vision.BaseURL = visionSrv.URL
	cfg.Vision.APIKey = "vision-key"
	cfg.Vision.Model = "vision-model"

	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	return &mediaOptInFixture{
		fi: NewFileIndexer(cfg, cfgMu, vdb, stm, logger), cfg: cfg, cfgMu: cfgMu, vdb: vdb, stm: stm,
		dir: dir, visionCalls: visionCalls, embedCalls: embedCalls, logs: logs,
	}
}

func (f *mediaOptInFixture) write(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(f.dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func (f *mediaOptInFixture) setAllow(allow bool) {
	f.cfgMu.Lock()
	f.cfg.Indexing.Directories[0].AllowMediaAnalysis = allow
	f.cfgMu.Unlock()
}

func (f *mediaOptInFixture) scan(t *testing.T) (total, indexed int, errs []string) {
	t.Helper()
	return f.fi.scanDirectory(context.Background(), f.dir, IndexerCollection)
}

func (f *mediaOptInFixture) tracked(t *testing.T, path string) bool {
	t.Helper()
	modTime, err := f.stm.GetFileIndex(path, IndexerCollection)
	if err != nil {
		t.Fatalf("GetFileIndex: %v", err)
	}
	ids, err := f.stm.GetFileEmbeddingDocIDs(path, IndexerCollection)
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs: %v", err)
	}
	return !modTime.IsZero() || len(ids) > 0
}

func (f *mediaOptInFixture) providerCalls() (vision, embed int32) {
	return f.visionCalls.Load(), f.embedCalls.Load()
}

func TestFileIndexerMediaOptInKeepsImagesAwayFromProvidersWithoutPermission(t *testing.T) {
	cases := []struct {
		name        string
		indexImages bool
		multimodal  bool
		extensions  []string
	}{
		{"vision only", true, false, []string{".txt"}},
		{"vision only with the image extension listed", true, false, []string{".txt", ".jpg"}},
		{"multimodal only", false, true, []string{".txt"}},
		{"vision and multimodal", true, true, []string{".txt", ".jpg"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newMediaOptInFixture(t, false, tc.indexImages, tc.multimodal, tc.extensions)
			path := f.write(t, "photo.jpg", tinyIndexerJPEG(t))

			total, indexed, errs := f.scan(t)
			if total != 0 || indexed != 0 || len(errs) != 0 {
				t.Fatalf("scan total=%d indexed=%d errors=%v, want nothing counted, indexed or reported", total, indexed, errs)
			}
			if counted, countErrs := f.fi.countIndexableFiles(context.Background(), f.dir); counted != 0 || len(countErrs) != 0 {
				t.Fatalf("countIndexableFiles = %d %v, want 0", counted, countErrs)
			}
			// The user-triggered single-file path shares the gate.
			if _, err := f.fi.IndexFile(context.Background(), f.cfg.Indexing.Directories[0], path, nil); !errors.Is(err, ErrNoIndexableContent) {
				t.Fatalf("IndexFile error = %v, want ErrNoIndexableContent", err)
			}
			if vision, embed := f.providerCalls(); vision != 0 || embed != 0 {
				t.Fatalf("provider calls vision=%d embed=%d, want 0 without folder permission", vision, embed)
			}
			if f.vdb.nextID != 0 || len(f.vdb.stored) != 0 {
				t.Fatalf("image reached the vector store: nextID=%d stored=%v", f.vdb.nextID, f.vdb.stored)
			}
			if f.tracked(t, path) {
				t.Fatal("an untracked image gained index state")
			}
		})
	}
}

func TestFileIndexerMediaOptInSendsImagesOnceThePermissionIsGiven(t *testing.T) {
	t.Run("vision", func(t *testing.T) {
		f := newMediaOptInFixture(t, true, true, false, []string{".txt"})
		f.write(t, "photo.jpg", tinyIndexerJPEG(t))
		total, indexed, errs := f.scan(t)
		if total != 1 || indexed != 1 || len(errs) != 0 {
			t.Fatalf("scan total=%d indexed=%d errors=%v", total, indexed, errs)
		}
		if vision, embed := f.providerCalls(); vision != 1 || embed != 0 {
			t.Fatalf("provider calls vision=%d embed=%d, want 1/0", vision, embed)
		}
	})
	t.Run("multimodal", func(t *testing.T) {
		f := newMediaOptInFixture(t, true, false, true, []string{".txt"})
		f.write(t, "photo.jpg", tinyIndexerJPEG(t))
		total, indexed, errs := f.scan(t)
		if total != 1 || indexed != 1 || len(errs) != 0 {
			t.Fatalf("scan total=%d indexed=%d errors=%v", total, indexed, errs)
		}
		if vision, embed := f.providerCalls(); vision != 0 || embed != 1 {
			t.Fatalf("provider calls vision=%d embed=%d, want 0/1", vision, embed)
		}
	})
	t.Run("permission alone is not a capability", func(t *testing.T) {
		f := newMediaOptInFixture(t, true, false, false, []string{".txt", ".jpg"})
		f.write(t, "photo.jpg", tinyIndexerJPEG(t))
		total, indexed, errs := f.scan(t)
		if total != 0 || indexed != 0 || len(errs) != 0 {
			t.Fatalf("scan total=%d indexed=%d errors=%v", total, indexed, errs)
		}
		if vision, embed := f.providerCalls(); vision != 0 || embed != 0 {
			t.Fatalf("provider calls vision=%d embed=%d, want 0/0 while both global switches are off", vision, embed)
		}
	})
}

func TestFileIndexerMediaOptInRemovesTrackedImageWhenPermissionIsRevoked(t *testing.T) {
	f := newMediaOptInFixture(t, true, true, false, []string{".txt"})
	path := f.write(t, "photo.jpg", tinyIndexerJPEG(t))

	if _, indexed, errs := f.scan(t); indexed != 1 || len(errs) != 0 {
		t.Fatalf("first scan indexed=%d errors=%v", indexed, errs)
	}
	if !f.tracked(t, path) {
		t.Fatal("image is not tracked after the permitted scan")
	}

	f.setAllow(false)
	total, indexed, errs := f.scan(t)
	if total != 0 || indexed != 0 || len(errs) != 0 {
		t.Fatalf("revoked scan total=%d indexed=%d errors=%v", total, indexed, errs)
	}
	if f.tracked(t, path) {
		t.Fatal("image is still tracked after the permission was revoked")
	}
	if len(f.vdb.deleted) == 0 {
		t.Fatal("the vector documents of the revoked image were not deleted")
	}
	if vision, _ := f.providerCalls(); vision != 1 {
		t.Fatalf("vision calls = %d, want only the one from the permitted scan", vision)
	}

	// Giving the permission back indexes the image again from scratch.
	f.setAllow(true)
	if _, indexed, errs := f.scan(t); indexed != 1 || len(errs) != 0 {
		t.Fatalf("re-permitted scan indexed=%d errors=%v", indexed, errs)
	}
	if vision, _ := f.providerCalls(); vision != 2 {
		t.Fatalf("vision calls after re-permitting = %d, want 2", vision)
	}
}

// A folder without permission never keeps provider-derived media in the index, also
// after the global capability switches were turned off again.
func TestFileIndexerMediaOptInRemovesTrackedImageWhenCapabilitiesAreOffToo(t *testing.T) {
	f := newMediaOptInFixture(t, true, true, false, []string{".txt"})
	path := f.write(t, "photo.jpg", tinyIndexerJPEG(t))
	if _, indexed, errs := f.scan(t); indexed != 1 || len(errs) != 0 {
		t.Fatalf("first scan indexed=%d errors=%v", indexed, errs)
	}

	f.setAllow(false)
	f.cfgMu.Lock()
	f.cfg.Indexing.IndexImages = false
	f.cfgMu.Unlock()
	if total, indexed, errs := f.scan(t); total != 0 || indexed != 0 || len(errs) != 0 {
		t.Fatalf("scan total=%d indexed=%d errors=%v", total, indexed, errs)
	}
	if f.tracked(t, path) {
		t.Fatal("image of a folder without permission is still tracked")
	}
	if strings.Contains(f.logs.String(), "allow_media_analysis") {
		t.Fatalf("no capability is enabled, so nothing was skipped on its account:\n%s", f.logs.String())
	}
}

func TestFileIndexerMediaOptInGatesAudio(t *testing.T) {
	f := newMediaOptInFixture(t, false, false, true, []string{".txt"})
	path := f.write(t, "talk.mp3", []byte("ID3 not really audio"))

	total, indexed, errs := f.scan(t)
	if total != 0 || indexed != 0 || len(errs) != 0 {
		t.Fatalf("scan total=%d indexed=%d errors=%v", total, indexed, errs)
	}
	if _, embed := f.providerCalls(); embed != 0 {
		t.Fatalf("embedding calls = %d, want 0 without folder permission", embed)
	}

	f.setAllow(true)
	if total, indexed, errs := f.scan(t); total != 1 || indexed != 1 || len(errs) != 0 {
		t.Fatalf("permitted scan total=%d indexed=%d errors=%v", total, indexed, errs)
	}
	if _, embed := f.providerCalls(); embed != 1 {
		t.Fatalf("embedding calls = %d, want 1 with folder permission", embed)
	}

	f.setAllow(false)
	if _, indexed, errs := f.scan(t); indexed != 0 || len(errs) != 0 {
		t.Fatalf("revoked scan indexed=%d errors=%v", indexed, errs)
	}
	if f.tracked(t, path) {
		t.Fatal("audio is still tracked after the permission was revoked")
	}
	if _, embed := f.providerCalls(); embed != 1 {
		t.Fatalf("embedding calls after revoking = %d, want still 1", embed)
	}
}

func TestFileIndexerMediaOptInGatesScannedPDFFallback(t *testing.T) {
	f := newMediaOptInFixture(t, false, false, true, []string{".pdf"})
	path := f.write(t, "scan.pdf", []byte("not a valid PDF but definitely raw bytes"))

	_, indexed, errs := f.scan(t)
	if indexed != 0 || len(errs) != 1 || !strings.Contains(errs[0], "text extraction error") {
		t.Fatalf("scan indexed=%d errors=%v, want one text extraction error", indexed, errs)
	}
	if _, embed := f.providerCalls(); embed != 0 {
		t.Fatalf("embedding calls = %d, want 0: the scanned PDF must not leave without permission", embed)
	}

	f.setAllow(true)
	if _, indexed, errs := f.scan(t); indexed != 1 || len(errs) != 0 {
		t.Fatalf("permitted scan indexed=%d errors=%v, want the fallback to index it", indexed, errs)
	}
	if _, embed := f.providerCalls(); embed != 1 {
		t.Fatalf("embedding calls = %d, want 1 with folder permission", embed)
	}
	if !f.tracked(t, path) {
		t.Fatal("scanned PDF is not tracked after the fallback")
	}

	// Revoking reindexes the PDF (fingerprint) and drops what the provider produced.
	f.setAllow(false)
	if _, indexed, errs := f.scan(t); indexed != 0 || len(errs) != 1 {
		t.Fatalf("revoked scan indexed=%d errors=%v, want one extraction error", indexed, errs)
	}
	if f.tracked(t, path) {
		t.Fatal("fallback-indexed PDF is still tracked after the permission was revoked")
	}
	if _, embed := f.providerCalls(); embed != 1 {
		t.Fatalf("embedding calls after revoking = %d, want still 1", embed)
	}
}

func TestIndexFingerprintMarksPDFMediaPermissionOnlyForMultimodal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{}
	cfg.Embeddings.Provider = "embed-provider"
	cfg.Embeddings.ProviderType = "openai"
	cfg.Embeddings.BaseURL = "https://embeddings.example.test/v1"
	cfg.Embeddings.Model = "embed-model"
	cfg.Embeddings.MultimodalFormat = "openai"
	cfgMu := &sync.RWMutex{}
	fi := NewFileIndexer(cfg, cfgMu, &fakeIndexerVectorDB{fingerprint: "emb-fp"}, nil, logger)

	// This is the fingerprint installations stored before the folder opt-in existed.
	legacy := strings.Join([]string{
		fileIndexerFingerprint, "mode=" + fileIndexModePDF, "embedding=emb-fp",
		"chunking=" + chunking.Fingerprint(fi.chunkingOptionsSnapshot()),
		fingerprintProvider("embeddings", "embed-provider", "openai", "https://embeddings.example.test/v1", "embed-model", ""),
		"multimodal_format=openai",
	}, "|")

	setMultimodal := func(on bool) {
		cfgMu.Lock()
		cfg.Embeddings.Multimodal = on
		cfgMu.Unlock()
	}
	for _, tc := range []struct {
		name       string
		multimodal bool
		allow      bool
		want       string
	}{
		{"no multimodal, no permission", false, false, legacy},
		{"no multimodal, permission", false, true, legacy},
		{"multimodal, no permission", true, false, legacy},
		{"multimodal, permission", true, true, legacy + "|media=allowed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setMultimodal(tc.multimodal)
			if got := fi.indexFingerprintForMode(fileIndexModePDF, tc.allow); got != tc.want {
				t.Fatalf("PDF fingerprint = %q, want %q", got, tc.want)
			}
		})
	}

	// Only PDFs depend on the permission; every other mode keeps its fingerprint.
	setMultimodal(true)
	for _, mode := range []string{fileIndexModeText, fileIndexModeDocument, fileIndexModeVision, fileIndexModeMultimodal} {
		if off, on := fi.indexFingerprintForMode(mode, false), fi.indexFingerprintForMode(mode, true); off != on {
			t.Fatalf("mode %s fingerprint changes with the permission: %q vs %q", mode, off, on)
		}
	}
}

func TestFileIndexerMediaOptInLogsOncePerDirectoryWithoutFileNames(t *testing.T) {
	f := newMediaOptInFixture(t, false, true, true, []string{".txt"})
	f.write(t, "private-holiday.jpg", tinyIndexerJPEG(t))
	f.write(t, "private-call.mp3", []byte("ID3 not really audio"))

	for i := 0; i < 3; i++ {
		f.scan(t)
	}
	var matching []string
	for _, line := range strings.Split(f.logs.String(), "\n") {
		if strings.Contains(line, "allow_media_analysis") {
			matching = append(matching, line)
		}
	}
	if len(matching) != 1 {
		t.Fatalf("got %d log lines about the missing permission, want exactly 1:\n%s", len(matching), f.logs.String())
	}
	if !strings.Contains(matching[0], "level=INFO") || !strings.Contains(matching[0], "mediafolder") {
		t.Fatalf("log line should be Info and name the directory: %s", matching[0])
	}
	if strings.Contains(f.logs.String(), "private-") {
		t.Fatalf("the log leaks file names:\n%s", f.logs.String())
	}

	quiet := newMediaOptInFixture(t, false, false, false, []string{".txt"})
	quiet.write(t, "photo.jpg", tinyIndexerJPEG(t))
	quiet.scan(t)
	if strings.Contains(quiet.logs.String(), "allow_media_analysis") {
		t.Fatalf("a folder is reported although no media capability is enabled:\n%s", quiet.logs.String())
	}
}

func (f *mediaOptInFixture) setDirs(dirs ...config.IndexingDirectory) {
	f.cfgMu.Lock()
	f.cfg.Indexing.Directories = dirs
	f.cfgMu.Unlock()
}

func (f *mediaOptInFixture) scanDir(t *testing.T, dir string) (total, indexed int, errs []string) {
	t.Helper()
	return f.fi.scanDirectory(context.Background(), dir, IndexerCollection)
}

// A parent entry also scans its configured subfolders, so the permission is the one of
// the deepest configured directory that contains the file, not the one of the scan root.
func TestFileIndexerMediaOptInFollowsDeepestConfiguredDirectory(t *testing.T) {
	t.Run("denied subfolder of an allowed parent", func(t *testing.T) {
		f := newMediaOptInFixture(t, true, true, false, []string{".txt"})
		private := filepath.Join(f.dir, "private")
		if err := os.MkdirAll(private, 0o755); err != nil {
			t.Fatal(err)
		}
		f.setDirs(config.IndexingDirectory{Path: f.dir, AllowMediaAnalysis: true}, config.IndexingDirectory{Path: private})
		open := f.write(t, "open.jpg", tinyIndexerJPEG(t))
		secret := f.write(t, filepath.Join("private", "secret.jpg"), tinyIndexerJPEG(t))

		// Several polls of both entries: the image of the denied folder is never sent and
		// the two scans do not take turns indexing and removing it.
		for i := 0; i < 2; i++ {
			if total, indexed, errs := f.scanDir(t, f.dir); total != 1 || len(errs) != 0 || (i == 0 && indexed != 1) {
				t.Fatalf("parent scan %d total=%d indexed=%d errors=%v, want only open.jpg", i, total, indexed, errs)
			}
			if total, indexed, errs := f.scanDir(t, private); total != 0 || indexed != 0 || len(errs) != 0 {
				t.Fatalf("subfolder scan %d total=%d indexed=%d errors=%v, want nothing", i, total, indexed, errs)
			}
		}
		if vision, _ := f.providerCalls(); vision != 1 {
			t.Fatalf("vision calls = %d, want 1 (open.jpg only)", vision)
		}
		if !f.tracked(t, open) || f.tracked(t, secret) {
			t.Fatalf("tracked open=%v secret=%v, want true/false", f.tracked(t, open), f.tracked(t, secret))
		}
		if counted, _ := f.fi.countIndexableFiles(context.Background(), f.dir); counted != 1 {
			t.Fatalf("countIndexableFiles(parent) = %d, want 1: the denied subfolder is not countable", counted)
		}
	})

	t.Run("allowed subfolder of a denied parent", func(t *testing.T) {
		f := newMediaOptInFixture(t, false, true, false, []string{".txt"})
		shared := filepath.Join(f.dir, "shared")
		if err := os.MkdirAll(shared, 0o755); err != nil {
			t.Fatal(err)
		}
		f.setDirs(config.IndexingDirectory{Path: f.dir}, config.IndexingDirectory{Path: shared, AllowMediaAnalysis: true})
		f.write(t, "closed.jpg", tinyIndexerJPEG(t))
		f.write(t, filepath.Join("shared", "open.jpg"), tinyIndexerJPEG(t))

		if total, indexed, errs := f.scanDir(t, f.dir); total != 1 || indexed != 1 || len(errs) != 0 {
			t.Fatalf("parent scan total=%d indexed=%d errors=%v, want only shared/open.jpg", total, indexed, errs)
		}
		if vision, _ := f.providerCalls(); vision != 1 {
			t.Fatalf("vision calls = %d, want 1", vision)
		}
	})
}

// Two entries for the same path with different flags must not fail open, whatever their order.
func TestFileIndexerMediaOptInDuplicateEntriesFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		allows []bool
	}{
		{"allowed first", []bool{true, false}},
		{"denied first", []bool{false, true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newMediaOptInFixture(t, false, true, true, []string{".txt"})
			f.setDirs(
				config.IndexingDirectory{Path: f.dir, AllowMediaAnalysis: tc.allows[0]},
				config.IndexingDirectory{Path: f.dir, AllowMediaAnalysis: tc.allows[1]},
			)
			path := f.write(t, "photo.jpg", tinyIndexerJPEG(t))
			if total, indexed, errs := f.scan(t); total != 0 || indexed != 0 || len(errs) != 0 {
				t.Fatalf("scan total=%d indexed=%d errors=%v", total, indexed, errs)
			}
			if vision, embed := f.providerCalls(); vision != 0 || embed != 0 {
				t.Fatalf("provider calls vision=%d embed=%d, want 0 when any entry for the path denies", vision, embed)
			}
			if f.tracked(t, path) {
				t.Fatal("the image was indexed")
			}
		})
	}
}

// A denied image must not leave the metadata row of the single-file path behind.
func TestIndexFileDeniedMediaWritesNoMetadata(t *testing.T) {
	f := newMediaOptInFixture(t, false, true, false, []string{".txt"})
	path := f.write(t, "photo.jpg", tinyIndexerJPEG(t))

	_, err := f.fi.IndexFile(context.Background(), f.cfg.Indexing.Directories[0], path, map[string]string{"archive_title": "Holiday"})
	if !errors.Is(err, ErrNoIndexableContent) {
		t.Fatalf("IndexFile error = %v, want ErrNoIndexableContent", err)
	}
	metadata, err := f.stm.GetFileIndexMetadata(path, IndexerCollection)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 0 {
		t.Fatalf("metadata of a denied image was stored: %v", metadata)
	}
	if vision, embed := f.providerCalls(); vision != 0 || embed != 0 {
		t.Fatalf("provider calls vision=%d embed=%d, want 0", vision, embed)
	}
}

func TestMediaAnalysisAllowedForFileResolvesTheDeepestConfiguredDirectory(t *testing.T) {
	root := t.TempDir()
	at := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	fi := NewFileIndexer(&config.Config{}, &sync.RWMutex{}, &fakeIndexerVectorDB{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	fi.cfg.Indexing.Directories = []config.IndexingDirectory{
		{Path: at("media"), AllowMediaAnalysis: true},
		{Path: at("media", "private")},
		{Path: at("media", "private", "shared"), AllowMediaAnalysis: true},
		{Path: at("dup"), AllowMediaAnalysis: true},
		{Path: at("dup")},
	}

	for _, tc := range []struct {
		name string
		path string
		want bool
	}{
		{"inside an allowed directory", at("media", "a.jpg"), true},
		{"denied subfolder of an allowed parent", at("media", "private", "b.jpg"), false},
		{"allowed folder below a denied one", at("media", "private", "shared", "deep", "c.jpg"), true},
		{"a sibling that only shares the name prefix", at("media-private", "d.jpg"), false},
		{"outside every configured directory", at("elsewhere", "e.jpg"), false},
		{"duplicate entries with different flags", at("dup", "f.jpg"), false},
		{"relative path is resolved", filepath.Join(root, "media", "..", "media", "g.jpg"), true},
		{"empty path", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fi.mediaAnalysisAllowedForFile(tc.path); got != tc.want {
				t.Fatalf("mediaAnalysisAllowedForFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}

	if runtime.GOOS == "windows" {
		if !fi.mediaAnalysisAllowedForFile(strings.ToUpper(at("media", "H.JPG"))) {
			t.Fatal("a Windows path that differs only in case must match its directory")
		}
	}

	// A configured filesystem root contains every file below it.
	fi.cfg.Indexing.Directories = []config.IndexingDirectory{{Path: filepath.VolumeName(root) + string(filepath.Separator), AllowMediaAnalysis: true}}
	if !fi.mediaAnalysisAllowedForFile(at("any", "i.jpg")) {
		t.Fatal("a file below a configured filesystem root was denied")
	}
}

// The server publishes a new config snapshot on every config save; the indexer must
// follow it, otherwise a revoked permission would keep sending media.
func TestFileIndexerReplaceConfigLockedAppliesNewFlagsAndExtensions(t *testing.T) {
	f := newMediaOptInFixture(t, true, true, false, []string{".txt"})
	path := f.write(t, "photo.jpg", tinyIndexerJPEG(t))
	if _, indexed, errs := f.scan(t); indexed != 1 || len(errs) != 0 {
		t.Fatalf("permitted scan indexed=%d errors=%v", indexed, errs)
	}

	next := f.cfg.Clone()
	next.Indexing.Directories[0].AllowMediaAnalysis = false
	next.Indexing.Extensions = []string{".txt", ".md"}
	f.cfgMu.Lock()
	f.fi.ReplaceConfigLocked(next)
	f.cfgMu.Unlock()

	// The old snapshot is no longer read: granting the permission on it changes nothing.
	f.setAllow(true)
	f.write(t, "note.md", []byte("a note about the holiday"))
	total, indexed, errs := f.scan(t)
	if total != 1 || indexed != 1 || len(errs) != 0 {
		t.Fatalf("scan after the swap total=%d indexed=%d errors=%v, want only the new .md file", total, indexed, errs)
	}
	if f.tracked(t, path) {
		t.Fatal("the image is still tracked after the new snapshot revoked the permission")
	}
	if vision, _ := f.providerCalls(); vision != 1 {
		t.Fatalf("vision calls = %d, want only the one from before the swap", vision)
	}
	if counted, _ := f.fi.countIndexableFiles(context.Background(), f.dir); counted != 1 {
		t.Fatalf("countIndexableFiles after the swap = %d, want 1", counted)
	}

	// A nil snapshot is ignored.
	f.cfgMu.Lock()
	f.fi.ReplaceConfigLocked(nil)
	f.cfgMu.Unlock()
	if _, _, errs := f.scan(t); len(errs) != 0 {
		t.Fatalf("scan after a nil swap errors=%v", errs)
	}
}
