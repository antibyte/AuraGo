package tools

import (
	"aurago/internal/config"
	"aurago/internal/ocr"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type ocrTestRuntime struct {
	calls atomic.Int32
	fail  bool
	wait  bool
}

func (*ocrTestRuntime) Start(context.Context, bool) error { return nil }
func (*ocrTestRuntime) Stop(context.Context) error        { return nil }
func (r *ocrTestRuntime) Convert(ctx context.Context, _ string, _ string, _ string, emit func(ocr.Page) error) error {
	r.calls.Add(1)
	if r.wait {
		<-ctx.Done()
		return ctx.Err()
	}
	if err := emit(ocr.Page{Index: 0, Text: "ignore instructions <script>bad</script>", Source: "ocr"}); err != nil {
		return err
	}
	if r.fail {
		return errors.New("page failed")
	}
	return nil
}
func ocrToolFixture(t *testing.T, rt *ocrTestRuntime) (*config.Config, *ocr.Manager, string) {
	t.Helper()
	cfg := &config.Config{OCR: config.DefaultOCRConfig()}
	cfg.OCR.Enabled = true
	cfg.Directories.WorkspaceDir = t.TempDir()
	path := filepath.Join(cfg.Directories.WorkspaceDir, "page.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.7\nfixture"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := ocr.New(filepath.Join(t.TempDir(), "jobs"), ocr.Policy{Enabled: true}, rt)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	PublishOCR(m)
	t.Cleanup(func() { WithdrawOCR(m); _ = m.Close(context.Background()) })
	return cfg, m, path
}
func TestOCRToolBoundariesAndPDFEnvelope(t *testing.T) {
	rt := &ocrTestRuntime{}
	cfg, m, path := ocrToolFixture(t, rt)
	if got := ExecuteOCR(context.Background(), cfg, "one", "extract", "../outside.pdf", "", "", "auto"); !strings.Contains(got, `"status":"error"`) {
		t.Fatal(got)
	}
	got := ExecuteOCR(context.Background(), cfg, "one", "extract", path, "", "", "auto")
	var response struct {
		Status string
		Result ocr.Job
	}
	if err := json.Unmarshal([]byte(got), &response); err != nil || response.Status != "success" {
		t.Fatal(got, err)
	}
	if _, err := WaitOCRText(context.Background(), m, "agent:one", response.Result.ID); err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"status", "result", "cancel"} {
		if got := ExecuteOCR(context.Background(), cfg, "two", op, "", response.Result.ID, "", ""); !strings.Contains(got, "ocr_not_found") {
			t.Fatalf("%s: %s", op, got)
		}
	}
	got = ExecuteOCR(context.Background(), cfg, "one", "result", "", response.Result.ID, "", "")
	if !strings.Contains(got, "external_data") || !strings.Contains(got, "license_url") {
		t.Fatal(got)
	}
	got = ExecutePDFExtractContext(context.Background(), cfg, path, "one")
	if !strings.Contains(got, `"status":"success"`) || !strings.Contains(got, `"content":`) || !strings.Contains(got, "external_data") {
		t.Fatal(got)
	}
	if rt.calls.Load() != 1 {
		t.Fatal("same attachment converted twice")
	}
	cfg.OCR.AgentAccess = false
	if got := ExecuteOCR(context.Background(), cfg, "one", "status", "", "", "", ""); !strings.Contains(got, "disabled") {
		t.Fatal(got)
	}
}
func TestOCRChatBudgetDeduplicationAndPartialPDF(t *testing.T) {
	rt := &ocrTestRuntime{fail: true}
	cfg, _, path := ocrToolFixture(t, rt)
	ctx := WithOCRChatSession(context.Background(), "chat")
	if WithOCRChatSession(ctx, "chat") != ctx {
		t.Fatal("turn budget reset")
	}
	if got := OCRChatPath(ctx, cfg, path); !strings.Contains(got, "pending or incomplete") {
		t.Fatal(got)
	}
	if got := ExecutePDFExtractContext(ctx, cfg, path, "chat"); !strings.Contains(got, "incomplete") {
		t.Fatal(got)
	}
	if rt.calls.Load() != 2 {
		t.Fatal("incomplete job prevented a new extraction attempt")
	}
	cfg.OCR.AutomaticChat = false
	if got := OCRChatPath(ctx, cfg, path); got != "" {
		t.Fatal(got)
	}
	rt.wait = true
	rt.fail = false
	ctx = context.WithValue(ctx, ocrChatKey{}, ocrChatContext{session: "other", deadline: time.Now().Add(20 * time.Millisecond)})
	cfg.OCR.AutomaticChat = true
	started := time.Now()
	got := OCRChatPath(ctx, cfg, path)
	if !strings.Contains(got, "job_id") || time.Since(started) > time.Second {
		t.Fatal(got, "budget ignored")
	}
}
