package tools

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func decodeCertificateManagerResult(t *testing.T, raw string) CertificateManagerResult {
	t.Helper()
	var r CertificateManagerResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("failed to decode result: %v - raw: %s", err, raw)
	}
	return r
}

func TestCertificateManagerGenerateSelfSignedAndInfo(t *testing.T) {
	workspace := t.TempDir()
	raw := ExecuteCertificateManager("generate_self_signed", "", "", 0, "localhost", "certs", 7, workspace)
	res := decodeCertificateManagerResult(t, raw)
	if res.Status != "success" {
		t.Fatalf("generate_self_signed status = %s: %s", res.Status, res.Message)
	}

	certPath := filepath.Join(workspace, "certs", "cert.pem")
	keyPath := filepath.Join(workspace, "certs", "key.pem")
	if _, err := os.Stat(certPath); err != nil {
		t.Fatalf("cert.pem missing: %v", err)
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("key.pem missing: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("key.pem mode = %v, want 0600", info.Mode().Perm())
	}

	raw = ExecuteCertificateManager("info", "certs/cert.pem", "", 0, "", "", 0, workspace)
	res = decodeCertificateManagerResult(t, raw)
	if res.Status != "success" {
		t.Fatalf("info status = %s: %s", res.Status, res.Message)
	}
	data, ok := res.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("info data = %#v", res.Data)
	}
	if data["dns_names"] == nil {
		t.Fatalf("info data missing dns_names: %#v", data)
	}
}

func TestCertificateManagerCheckRemoteUsesPeerCertificate(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	host, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("split hostport: %v", err)
	}
	portNum, err := net.LookupPort("tcp", port)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	raw := ExecuteCertificateManager("check_remote", "", host, portNum, "", "", 0, t.TempDir())
	res := decodeCertificateManagerResult(t, raw)
	if res.Status != "success" {
		t.Fatalf("check_remote status = %s: %s", res.Status, res.Message)
	}
	data, ok := res.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("remote data = %#v", res.Data)
	}
	if data["not_after"] == "" {
		t.Fatalf("remote data missing not_after: %#v", data)
	}
}
