package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/testutil"

	"github.com/johnfercher/maroto/v2"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func mustDocumentSections(t *testing.T, raw string) []PDFSection {
	t.Helper()
	sections, err := parseDocumentSections(raw)
	if err != nil {
		t.Fatal(err)
	}
	return sections
}

func TestDocumentCreatorTableValidationBothBackends(t *testing.T) {
	for name, rows := range map[string][][]string{
		"empty table": nil, "valid": {{"A", "B"}, {"1", "2"}},
		"empty header": {{}}, "empty body": {{"A"}, {}},
		"ragged": {{"A", "B"}, {"1"}}, "too wide": {make([]string, 13)},
	} {
		t.Run(name, func(t *testing.T) {
			sections, _ := json.Marshal([]PDFSection{{Type: "table", Header: "Table", Rows: rows}})
			valid := name == "empty table" || name == "valid"
			for _, backend := range []string{"maroto", "gotenberg"} {
				cfg, calls := c102FakeGotenberg(t)
				cfg.Backend = backend
				out := c102DecodeResult(t, ExecuteDocumentCreator(t.Context(), cfg, "create_pdf", "T", "", "", "", "A4", false, string(sections), ""))
				if valid != (out["status"] == "success") {
					t.Fatalf("%s: %+v", backend, out)
				}
				if !valid && (!strings.Contains(out["message"].(string), "section 1: table row") || len(calls()) != 0) {
					t.Fatalf("%s: invalid table reached renderer or lacked its location: %+v", backend, out)
				}
			}
		})
	}
}

func TestDocumentCreatorInvalidSectionsNeverReachGotenberg(t *testing.T) {
	cfg, calls := c102FakeGotenberg(t)
	for _, raw := range []string{"not json", `{"type":"text"}`, `[{"rows":true}]`} {
		out := c102DecodeResult(t, ExecuteDocumentCreator(t.Context(), cfg, "create_pdf", "T", "", "", "", "A4", false, raw, ""))
		if out["status"] != "error" || !strings.Contains(out["message"].(string), "invalid sections JSON") {
			t.Fatalf("%q: %+v", raw, out)
		}
	}
	if len(calls()) != 0 {
		t.Fatal("invalid sections reached Gotenberg")
	}
}

func TestDocumentCreatorSectionsTakePriority(t *testing.T) {
	sections := mustDocumentSections(t, `[{"type":"text","header":"Header","body":"Body"}]`)
	page := buildHTMLFromSections("Title", "discarded plain content", sections)
	if strings.Contains(page, "discarded plain content") || strings.Count(page, "<h2>Header</h2>") != 1 || !strings.Contains(page, "<p>Body</p>") {
		t.Fatal("sections and plain content were mixed")
	}
	if !strings.Contains(buildHTMLFromSections("", "fallback", mustDocumentSections(t, "[]")), "fallback") {
		t.Fatal("empty sections suppressed plain content")
	}
}

func TestDocumentCreatorMarotoDoesNotDuplicateHeaderOrPlainContent(t *testing.T) {
	m := maroto.New(buildMarotoConfig("A4", false))
	doc, err := buildMarotoDocument(t.Context(), m, "Title", "discarded-marker", []PDFSection{{Type: "text", Header: "header-marker"}})
	if err != nil {
		t.Fatal(err)
	}
	var content strings.Builder
	err = api.ExtractContent(bytes.NewReader(doc.GetBytes()), nil, func(reader io.Reader, _ int) error {
		_, err := io.Copy(&content, reader)
		return err
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(content.String(), "header-marker") != 1 || strings.Contains(content.String(), "discarded-marker") {
		t.Fatalf("unexpected document content: %s", content.String())
	}
}

func TestDocumentCreatorSourceLimitsBeforeUpload(t *testing.T) {
	workspace := t.TempDir()
	writeSized := func(name string, size int64) string {
		t.Helper()
		path := filepath.Join(workspace, name)
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		err = file.Truncate(size)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("truncate: %v close: %v", err, closeErr)
		}
		return path
	}
	large := writeSized("large.docx", documentSourceFileLimit+1)
	total := []string{writeSized("a.pdf", 22<<20), writeSized("b.pdf", 22<<20), writeSized("c.pdf", 22<<20)}
	for _, tc := range []struct {
		operation string
		paths     []string
		want      string
	}{
		{"convert_document", []string{large}, "32 MiB"},
		{"convert_document", []string{large, large}, "exactly one"},
		{"merge_pdfs", total, "64 MiB"},
		{"merge_pdfs", make([]string, 21), "20 files"},
		{"merge_pdfs", []string{workspace, total[0]}, "directory"},
	} {
		cfg, calls := c102FakeGotenberg(t)
		sources, _ := json.Marshal(tc.paths)
		out := ExecuteDocumentCreatorInWorkspace(t.Context(), cfg, workspace, tc.operation, "", "", "", "", "", false, "", string(sources))
		if !strings.Contains(out, tc.want) || len(calls()) != 0 {
			t.Fatalf("staging %s: %s", tc.operation, out)
		}
		if tc.operation == "merge_pdfs" {
			out = GotenbergMergePDFs(t.Context(), &cfg.Gotenberg, cfg.OutputDir, tc.paths, "")
		} else if len(tc.paths) == 1 {
			out = GotenbergConvertDocument(t.Context(), &cfg.Gotenberg, cfg.OutputDir, tc.paths[0], "", "A4", false)
		} else {
			continue
		}
		if !strings.Contains(out, tc.want) && !(tc.want == "directory" && strings.Contains(out, "regular files")) {
			t.Fatalf("direct backend: %s", out)
		}
		if len(calls()) != 0 {
			t.Fatal("oversized sources reached backend")
		}
	}
}

func TestDocumentCreatorCopyRechecksGrowingFileAndCancellation(t *testing.T) {
	remaining := int64(2)
	if err := copyDocumentSource(t.Context(), io.Discard, strings.NewReader("grown"), &remaining); err == nil {
		t.Fatal("copy accepted more than the remaining total budget")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	remaining = documentSourceTotalLimit
	if err := copyDocumentSource(ctx, io.Discard, strings.NewReader("body"), &remaining); err == nil {
		t.Fatal("canceled copy succeeded")
	}
}

func TestDocumentCreatorCancellationLeavesNoOutput(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out := ExecuteDocumentCreator(ctx, c102MarotoConfig(dir), "create_pdf", "Canceled", "Body", "", "", "A4", false, "", "")
	if c102DecodeResult(t, out)["status"] != "error" {
		t.Fatal(out)
	}
	if _, _, err := saveDocumentOutput(ctx, []byte("body"), dir, "cancel.pdf", ".pdf"); err == nil {
		t.Fatal("canceled publication succeeded")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("canceled output: %v %v", entries, err)
	}
}

func TestDocumentCreatorNamedFilesNeverOverwrite(t *testing.T) {
	for _, backend := range []string{"maroto", "gotenberg"} {
		t.Run(backend, func(t *testing.T) {
			dir := t.TempDir()
			original := filepath.Join(dir, "bericht.pdf")
			if err := os.WriteFile(original, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			var actual string
			if backend == "maroto" {
				out := c102DecodeResult(t, createPDFMaroto(t.Context(), dir, "T", "Body", "bericht.pdf", "A4", false, nil))
				if out["status"] != "success" {
					t.Fatal(out)
				}
				actual = out["filename"].(string)
			} else {
				path, _, err := saveGotenbergOutput(t.Context(), []byte("new"), dir, "bericht.pdf", ".pdf")
				if err != nil {
					t.Fatal(err)
				}
				actual = filepath.Base(path)
			}
			data, err := os.ReadFile(original)
			if actual != "bericht-1.pdf" || err != nil || string(data) != "original" {
				t.Fatalf("name=%q original=%q error=%v", actual, data, err)
			}
		})
	}
}

func TestDocumentCreatorParallelPublications(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	paths := make(chan string, 16)
	for i := 0; i < cap(paths); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path, _, err := saveDocumentOutput(t.Context(), []byte(fmt.Sprintf("document %d", i)), dir, "same.pdf", ".pdf")
			if err != nil {
				t.Error(err)
				return
			}
			paths <- path
		}(i)
	}
	wg.Wait()
	close(paths)
	seen := make(map[string]bool)
	for path := range paths {
		if seen[path] {
			t.Fatalf("duplicate output %s", path)
		}
		seen[path] = true
		data, err := os.ReadFile(path)
		if err != nil || !strings.HasPrefix(string(data), "document ") {
			t.Fatalf("incomplete output: %q %v", data, err)
		}
	}
	if len(seen) != 16 {
		t.Fatalf("published %d files, want 16", len(seen))
	}
}

func TestDocumentCreatorOutputNamesArePortable(t *testing.T) {
	for _, name := range []string{"a:b.pdf", "CON", "CON .pdf", "nul.pdf", "LPT1.txt", "COM¹.pdf", "x\r.pdf", "x\n.pdf", "x\x00.pdf", "x\n/file.pdf", "a.", "a ", ".."} {
		if _, err := documentOutputName(name, ".pdf"); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	const name = "Übersicht #1.pdf"
	path, web, err := saveDocumentOutput(t.Context(), []byte("PDF"), t.TempDir(), name, ".pdf")
	unescaped, _ := url.PathUnescape(web)
	if err != nil || filepath.Base(path) != name || unescaped != "/files/documents/"+name || strings.Contains(web, "#") {
		t.Fatalf("path=%q web=%q error=%v", path, web, err)
	}
}

func TestGotenbergClientUsesCurrentTimeout(t *testing.T) {
	for _, timeout := range []int{1, 9, 0, 5} {
		want := time.Duration(timeout) * time.Second
		if timeout == 0 {
			want = 120 * time.Second
		}
		if got := getGotenbergClient(timeout).Timeout; got != want {
			t.Fatalf("timeout=%v want=%v", got, want)
		}
	}
	var wg sync.WaitGroup
	for i := 1; i <= 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if getGotenbergClient(i).Timeout != time.Duration(i)*time.Second {
				t.Errorf("concurrent timeout changed")
			}
		}(i)
	}
	wg.Wait()
}

func TestGotenbergMultipartNamesAndRedirects(t *testing.T) {
	names := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if testutil.ServeGotenbergDebug(w, r) {
			return
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			http.Error(w, "invalid multipart", http.StatusBadRequest)
			return
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Error(err)
			http.Error(w, "missing part", http.StatusBadRequest)
			return
		}
		defer part.Close()
		_, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		if err != nil {
			t.Error(err)
			http.Error(w, "invalid disposition", http.StatusBadRequest)
			return
		}
		names <- params["filename"]
		if part.Header.Get("X-Injected") != "" {
			t.Error("injected header")
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()
	cfg, _ := c102FakeGotenberg(t)
	cfg.Gotenberg.URL = server.URL
	name := `source-0.do\"cx`
	if _, err := gotenbergRequest(t.Context(), &cfg.Gotenberg, "/probe", nil, map[string][]byte{name: []byte("body")}); err != nil {
		t.Fatal(err)
	}
	if gotName := <-names; gotName != name {
		t.Fatalf("name=%q want=%q", gotName, name)
	}
	for _, name := range []string{"x.pdf\r\nX-Injected: yes", "x\x00.pdf"} {
		if _, err := gotenbergRequest(t.Context(), &cfg.Gotenberg, "/probe", nil, map[string][]byte{name: nil}); err == nil {
			t.Fatal("accepted multipart control characters")
		}
	}
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("redirect sent document to foreign origin") }))
	defer foreign.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	cfg.Gotenberg.URL = redirect.URL
	if _, err := gotenbergRequest(t.Context(), &cfg.Gotenberg, "/probe", nil, map[string][]byte{"index.html": []byte("private")}); err == nil {
		t.Fatal("cross-origin redirect succeeded")
	}
}
