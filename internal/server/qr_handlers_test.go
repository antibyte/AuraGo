package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/qrcode"
	"aurago/internal/tools"
)

func qrHTTPFixture(t *testing.T) (*Server, func(string, string, []byte) *httptest.ResponseRecorder) {
	t.Helper()
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	s := &Server{Cfg: cfg}
	return s, func(method, path string, body []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost"+path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handleQR(s).ServeHTTP(w, r)
		return w
	}
}

func TestQRCodeHTTPAndNativeShareAdmission(t *testing.T) {
	s, call := qrHTTPFixture(t)
	for i := 0; i < 2; i++ {
		release, ok := tools.AcquireQRWork()
		if !ok {
			t.Fatal("cannot occupy QR admission")
		}
		defer release()
	}
	if w := call("POST", "/api/qr/render", []byte(`{}`)); w.Code != 429 || !strings.Contains(w.Body.String(), "qr_busy") {
		t.Fatalf("busy render = %d %s", w.Code, w.Body)
	}
	if w := call("GET", "/api/qr/presets", nil); w.Code != 200 {
		t.Fatal("cheap catalog was blocked by render admission")
	}
	s.Cfg.Tools.QRCode.Enabled = true
	result := tools.ExecuteQRCode(context.Background(), s.Cfg, map[string]interface{}{"operation": "scan"})
	if !strings.Contains(result.Output, "QR processing is busy") {
		t.Fatal("native tool bypassed shared admission")
	}
}

func TestQRCodeHTTPRenderScanAndConfig(t *testing.T) {
	s, call := qrHTTPFixture(t)
	if s.Cfg.Tools.QRCode.Enabled {
		t.Fatal("fixture must prove human/agent independence")
	}
	spec := qrcode.QRSpec{SchemaVersion: 1, Content: qrcode.Content{Type: "text", Text: "Grüße HTTP QR"}, Output: qrcode.Output{Format: "png"}}
	body, _ := json.Marshal(spec)
	render := call("POST", "/api/qr/render", body)
	if render.Code != 200 {
		t.Fatalf("render %d: %s", render.Code, render.Body)
	}
	if render.Header().Get("Cache-Control") != "no-store" || render.Header().Get("X-QR-Verified") != "true" {
		t.Fatal("private verified response headers missing")
	}
	var metadata qrcode.Metadata
	if json.Unmarshal([]byte(render.Header().Get("X-QR-Metadata")), &metadata) != nil || metadata.Width < 100 {
		t.Fatal("invalid render metadata")
	}
	decoded, err := qrcode.Decode(context.Background(), render.Body.Bytes())
	if err != nil || decoded.Text != spec.Content.Text {
		t.Fatalf("decode %v %+v", err, decoded)
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	scan := call("POST", "/api/qr/scan", render.Body.Bytes())
	if scan.Code != 200 || !strings.Contains(scan.Body.String(), spec.Content.Text) {
		t.Fatalf("scan %d: %s", scan.Code, scan.Body)
	}
	if got := call("POST", "/api/qr/render", body); got.Code != 403 {
		t.Fatal("readonly render permitted")
	}
	exported := call("POST", "/api/qr/config", body)
	if exported.Code != 200 {
		t.Fatalf("export %d: %s", exported.Code, exported.Body)
	}
	if _, err := qrcode.ParseConfig(context.Background(), exported.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
}

func TestQRCodeHTTPConfigurationRetainsStrictRawValidation(t *testing.T) {
	_, call := qrHTTPFixture(t)
	for _, raw := range [][]byte{
		[]byte(`{"content":{"type":"text","type":"url","text":"keep"}}`),
		[]byte(`{"content":{"type":"text","text":"first","text":"second"}}`),
		append([]byte(`{"content":{"type":"text","text":"`), append([]byte{0xff}, []byte(`"}}`)...)...),
	} {
		if w := call("POST", "/api/qr/config", raw); w.Code != 400 {
			t.Fatalf("ambiguous/invalid UTF-8 config accepted: %d", w.Code)
		}
	}
}

func TestQRCodeHTTPAuthorizationAndInputBoundaries(t *testing.T) {
	s, call := qrHTTPFixture(t)
	s.Cfg.Auth.Enabled = true
	if w := call("GET", "/api/qr/presets", nil); w.Code != 401 {
		t.Fatal("unauthenticated catalog exposed")
	}
	s.Cfg.Auth.Enabled = false
	r := httptest.NewRequest("GET", "http://localhost/api/qr/presets", nil)
	r.Header.Set("Authorization", "Bearer invalid")
	w := httptest.NewRecorder()
	handleQR(s).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("invalid bearer accepted")
	}
	if !isDesktopScopedAPIPath("/api/qr/render") {
		t.Fatal("QR not covered by scoped bearer middleware")
	}
	if w := call("GET", "/api/qr/render", nil); w.Code != 405 {
		t.Fatal("wrong method accepted")
	}
	for _, body := range []string{`{"unknown":true}`, `{"content":{"type":"text","text":"x"},"style":{"size":99999}}`, `{"content":{"type":"text","text":"x"}} {"extra":1}`} {
		if w := call("POST", "/api/qr/render", []byte(body)); w.Code < 400 {
			t.Fatalf("invalid input accepted: %s", body)
		}
	}
	if w := call("POST", "/api/qr/import-image", []byte(`{"url":"http://127.0.0.1/private"}`)); w.Code != 403 {
		t.Fatal("network grant ignored")
	}
	s.Cfg.Agent.AllowNetworkRequests = true
	if w := call("POST", "/api/qr/import-image", []byte(`{"url":"http://127.0.0.1/private"}`)); w.Code != 422 {
		t.Fatal("private URL permitted")
	}
	s.Cfg.VirtualDesktop.Enabled = false
	if w := call("GET", "/api/qr/presets", nil); w.Code != 403 {
		t.Fatal("disabled Desktop bypassed")
	}
}

func TestQRCodeHTTPBatchRejectsPartialArchive(t *testing.T) {
	_, call := qrHTTPFixture(t)
	request := map[string]interface{}{"spec": qrcode.QRSpec{Content: qrcode.Content{Type: "text"}, Output: qrcode.Output{Format: "png"}}, "csv": "text,filename\nhello,one\n,missing\n", "content_type": "text", "dry_run": true}
	body, _ := json.Marshal(request)
	dry := call("POST", "/api/qr/batch", body)
	var result qrcode.BatchResult
	if dry.Code != 200 || json.Unmarshal(dry.Body.Bytes(), &result) != nil || len(result.Errors) == 0 {
		t.Fatalf("dry batch %d: %s", dry.Code, dry.Body)
	}
	request["dry_run"] = false
	body, _ = json.Marshal(request)
	final := call("POST", "/api/qr/batch", body)
	if final.Code < 400 || final.Header().Get("Content-Type") == "application/zip" {
		t.Fatalf("invalid batch published: %d", final.Code)
	}
}
