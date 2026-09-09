package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/office"
	"github.com/sashabaranov/go-openai"
)

func writerComplexDOCX(t *testing.T) []byte {
	t.Helper()
	simple, err := office.EncodeDOCX(office.Document{Text: "A complete document"})
	if err != nil {
		t.Fatal(err)
	}
	parts, err := office.ReadDOCXParts(simple)
	if err != nil {
		t.Fatal(err)
	}
	parts["customXml/item1.xml"] = []byte("<preserved>review and unknown application metadata</preserved>")
	var buffer bytes.Buffer
	zw := zip.NewWriter(&buffer)
	for name, data := range parts {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
func TestDesktopWriterNativeDocumentPreservesAndLocks(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	data := writerComplexDOCX(t)
	handler := handleDesktopOfficeDocument(s)
	request := func(method, precondition, value string, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/desktop/office/document?representation=docx&path=Documents/native.docx", bytes.NewReader(body))
		if precondition != "" {
			req.Header.Set(precondition, value)
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}
	if got := request("PUT", "", "", data); got.Code != 428 {
		t.Fatalf("unguarded: %d %s", got.Code, got.Body)
	}
	created := request("PUT", "If-None-Match", "*", data)
	if created.Code != 200 {
		t.Fatalf("create: %d %s", created.Code, created.Body)
	}
	etag := created.Header().Get("ETag")
	if etag == "" {
		t.Fatal("No ETag")
	}
	read := request("GET", "", "", nil)
	if read.Code != 200 || !bytes.Equal(read.Body.Bytes(), data) {
		t.Fatal("Native read lost package bytes")
	}
	if got := request("PUT", "If-None-Match", "*", data); got.Code != 412 {
		t.Fatalf("create collision %d", got.Code)
	}
	if got := request("PUT", "If-Match", `"old"`, data); got.Code != 412 {
		t.Fatalf("stale update %d", got.Code)
	}
	if got := request("PUT", "If-Match", etag, []byte("not a zip")); got.Code != 400 {
		t.Fatalf("invalid docx %d", got.Code)
	}
	saved := request("PUT", "If-Match", etag, data)
	if saved.Code != 200 {
		t.Fatalf("matching update: %d %s", saved.Code, saved.Body)
	}
	legacy := doOfficeDocumentRequest(t, s, "PUT", "/api/desktop/office/document", map[string]interface{}{"path": "Documents/native.docx", "text": "flattened"})
	if legacy.Code == 200 {
		t.Fatal("Legacy API destroyed complex DOCX")
	}
	read = request("GET", "", "", nil)
	if !bytes.Equal(read.Body.Bytes(), data) {
		t.Fatal("Failed writes changed the file")
	}
	exp := httptest.NewRecorder()
	handleDesktopOfficeExport(s)(exp, httptest.NewRequest("GET", "/api/desktop/office/export?path=Documents/native.docx&format=docx", nil))
	if exp.Code != 200 || !bytes.Equal(exp.Body.Bytes(), data) {
		t.Fatal("DOCX download changed bytes")
	}
	exp = httptest.NewRecorder()
	handleDesktopOfficeExport(s)(exp, httptest.NewRequest("POST", "/api/desktop/office/export?format=txt", bytes.NewReader(data)))
	if exp.Code != 200 || !strings.Contains(exp.Body.String(), "A complete document") {
		t.Fatalf("Snapshot export: %d %s", exp.Code, exp.Body)
	}
}

type writerAssistClient struct {
	request openai.ChatCompletionRequest
	calls   int
	fail    bool
}

func (c *writerAssistClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.request = req
	c.calls++
	if c.fail {
		return openai.ChatCompletionResponse{}, errors.New("offline")
	}
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonStop, Message: openai.ChatCompletionMessage{Role: "assistant", Content: "A clearer sentence."}}}}, nil
}
func (c *writerAssistClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, errors.New("not used")
}
func TestDesktopWriterAssistIsIsolatedAndRevisionBound(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	client := &writerAssistClient{}
	s.LLMClient = client
	s.Cfg.LLM.Model = "test"
	call := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handleDesktopWriterAssist(s)(rec, httptest.NewRequest("POST", "/api/desktop/office/assist", strings.NewReader(body)))
		return rec
	}
	for _, bad := range []string{`{"action":"shell","text":"hello"}`, `{"action":"rewrite","text":""}`, `{"action":"translate","text":"hello"}`, `{"action":"custom","text":"hello"}`} {
		if got := call(bad); got.Code != 400 {
			t.Fatalf("validation %d: %s", got.Code, got.Body)
		}
	}
	if client.calls != 0 {
		t.Fatal("Invalid requests reached LLM")
	}
	got := call(`{"action":"rewrite","text":"An original sentence.","source_revision":42}`)
	if got.Code != 200 {
		t.Fatalf("assist %d: %s", got.Code, got.Body)
	}
	var body struct {
		Replacement string `json:"replacement"`
		Revision    uint64 `json:"source_revision"`
	}
	json.Unmarshal(got.Body.Bytes(), &body)
	if body.Revision != 42 || body.Replacement != "A clearer sentence." {
		t.Fatalf("%+v", body)
	}
	req := client.request
	if len(req.Tools) != 0 || len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
		t.Fatalf("not isolated: tools=%d messages=%d", len(req.Tools), len(req.Messages))
	}
	if strings.Contains(req.Messages[0].Content, "An original sentence.") {
		t.Fatal("Document text entered trusted system instructions")
	}
	client.fail = true
	got = call(`{"action":"correct","text":"Text","source_revision":1}`)
	if got.Code != 502 {
		t.Fatalf("LLM failure: %d", got.Code)
	}
}
