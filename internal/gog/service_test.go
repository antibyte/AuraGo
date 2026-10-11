package gog

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

func fixtureService(t *testing.T) (*Service, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	vault, e := security.NewVault(strings.Repeat("51", 32), filepath.Join(dir, "vault"))
	if e != nil {
		t.Fatal(e)
	}
	cfg := &config.Config{}
	cfg.Gog.Enabled = true
	cfg.Agent.AllowShell = true
	cfg.Agent.AllowNetworkRequests = true
	cfg.VirtualDesktop.Enabled = true
	cfg.Gog.Accounts = []config.GogAccount{{ID: "primary", Enabled: true, AgentAccess: true, DesktopAccess: true, ClientID: "fixture-client", Services: map[string]config.GogGrants{"gmail": {Read: true, Send: true, Update: true}, "drive": {Read: true, Create: true}}}}
	s := &Service{Root: filepath.Join(dir, "gog"), Vault: vault, Config: func() *config.Config { return cfg }, RunProcess: func(context.Context, string, []string, string, []string) (string, int, bool, error) {
		return `{"items":[]}`, 0, true, nil
	}}
	tok := Token{AccessToken: "fixture-gog-access", ClientID: "fixture-client", Expiry: time.Now().Add(time.Hour), Scopes: []string{"https://www.googleapis.com/auth/gmail.modify", "https://www.googleapis.com/auth/drive"}}
	raw, _ := json.Marshal(tok)
	if e = vault.WriteSecret(TokenKey("primary"), string(raw)); e != nil {
		t.Fatal(e)
	}
	data := []byte("fixture managed executable")
	sum := sha256.Sum256(data)
	a := Artifact{OS: runtime.GOOS, Arch: runtime.GOARCH, BinarySHA256: hex.EncodeToString(sum[:]), ArchiveSHA256: strings.Repeat("0", 64)}
	previous := artifactManifest
	artifactManifest, _ = json.Marshal([]Artifact{a})
	t.Cleanup(func() { artifactManifest = previous })
	if e = os.MkdirAll(filepath.Dir(s.binaryPath(a)), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(s.binaryPath(a), data, 0700); e != nil {
		t.Fatal(e)
	}
	return s, cfg
}

func TestGogSingleAttemptAndUnknownWrite(t *testing.T) {
	for _, code := range []int{1, 7, 8, 130} {
		t.Run(string(rune('a'+code)), func(t *testing.T) {
			s, _ := fixtureService(t)
			calls := 0
			s.RunProcess = func(context.Context, string, []string, string, []string) (string, int, bool, error) {
				calls++
				return `{"error":{"code":503}}`, code, true, nil
			}
			r := s.Execute(context.Background(), "agent", Request{AccountID: "primary", Service: "gmail", Operation: "send", Args: map[string]any{"to": "fixture@example.invalid", "subject": "fixture", "body": "fixture"}}, nil)
			if r.Status != "external_status_unknown" || calls != 1 {
				t.Fatalf("status=%s calls=%d", r.Status, calls)
			}
		})
	}
}

func TestGogDeniedBeforeProcess(t *testing.T) {
	for _, kind := range []string{"disabled", "readonly", "scope", "account", "source"} {
		t.Run(kind, func(t *testing.T) {
			s, c := fixtureService(t)
			calls := 0
			s.RunProcess = func(context.Context, string, []string, string, []string) (string, int, bool, error) {
				calls++
				return `{}`, 0, true, nil
			}
			req := Request{AccountID: "primary", Service: "gmail", Operation: "archive", Args: map[string]any{"message_id": "123"}}
			switch kind {
			case "disabled":
				c.Gog.Enabled = false
			case "readonly":
				c.Gog.ReadOnly = true
			case "scope":
				c.Gog.Accounts[0].Services["gmail"] = config.GogGrants{}
			case "account":
				req.AccountID = "other"
			case "source":
				c.Gog.Accounts[0].AgentAccess = false
			}
			r := s.Execute(context.Background(), "agent", req, nil)
			if r.Status != "error" || calls != 0 {
				t.Fatalf("denial failed: %+v, calls=%d", r, calls)
			}
		})
	}
}

func TestGogRevocationCancelsRunningCall(t *testing.T) {
	s, c := fixtureService(t)
	var current atomic.Pointer[config.Config]
	current.Store(c)
	s.Config = current.Load
	started := make(chan struct{})
	s.RunProcess = func(ctx context.Context, _ string, _ []string, _ string, _ []string) (string, int, bool, error) {
		close(started)
		<-ctx.Done()
		return "", 130, true, ctx.Err()
	}
	done := make(chan Result, 1)
	go func() {
		done <- s.Execute(context.Background(), "agent", Request{AccountID: "primary", Service: "gmail", Operation: "labels"}, nil)
	}()
	select {
	case <-started:
	case r := <-done:
		t.Fatalf("call never started: %+v", r)
	case <-time.After(3 * time.Second):
		t.Fatal("call did not start")
	}
	next := *c
	next.Gog.Enabled = false
	current.Store(&next)
	select {
	case r := <-done:
		if r.Status == "ok" {
			t.Fatal("revoked call succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("revocation did not cancel")
	}
}

func TestGogDisconnectWinsOverStaleRefresh(t *testing.T) {
	s, _ := fixtureService(t)
	_, before, e := s.token("primary")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Disconnect("primary"); e != nil {
		t.Fatal(e)
	}
	if e = s.Vault.CompareAndSwapSecret(TokenKey("primary"), before, before); e == nil {
		t.Fatal("stale refresh resurrected account")
	}
	if s.Connected("primary") {
		t.Fatal("still connected")
	}
}

func TestGogArgumentsRejectEscapeHatches(t *testing.T) {
	for _, bad := range []map[string]any{{"access_token": "x"}, {"max": 101.0}, {"max": -1.0}, {"page": "@private"}, {"page": "x\x00y"}} {
		op, _ := Lookup("drive", "list")
		if _, e := Arguments(Request{Service: "drive", Operation: "list", Args: bad}, op, t.TempDir(), nil); e == nil {
			t.Fatalf("accepted %#v", bad)
		}
	}
	op, _ := Lookup("drive", "search")
	args, e := Arguments(Request{Service: "drive", Operation: "search", Args: map[string]any{"query": "--access-token=bad"}}, op, t.TempDir(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if args[len(args)-2] != "--" {
		t.Fatal("positional flags not isolated")
	}
	op, _ = Lookup("calendar", "update")
	if _, e = Arguments(Request{Service: "calendar", Args: map[string]any{"calendar_id": "primary", "event_id": "x"}}, op, t.TempDir(), nil); e == nil {
		t.Fatal("implicit recurrence scope")
	}
}

func TestGogInstallerRejectsArchiveAndKeepsBinary(t *testing.T) {
	s, _ := fixtureService(t)
	a, _ := CurrentArtifact()
	path := s.binaryPath(a)
	before, _ := os.ReadFile(path)
	for _, name := range []string{"../gog", executableName()} {
		var buf bytes.Buffer
		z := zip.NewWriter(&buf)
		w, _ := z.Create(name)
		w.Write([]byte("not approved"))
		z.Close()
		h := sha256.Sum256(buf.Bytes())
		a.ArchiveSHA256 = hex.EncodeToString(h[:])
		if e := s.installArchive(context.Background(), a, bytes.NewReader(buf.Bytes())); e == nil {
			t.Fatal("accepted unsafe package")
		}
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("previous executable changed")
	}
}

func TestGogInvalidOutputAndQuota(t *testing.T) {
	for _, test := range []struct {
		out  string
		code int
		want string
	}{{"bad", 0, "invalid_response"}, {`{} {}`, 0, "invalid_response"}, {`{"error":{}}`, 7, "rate_limited"}} {
		s, _ := fixtureService(t)
		s.RunProcess = func(context.Context, string, []string, string, []string) (string, int, bool, error) {
			return test.out, test.code, true, nil
		}
		r := s.Execute(context.Background(), "agent", Request{AccountID: "primary", Service: "gmail", Operation: "labels"}, nil)
		if r.Code != test.want {
			t.Fatalf("got %s want %s", r.Code, test.want)
		}
	}
}
