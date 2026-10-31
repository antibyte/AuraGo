package kaggle

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fixtureSecrets map[string]string

func (v fixtureSecrets) ReadSecret(k string) (string, error) {
	s, ok := v[k]
	if !ok {
		return "", errors.New("missing")
	}
	return s, nil
}
func (v fixtureSecrets) WriteSecret(k, vv string) error { v[k] = vv; return nil }

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestKaggleUploadVaultReceiptAndCredentialFreeDownload(t *testing.T) {
	s := newTestService(t)
	s.secrets = fixtureSecrets{}
	p, err := s.CreateProject(context.Background(), "Transfer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.WriteFile(context.Background(), p.ID, "source.csv", strings.NewReader("n\n42\n"), nil); err != nil {
		t.Fatal(err)
	}
	calls := 0
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing Kaggle auth")
		}
		if strings.Contains(r.URL.Path, "Upload") {
			io.WriteString(w, `{"token":"opaque-upload-secret","createUrl":"https://storage.invalid/object"}`)
		} else {
			w.Header().Set("Location", "https://storage.invalid/object")
			w.WriteHeader(302)
		}
	})
	c.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	s.storageFactory = func(ctx context.Context, url string, timeout time.Duration) (*http.Client, error) {
		return &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
				t.Fatal("Kaggle credentials reached storage")
			}
			if r.Method == "PUT" {
				b, _ := io.ReadAll(r.Body)
				if string(b) != "n\n42\n" {
					t.Fatalf("wrong upload %q", b)
				}
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("n\n42\n"))}, nil
		})}, nil
	}
	op, _ := Lookup("datasets", "upload_dataset_file")
	id := newID()
	check := func() error { return nil }
	data, err := s.upload(context.Background(), c, op, map[string]any{}, Request{ID: id, ProjectID: p.ID, File: "source.csv"}, "owner", check)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "opaque-upload-secret") {
		t.Fatal("upload secret in response")
	}
	var stored string
	s.Store.db.QueryRow(`SELECT token FROM uploads WHERE id=?`, id).Scan(&stored)
	if stored != "kaggle_upload_"+id {
		t.Fatal("database did not store a Vault reference")
	}
	params := map[string]any{"token": "upload:" + id}
	if err = s.resolveUploads(context.Background(), params, "other"); err == nil {
		t.Fatal("cross-account upload reference")
	}
	if err = s.resolveUploads(context.Background(), params, "owner"); err != nil || params["token"] != "opaque-upload-secret" {
		t.Fatal("opaque receipt did not resolve")
	}
	op, _ = Lookup("datasets", "download_dataset")
	if _, err = s.download(context.Background(), c, op, map[string]any{}, Request{ID: newID(), ProjectID: p.ID, File: "download.csv"}, check); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("unexpected replay: %d", calls)
	}
}

func TestKaggleActiveJobsSurviveLongHistory(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	old := newID()
	s.Store.admit(ctx, Job{ID: old, Account: "owner", Hash: "x", Request: json.RawMessage(`{}`)})
	s.Store.update(ctx, old, "remote_running", 0, nil, "")
	for i := 0; i < 205; i++ {
		id := newID()
		s.Store.admit(ctx, Job{ID: id, Account: "owner", Hash: "x", Request: json.RawMessage(`{}`)})
		s.Store.update(ctx, id, "completed", 0, nil, "")
	}
	jobs, err := s.Store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		if j.ID == old {
			return
		}
	}
	t.Fatal("active remote job disappeared behind history limit")
}

func TestKaggleQuotaAndOpaqueArchives(t *testing.T) {
	s := newTestService(t)
	snap := s.current()
	snap.Config.MaxTransferMB = 1
	s.current = func() Snapshot { return snap }
	p, err := s.CreateProject(context.Background(), "Quota")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.WriteFile(context.Background(), p.ID, "large.bin", strings.NewReader(strings.Repeat("x", (1<<20)+1)), nil); err == nil {
		t.Fatal("transfer limit ignored")
	}
	files, _ := s.Files(p.ID)
	if len(files) != 0 {
		t.Fatal("partial file published")
	}
	// Archive bytes are never extracted by transfers, regardless of internal paths.
	if _, err = s.WriteFile(context.Background(), p.ID, "malicious.zip", strings.NewReader("PK\x03\x04../../escape"), nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"CON", "nul.txt", "LPT1.csv", "COM9"} {
		if validFile(name) {
			t.Fatal("Windows device filename accepted")
		}
	}
}
