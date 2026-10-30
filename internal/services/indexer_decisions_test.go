package services

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
	"aurago/internal/memory"
)

// fakeIndexerDecisions is a scripted IndexerDecisionService.
type fakeIndexerDecisions struct {
	mu        sync.Mutex
	available bool
	kind      string
	textP     float64
	snapshots int
	evaluates int
}

func (f *fakeIndexerDecisions) Snapshot(_ *config.Config, id consumers.ID) consumers.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshots++
	if !f.available {
		return consumers.Snapshot{Consumer: id, Code: decisions.CodeImagesUnsupported}
	}
	return consumers.Snapshot{Consumer: id, Available: true, Code: consumers.CodeAvailable, ImagesAllowed: true,
		Evaluate: func(_ context.Context, r decisions.Request) (decisions.Response, error) {
			f.mu.Lock()
			f.evaluates++
			f.mu.Unlock()
			if len(r.Images) != 1 {
				panic("indexer must send exactly one image")
			}
			probabilities := map[string]float64{}
			for _, option := range indexerKindOptions {
				probabilities[option.ID] = 0.025
			}
			probabilities[f.kind] = 0.9
			p := f.textP
			return decisions.Response{Answers: map[string]decisions.Answer{
				"kind": {Type: "choice", Decision: f.kind, Probabilities: probabilities},
				"text": {Type: "noul", Noul: &p, Probabilities: map[string]float64{"yes": p, "no": 1 - p}},
			}}, nil
		}}
}

func (f *fakeIndexerDecisions) RefreshImageCapability(_ context.Context, cfg *config.Config, id consumers.ID) consumers.Snapshot {
	return f.Snapshot(cfg, id)
}

func (f *fakeIndexerDecisions) PrepareImage(data []byte, _ int) (decisions.Image, error) {
	return decisions.Image{Data: data, MIME: "image/jpeg"}, nil
}

type indexerDecisionFixture struct {
	fi      *FileIndexer
	vdb     *fakeIndexerVectorDB
	dir     string
	service *fakeIndexerDecisions
	mu      sync.Mutex
	prompts []string
}

func (f *indexerDecisionFixture) visionBodies() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.prompts...)
}

func newIndexerDecisionFixture(t *testing.T, allowMedia, agnes, attach bool, service *fakeIndexerDecisions) *indexerDecisionFixture {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stm.Close() })
	f := &indexerDecisionFixture{dir: t.TempDir(), vdb: &fakeIndexerVectorDB{}, service: service}
	vision := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.prompts = append(f.prompts, string(body))
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"description"}}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`))
	}))
	t.Cleanup(vision.Close)
	path := filepath.Join(f.dir, "shot.jpg")
	if err := os.WriteFile(path, tinyIndexerJPEG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = f.dir
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: f.dir, AllowMediaAnalysis: allowMedia}}
	cfg.Indexing.Extensions = []string{".jpg"}
	cfg.Indexing.IndexImages = true
	if agnes {
		cfg.Vision.ProviderType = "agnes"
		cfg.Vision.APIKey = "unused"
	} else {
		cfg.Vision.BaseURL = vision.URL
		cfg.Vision.APIKey = "vision-key"
		cfg.Vision.Model = "vision-model"
	}
	f.fi = NewFileIndexer(cfg, &sync.RWMutex{}, f.vdb, stm, logger)
	if attach {
		f.fi.SetDecisionService(service)
	}
	return f
}

func (f *indexerDecisionFixture) scan(t *testing.T) int {
	t.Helper()
	_, indexed, errs := f.fi.scanDirectory(context.Background(), f.dir, IndexerCollection)
	if len(errs) != 0 {
		t.Fatalf("scan errors: %v", errs)
	}
	return indexed
}

func (f *indexerDecisionFixture) stored() []string {
	f.vdb.mu.Lock()
	defer f.vdb.mu.Unlock()
	return append([]string(nil), f.vdb.stored...)
}

const screenshotHint = "Gib sichtbaren Text so"

func TestIndexerDecisionsPicksScreenshotPrompt(t *testing.T) {
	service := &fakeIndexerDecisions{available: true, kind: "screenshot", textP: 0.9}
	f := newIndexerDecisionFixture(t, true, false, true, service)
	if indexed := f.scan(t); indexed != 1 {
		t.Fatalf("indexed = %d", indexed)
	}
	bodies := f.visionBodies()
	if len(bodies) != 1 || !strings.Contains(bodies[0], screenshotHint) {
		t.Fatalf("vision bodies = %q", bodies)
	}
	if stored := f.stored(); len(stored) != 1 || !strings.Contains(stored[0], "Bildanalyse") {
		t.Fatalf("stored = %q", stored)
	}
	if service.evaluates != 1 {
		t.Fatalf("evaluates = %d", service.evaluates)
	}
}

func TestIndexerDecisionsAgnesKeepsCoarseEntry(t *testing.T) {
	service := &fakeIndexerDecisions{available: true, kind: "screenshot", textP: 0.9}
	f := newIndexerDecisionFixture(t, true, true, true, service)
	if indexed := f.scan(t); indexed != 1 {
		t.Fatalf("indexed = %d", indexed)
	}
	stored := f.stored()
	if len(stored) != 1 || !strings.Contains(stored[0], "Screenshot") || !strings.Contains(stored[0], "enthält Text: ja") {
		t.Fatalf("stored = %q", stored)
	}
	if len(f.visionBodies()) != 0 {
		t.Fatal("Agnes branch called the vision provider")
	}
	if f.scan(t); service.evaluates != 1 {
		t.Fatalf("unchanged rescan evaluated again: %d", service.evaluates)
	}
}

func TestIndexerDecisionsOffUnchanged(t *testing.T) {
	if got, want := indexImagePrompt("a.jpg", ""), "Analysiere dieses Bild detailliert. Beschreibe den Inhalt, erkennbare Texte, Objekte und relevante Details. Dateiname: a.jpg"; got != want {
		t.Fatalf("prompt = %q", got)
	}
	f := newIndexerDecisionFixture(t, true, false, false, nil)
	if indexed := f.scan(t); indexed != 1 {
		t.Fatalf("indexed = %d", indexed)
	}
	if bodies := f.visionBodies(); len(bodies) != 1 || strings.Contains(bodies[0], screenshotHint) {
		t.Fatalf("vision bodies = %q", bodies)
	}
	agnes := newIndexerDecisionFixture(t, true, true, false, nil)
	if indexed := agnes.scan(t); indexed != 0 || len(agnes.stored()) != 0 {
		t.Fatalf("Agnes without Decisions indexed=%d stored=%q", indexed, agnes.stored())
	}
}

func TestIndexerDecisionsImagesUnsupportedUnchanged(t *testing.T) {
	service := &fakeIndexerDecisions{available: false, kind: "screenshot", textP: 0.9}
	f := newIndexerDecisionFixture(t, true, false, true, service)
	f.scan(t)
	if bodies := f.visionBodies(); len(bodies) != 1 || strings.Contains(bodies[0], screenshotHint) {
		t.Fatalf("vision bodies = %q", bodies)
	}
	agnes := newIndexerDecisionFixture(t, true, true, true, service)
	if agnes.scan(t); len(agnes.stored()) != 0 || service.evaluates != 0 {
		t.Fatalf("stored=%q evaluates=%d", agnes.stored(), service.evaluates)
	}
}

func TestIndexerDecisionsRespectsMediaOptIn(t *testing.T) {
	service := &fakeIndexerDecisions{available: true, kind: "screenshot", textP: 0.9}
	f := newIndexerDecisionFixture(t, false, false, true, service)
	f.scan(t)
	if service.evaluates != 0 || len(f.visionBodies()) != 0 || len(f.stored()) != 0 {
		t.Fatalf("evaluates=%d vision=%d stored=%d", service.evaluates, len(f.visionBodies()), len(f.stored()))
	}
}

func TestIndexerDecisionsFingerprint(t *testing.T) {
	service := &fakeIndexerDecisions{available: true}
	f := newIndexerDecisionFixture(t, true, false, false, service)
	plain := f.fi.indexFingerprintForMode(fileIndexModeVision, true)
	f.fi.SetDecisionService(service)
	with := f.fi.indexFingerprintForMode(fileIndexModeVision, true)
	if with == plain || !strings.HasPrefix(with, plain[:len(fileIndexerFingerprint)]) {
		t.Fatalf("fingerprint did not change: %q vs %q", plain, with)
	}
	if got := f.fi.indexFingerprintForMode(fileIndexModeVision, false); got != plain {
		t.Fatalf("no media opt-in changed the fingerprint: %q", got)
	}
	if got := f.fi.indexFingerprintForMode(fileIndexModeText, true); strings.Contains(got, "decisions") {
		t.Fatalf("text mode fingerprint = %q", got)
	}
	service.available = false
	if got := f.fi.indexFingerprintForMode(fileIndexModeVision, true); got != plain {
		t.Fatalf("unavailable consumer changed the fingerprint: %q", got)
	}
}
