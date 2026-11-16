package storeapps

import (
	"archive/zip"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPDFArchiveRejectsExpandedBudgetAndTraversal(t *testing.T) {
	for _, name := range []string{"page.pdf", "../page.pdf"} {
		var data bytes.Buffer
		w := zip.NewWriter(&data)
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte("%PDF-1.7 test")); err != nil {
			t.Fatal(err)
		}
		if err = w.Close(); err != nil {
			t.Fatal(err)
		}
		if err = validatePDFArchive(data.Bytes(), 2); err == nil {
			t.Fatal("accepted oversized or escaping archive")
		}
		if name == "page.pdf" && validatePDFArchive(data.Bytes(), 100) != nil {
			t.Fatal("rejected bounded PDF archive")
		}
		if name != "page.pdf" && validatePDFArchive(data.Bytes(), 100) == nil {
			t.Fatal("accepted traversal")
		}
	}
}

func TestBeszelUsesNormalUserAndMarksStaleMetrics(t *testing.T) {
	superuser := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/collections/users/auth-with-password" {
			if r.Method != "POST" || r.Header.Get("Authorization") != "" {
				t.Error("password used as bearer")
			}
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["identity"] != "reader@example.test" || body["password"] != "beszel-password" {
				t.Error("wrong login")
			}
			collection := "users"
			if superuser {
				collection = "_superusers"
			}
			fmt.Fprintf(w, `{"token":"beszel-test-jwt","record":{"collectionName":%q}}`, collection)
			return
		}
		if r.Method != "GET" || r.Header.Get("Authorization") != "beszel-test-jwt" {
			t.Error("unexpected privileged request")
		}
		switch r.URL.Path {
		case "/api/collections/systems/records":
			fmt.Fprint(w, `{"totalItems":1,"items":[{"id":"host1","name":"NAS","status":"up","updated":"2020-01-01 00:00:00.000Z"}]}`)
		case "/api/collections/system_stats/records":
			if r.URL.Query().Get("filter") != "system='host1' && type='1m'" {
				t.Error("unscoped metrics")
			}
			fmt.Fprint(w, `{"items":[{"created":"2020-01-01 00:00:00.000Z","stats":{"cpu":12,"mp":50}}]}`)
		case "/api/collections/container_stats/records":
			fmt.Fprint(w, `{"items":[{"created":"2020-01-01 00:00:00.000Z","stats":[{"n":"app","c":3,"m":128}]}]}`)
		default:
			t.Error("unexpected endpoint")
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	c := Connection{URL: upstream.URL, Username: "reader@example.test", Credential: "beszel-password"}
	if _, err := c.BeszelSystems(t.Context(), 0, 25); err != nil {
		t.Fatal(err)
	}
	for _, containers := range []bool{false, true} {
		got, err := c.BeszelMetrics(t.Context(), "host1", containers)
		if err != nil || !got.(map[string]any)["stale"].(bool) {
			t.Fatalf("metrics: %v %v", got, err)
		}
	}
	if _, err := c.BeszelMetrics(t.Context(), "host1' || true", false); err == nil {
		t.Fatal("filter injection")
	}
	superuser = true
	if _, err := c.BeszelSystems(t.Context(), 0, 25); err == nil {
		t.Fatal("superuser accepted")
	}
}

func TestFileBrowserRequiresOneReadOnlySourceAndBoundsDownloads(t *testing.T) {
	writable := false
	downloads := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer filebrowser-token" {
			t.Error("invalid read")
		}
		switch r.URL.Path {
		case "/api/users":
			fmt.Fprintf(w, `{"username":"reader","permissions":{"api":true},"scopes":[{"name":"Store files","permissions":{"view":true,"download":true,"modify":%t}}]}`, writable)
		case "/api/resources":
			if r.URL.Query().Get("source") != "Store files" {
				t.Error("wrong source")
			}
			if r.URL.Query().Get("path") == "/" {
				fmt.Fprint(w, `{"type":"directory","files":[{"name":"a.txt","size":4,"type":"text/plain","viewToken":"must-not-leak"}]}`)
			} else {
				fmt.Fprint(w, `{"name":"a.txt","size":4,"type":"text/plain"}`)
			}
		case "/api/tools/search":
			if r.URL.Query().Get("scope") != "Store files:/" || r.URL.Query().Get("terms") != "a" {
				t.Error("unscoped search")
			}
			fmt.Fprint(w, `[{"source":"Store files","name":"a.txt","path":"/a.txt","type":"text/plain"}]`)
		case "/api/resources/download":
			downloads++
			fmt.Fprint(w, "longer than the announced file")
		default:
			t.Error("unexpected endpoint")
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	c := Connection{URL: upstream.URL, Credential: "filebrowser-token", Username: "reader", Source: "Store files"}
	got, err := c.FileBrowserList(t.Context(), "/", 0, 25)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "must-not-leak") {
		t.Fatal("view token leaked")
	}
	if _, err = c.FileBrowserSearch(t.Context(), "a", 25); err != nil {
		t.Fatal(err)
	}
	if _, _, err = c.FileBrowserRead(t.Context(), "/a.txt", 8); err == nil {
		t.Fatal("oversize download")
	}
	if _, _, err = c.FileBrowserRead(t.Context(), "../outside", 100); err == nil {
		t.Fatal("traversal")
	}
	writable = true
	if _, _, err = c.FileBrowserRead(t.Context(), "/a.txt", 100); err == nil || downloads != 1 {
		t.Fatal("writable user accepted")
	}
}

func testServerCertificate(t *testing.T, serial int64) ([]byte, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "Store signing fixture"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(der)
	return der, hex.EncodeToString(sum[:])
}

func TestStirlingPinsServerCertificateAndUsesFixedMultipart(t *testing.T) {
	cert, fingerprint := testServerCertificate(t, 1)
	changed, _ := testServerCertificate(t, 2)
	current := cert
	changeDuringSign := false
	posts := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != "stirling-key" {
			t.Error("missing API key")
		}
		switch r.URL.Path {
		case "/v1/api-docs":
			fmt.Fprint(w, `{"paths":{"/api/v1/misc/ocr-pdf":{"post":{}},"/api/v1/general/split-pages":{"post":{}},"/api/v1/general/merge-pdfs":{"post":{}},"/api/v1/security/cert-sign":{"post":{}}}}`)
		case "/api/v1/ui-data/ocr-pdf":
			fmt.Fprint(w, `{"languages":["deu","eng"]}`)
		case "/api/v1/admin/server-certificate/certificate":
			w.Write(current)
		default:
			posts++
			if r.Method != "POST" {
				t.Error("unexpected request")
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
				return
			}
			defer r.MultipartForm.RemoveAll()
			if len(r.MultipartForm.File["fileInput"]) == 0 {
				t.Error("no PDF")
			}
			for key := range r.MultipartForm.File {
				if key != "fileInput" {
					t.Error("uploaded key material")
				}
			}
			if r.URL.Path == pdfOperations["cert_sign"] {
				if r.FormValue("certType") != "SERVER" || r.FormValue("password") != "" {
					t.Error("not server signing")
				}
				if changeDuringSign {
					current = changed
				}
			}
			if r.URL.Path == pdfOperations["ocr"] && r.FormValue("ocrType") != "skip-text" {
				t.Error("unexpected OCR mode")
			}
			fmt.Fprint(w, "%PDF-1.7\nfixture result")
		}
	}))
	defer upstream.Close()
	c := Connection{URL: upstream.URL, Credential: "stirling-key", CertificateFingerprint: fingerprint}
	if err := c.StirlingProbe(t.Context(), true, true); err != nil || posts != 0 {
		t.Fatalf("probe: %v", err)
	}
	pdf := PDFInput{Data: []byte("%PDF-1.7\ninput")}
	for _, op := range []string{"ocr", "split", "merge", "cert_sign"} {
		inputs := []PDFInput{pdf}
		if op == "merge" {
			inputs = append(inputs, pdf)
		}
		out, ext, err := c.StirlingProcess(t.Context(), PDFRequest{Operation: op, Languages: []string{"deu"}, Pages: "all"}, inputs, 1024)
		if err != nil || ext != ".pdf" || !bytes.HasPrefix(out, []byte("%PDF-")) {
			t.Fatalf("%s: %v", op, err)
		}
	}
	before := posts
	if _, _, err := c.StirlingProcess(t.Context(), PDFRequest{Operation: "ocr", Languages: []string{"missing"}}, []PDFInput{pdf}, 1024); err == nil || posts != before {
		t.Fatal("missing language processed")
	}
	current = changed
	if _, _, err := c.StirlingProcess(t.Context(), PDFRequest{Operation: "cert_sign"}, []PDFInput{pdf}, 1024); err == nil || posts != before {
		t.Fatal("changed certificate signed")
	}
	current = cert
	changeDuringSign = true
	if out, _, err := c.StirlingProcess(t.Context(), PDFRequest{Operation: "cert_sign"}, []PDFInput{pdf}, 1024); err == nil || out != nil {
		t.Fatal("changed certificate result published")
	}
}
