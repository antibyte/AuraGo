package agent

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
)

func TestPDFExtractionFailureNeverStartsSummary(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"unexpected summary"}}]}`)
	}))
	defer provider.Close()
	dir := t.TempDir()
	bad := "%PDF-1.4\n" + strings.Repeat("x", 6000) + "\nstartxref\n9\n%%EOF\n"
	if err := os.WriteFile(filepath.Join(dir, "malformed.pdf"), []byte(bad), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir, cfg.Directories.SkillsDir = dir, dir
	cfg.LLM.ProviderType, cfg.LLM.BaseURL, cfg.LLM.Model, cfg.LLM.APIKey = "custom", provider.URL+"/v1", "fixture-model", "fixture-key"
	cfg.Tools.PDFExtractor.Enabled, cfg.Tools.PDFExtractor.SummaryMode = true, true
	cfg.Tools.PDFExtractor.SummaryBaseURL, cfg.Tools.PDFExtractor.SummaryModel, cfg.Tools.PDFExtractor.SummaryAPIKey = cfg.LLM.BaseURL, cfg.LLM.Model, cfg.LLM.APIKey
	dc := &DispatchContext{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	out, handled := dispatchComm(context.Background(), ToolCall{Action: "execute_skill", Skill: "pdf_extractor", SkillArgs: map[string]interface{}{"filepath": "malformed.pdf"}}, dc)
	if !handled || !strings.Contains(out, `"status":"error"`) || calls.Load() != 0 {
		t.Fatalf("handled=%v calls=%d output length=%d", handled, calls.Load(), len(out))
	}
}
