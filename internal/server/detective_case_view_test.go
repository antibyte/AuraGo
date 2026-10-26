package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/detective"
)

// The case payload stays bounded: no source excerpt and no finding text, while the
// per-source endpoint still serves the recorded evidence (audit 4.3).
func TestDetectiveCaseResponseStaysBounded(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = false
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Detective.Enabled = true
	s := &Server{Cfg: cfg}
	svc, err := detective.New(detective.Options{Path: filepath.Join(t.TempDir(), "view.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	s.Detective = svc
	excerpt := strings.Repeat("recorded evidence sentence. ", 2000)
	svc.SetRunner(detectiveTestRunner(func(ctx context.Context, job *detective.Session) error {
		_, done, err := job.Activate(ctx)
		if err != nil {
			return err
		}
		defer done()
		_, err = job.RecordSource("https://example.org/evidence", "Evidence", "web_scraper", excerpt, true)
		return err
	}))
	c, err := svc.Create(detective.Request{Topic: "bounded", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Start(c.ID, "start", "quick", "bounded-view"); err != nil {
		t.Fatal(err)
	}
	var sourceID string
	deadline := time.Now().Add(5 * time.Second)
	for sourceID == "" {
		page, archiveErr := svc.Archive(c.ID, "sources", 0, 20, "")
		if archiveErr != nil {
			t.Fatal(archiveErr)
		}
		if len(page.Sources) > 0 {
			sourceID = page.Sources[0].ID
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("source was never archived")
		}
		time.Sleep(10 * time.Millisecond)
	}

	r := httptest.NewRequest("GET", "/api/desktop/detective/cases/"+c.ID, nil)
	w := httptest.NewRecorder()
	s.handleDetective(w, r)
	if w.Code != 200 {
		t.Fatalf("case: %d %s", w.Code, w.Body.String())
	}
	var got detective.Case
	if err = json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Findings) != 0 {
		t.Fatalf("case payload carried %d findings", len(got.Findings))
	}
	if len(got.Sources) != 1 || got.Sources[0].Excerpt != "" {
		t.Fatalf("case payload carried a source excerpt: %+v", got.Sources)
	}
	if strings.Contains(w.Body.String(), "recorded evidence sentence") {
		t.Fatal("case response leaked evidence text")
	}

	r = httptest.NewRequest("GET", "/api/desktop/detective/cases/"+c.ID+"/source?id="+url.QueryEscape(sourceID), nil)
	w = httptest.NewRecorder()
	s.handleDetective(w, r)
	if w.Code != 200 {
		t.Fatalf("source: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "recorded evidence sentence") {
		t.Fatal("the source endpoint lost the excerpt")
	}
}
