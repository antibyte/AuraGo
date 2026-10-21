package ocr

import (
	"aurago/internal/dockerutil"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func textPDF(text string) []byte {
	var data bytes.Buffer
	data.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	stream := "BT /F1 18 Tf 50 750 Td (" + text + ") Tj ET"
	for i, object := range []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
	} {
		offsets = append(offsets, data.Len())
		fmt.Fprintf(&data, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	start := data.Len()
	fmt.Fprintf(&data, "xref\n0 6\n0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&data, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&data, "trailer << /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", start)
	return data.Bytes()
}
func TestOCRDockerSmoke(t *testing.T) {
	image := os.Getenv("AURAGO_OCR_SMOKE_IMAGE")
	if image == "" {
		t.Skip("set AURAGO_OCR_SMOKE_IMAGE to a locally built immutable image ID")
	}
	policy := Policy{Enabled: true, DockerEnabled: true, DockerHost: os.Getenv("DOCKER_HOST")}
	policy.InstanceKey = id()
	policy.InDocker = os.Getenv("AURAGO_OCR_SMOKE_IN_DOCKER") == "1"
	self := func(ctx context.Context, host string) (string, error) {
		name, err := os.Hostname()
		if err != nil {
			return "", err
		}
		var info struct{ ID string }
		_, err = dockerutil.NewClient(host, 5*time.Second).DoJSON(ctx, http.MethodGet, "containers/"+name+"/json", nil, &info)
		return info.ID, err
	}
	docker := NewDockerRuntime(func() Policy { return policy }, self, image)
	m, err := New(filepath.Join(t.TempDir(), "jobs"), policy, docker)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := m.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := m.Start(ctx, false); err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(docker.baseURL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal("private service accepted unauthenticated request")
	}
	probe := filepath.Join(t.TempDir(), "probe.pdf")
	_ = os.WriteFile(probe, textPDF("AuraGo digital document 24680"), 0600)
	if err := docker.Convert(ctx, probe, "application/pdf", "auto", func(Page) error { return nil }); err != nil {
		t.Fatal("direct protocol check:", err)
	}
	pdf, err := m.Stage("smoke", "digital.pdf", bytes.NewReader(textPDF("AuraGo digital document 24680")))
	if err != nil {
		t.Fatal(err)
	}
	job, err := m.Submit("smoke", []string{pdf.ID}, "auto")
	if err != nil {
		t.Fatal(err)
	}
	waitSmoke(t, m, "smoke", job.ID)
	result, err := m.Result("smoke", job.ID, pdf.ID)
	if err != nil || len(result.Pages) != 1 || !strings.Contains(result.Pages[0].Text, "24680") {
		t.Fatalf("digital PDF: %+v %v", result, err)
	}
	model.ConfigPath = "disable"
	protected := filepath.Join(t.TempDir(), "protected.pdf")
	encryption := model.NewDefaultConfiguration()
	encryption.UserPW = "synthetic-ocr-password"
	encryption.OwnerPW = "synthetic-owner-password"
	if err := api.EncryptFile(probe, protected, encryption); err != nil {
		t.Fatal(err)
	}
	if err := docker.Convert(ctx, protected, "application/pdf", "auto", func(Page) error { t.Fatal("protected PDF yielded a page"); return nil }); err == nil {
		t.Fatal("protected PDF accepted without password")
	}
	if dir := os.Getenv("AURAGO_OCR_SMOKE_DIR"); dir != "" {
		for _, name := range []string{"scan.png", "rotated.png", "pages.tiff"} {
			t.Run(name, func(t *testing.T) {
				file, err := os.Open(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				staged, err := m.Stage("smoke", name, file)
				if err != nil {
					t.Fatal(err)
				}
				job, err := m.Submit("smoke", []string{staged.ID}, "ocr")
				if err != nil {
					t.Fatal(err)
				}
				waitSmoke(t, m, "smoke", job.ID)
				result, err := m.Result("smoke", job.ID, staged.ID)
				if err != nil {
					t.Fatal(err)
				}
				want := 1
				if name == "pages.tiff" {
					want = 2
				}
				if len(result.Pages) != want || !strings.Contains(result.Pages[0].Text, "12345") {
					var texts []string
					for _, page := range result.Pages {
						texts = append(texts, page.Text)
					}
					t.Fatalf("OCR %s: state=%s pages=%d text=%q error=%s", name, result.State, len(result.Pages), texts, result.Error)
				}
				if name == "scan.png" && !strings.Contains(result.Pages[0].Text, "Grüße") {
					t.Fatalf("German glyph recognition failed: %q", result.Pages[0].Text)
				}
			})
		}
		t.Run("mixed_and_page_limit", func(t *testing.T) {
			var pages []Page
			if err := docker.Convert(ctx, filepath.Join(dir, "mixed.pdf"), "application/pdf", "auto", func(p Page) error { pages = append(pages, p); return nil }); err != nil {
				t.Fatal(err)
			}
			if len(pages) != 2 || !strings.Contains(pages[0].Text, "24680") || !strings.Contains(pages[1].Text, "12345") {
				t.Fatalf("mixed PDF pages: %+v", pages)
			}
			if err := docker.Convert(ctx, filepath.Join(dir, "too-many.tiff"), "image/tiff", "ocr", func(Page) error { t.Fatal("oversized TIFF decoded"); return nil }); err == nil {
				t.Fatal("101-page TIFF accepted")
			}
		})
		t.Run("cancel_before_first_page", func(t *testing.T) {
			cancelCtx, cancelWorker := context.WithCancel(ctx)
			defer cancelWorker()
			done := make(chan error, 1)
			go func() {
				done <- docker.Convert(cancelCtx, filepath.Join(dir, "scan.png"), "image/png", "ocr", func(Page) error { return nil })
			}()
			workerSeen := false
			for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
				var top struct{ Processes [][]string }
				if _, err := docker.engine.DoJSON(ctx, "GET", "containers/"+docker.container+"/top", nil, &top); err != nil {
					t.Fatal(err)
				}
				for _, process := range top.Processes {
					if strings.Contains(strings.Join(process, " "), "worker.py") {
						workerSeen = true
					}
				}
				if workerSeen {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !workerSeen {
				t.Fatal("worker not observed")
			}
			cancelWorker()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatal("cancel did not interrupt conversion", err)
			}
			for deadline := time.Now().Add(3 * time.Second); ; {
				var top struct{ Processes [][]string }
				if _, err := docker.engine.DoJSON(ctx, "GET", "containers/"+docker.container+"/top", nil, &top); err != nil {
					t.Fatal(err)
				}
				alive := false
				for _, process := range top.Processes {
					alive = alive || strings.Contains(strings.Join(process, " "), "worker.py")
				}
				if !alive {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("worker survived cancel")
				}
				time.Sleep(20 * time.Millisecond)
			}
		})
	}
	t.Run("restart_reaps_only_owned_resources", func(t *testing.T) {
		previous := docker.container
		replacement := NewDockerRuntime(func() Policy { return policy }, self, image)
		defer replacement.Stop(context.Background())
		if err := replacement.Start(ctx, false); err != nil {
			t.Fatal(err)
		}
		if code, _ := replacement.engine.DoJSON(ctx, "GET", "containers/"+previous+"/json", nil, nil); code != 404 {
			t.Fatal("old parser survived restart", code)
		}
	})
}
func waitSmoke(t *testing.T, m *Manager, owner, key string) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		job, err := m.Get(owner, key)
		if err != nil {
			t.Fatal(err)
		}
		if job.FinishedAt != nil {
			if job.State != "completed" {
				t.Fatalf("OCR job failed: %+v", job)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("OCR smoke timeout")
}
