package gog

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGogInstallerPublishesOnlyApprovedCompletePackage(t *testing.T) {
	s, _ := fixtureService(t)
	a, _ := CurrentArtifact()
	before, _ := os.ReadFile(s.binaryPath(a))
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, _ := z.Create(executableName())
	w.Write(before)
	z.Close()
	h := sha256.Sum256(b.Bytes())
	a.ArchiveSHA256 = hex.EncodeToString(h[:])
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := s.installArchive(ctx, a, bytes.NewReader(b.Bytes())); !errors.Is(e, context.Canceled) {
		t.Fatalf("cancel: %v", e)
	}
	if e := s.installArchive(context.Background(), a, strings.NewReader("broken archive")); e == nil {
		t.Fatal("wrong archive checksum accepted")
	}
	h = sha256.Sum256([]byte("broken archive"))
	broken := a
	broken.ArchiveSHA256 = hex.EncodeToString(h[:])
	if e := s.installArchive(context.Background(), broken, strings.NewReader("broken archive")); e == nil {
		t.Fatal("corrupt archive accepted")
	}
	after, _ := os.ReadFile(s.binaryPath(a))
	if !bytes.Equal(before, after) {
		t.Fatal("failed installation changed installed binary")
	}
	if e := s.installArchive(context.Background(), a, bytes.NewReader(b.Bytes())); e != nil {
		t.Fatal(e)
	}
	if _, e := s.VerifiedBinary(); e != nil {
		t.Fatal(e)
	}
}

func TestGogManifestHasExactlySupportedPlatforms(t *testing.T) {
	var all []Artifact
	if e := json.Unmarshal(artifactManifest, &all); e != nil {
		t.Fatal(e)
	}
	seen := map[string]bool{}
	for _, a := range all {
		key := a.OS + "/" + a.Arch
		if seen[key] {
			t.Fatal("duplicate platform")
		}
		seen[key] = true
		for _, hash := range []string{a.ArchiveSHA256, a.BinarySHA256} {
			if raw, e := hex.DecodeString(hash); e != nil || len(raw) != 32 {
				t.Fatal("invalid checksum")
			}
		}
	}
	for _, key := range []string{"linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64", "darwin/amd64", "darwin/arm64"} {
		if !seen[key] {
			t.Fatal("missing " + key)
		}
	}
	if len(all) != 6 {
		t.Fatal("unexpected platform")
	}
	old := artifactManifest
	defer func() { artifactManifest = old }()
	artifactManifest = []byte(`[]`)
	if _, e := CurrentArtifact(); e == nil {
		t.Fatal("unsupported platform admitted")
	}
}

func TestGogRuntimeDenialPrecedesAuthentication(t *testing.T) {
	s, _ := fixtureService(t)
	s.CheckRuntime = func(context.Context) error { return errors.New("host execution disabled") }
	if e := s.Vault.DeleteSecret(TokenKey("primary")); e != nil {
		t.Fatal(e)
	}
	r := s.Execute(context.Background(), "test", Request{AccountID: "primary", Service: "gmail", Operation: "labels"}, nil)
	if r.Code != "execution_denied" {
		t.Fatalf("unexpected %+v", r)
	}
}

func TestGogOutputLimitsAndNullError(t *testing.T) {
	for _, tc := range []struct {
		out  string
		want string
	}{{strings.Repeat(" ", MaxOutput) + "{}", "error"}, {`{"error":null,"items":[]}`, "ok"}} {
		s, _ := fixtureService(t)
		s.RunProcess = func(context.Context, string, []string, string, []string) (string, int, bool, error) {
			return tc.out, 0, true, nil
		}
		if r := s.Execute(context.Background(), "agent", Request{AccountID: "primary", Service: "gmail", Operation: "labels"}, nil); r.Status != tc.want {
			t.Fatalf("got %s", r.Status)
		}
	}
}

func TestGogCalendarDatesAndUnsafeSlidesAssets(t *testing.T) {
	op, _ := Lookup("calendar", "create")
	for _, args := range []map[string]any{
		{"calendar_id": "primary", "summary": "Fixture", "from": "2026-10-11", "to": "2026-10-12", "all_day": true, "start_timezone": "Europe/Berlin"},
		{"calendar_id": "primary", "summary": "Fixture", "from": "2026-10-11T10:00:00+02:00", "to": "2026-10-11T11:00:00+02:00"},
	} {
		if _, e := Arguments(Request{Service: "calendar", Args: args}, op, t.TempDir(), nil); e != nil {
			t.Fatal(e)
		}
	}
	op, _ = Lookup("slides", "create")
	for _, md := range []string{"![image](https://example.invalid/x)", "<img src='file:///private'>", "```mermaid\na-->b\n```"} {
		if _, e := Arguments(Request{Service: "slides", Operation: "create", Args: map[string]any{"title": "Fixture", "content": md}}, op, t.TempDir(), nil); e == nil {
			t.Fatal("unsafe asset admitted")
		}
	}
}

func TestGogCuratedTrainingArgumentsReachRuntime(t *testing.T) {
	dir := os.Getenv("AURAGO_TRAINING_DIR")
	if dir == "" {
		t.Skip("separate training repository not configured")
	}
	data, err := os.ReadFile(filepath.Join(dir, "operation_contracts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contracts struct {
		Tools map[string]struct {
			Operations []struct {
				Value     string         `json:"value"`
				Arguments map[string]any `json:"arguments"`
			} `json:"operations"`
		} `json:"tools"`
	}
	if err = json.Unmarshal(data, &contracts); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, op := range Catalog() {
		found := false
		for _, fixture := range contracts.Tools["gog_"+op.Service].Operations {
			if fixture.Value != op.Operation {
				continue
			}
			found = true
			count++
			args := fixture.Arguments
			delete(args, "account_id")
			delete(args, "operation")
			delete(args, "input_path")
			delete(args, "output_path")
			files := &Files{Input: strings.NewReader("synthetic Office fixture"), Output: &bytes.Buffer{}, Name: "fixture.docx"}
			if _, err = Arguments(Request{Service: op.Service, Operation: op.Operation, Args: args}, op, t.TempDir(), files); err != nil {
				t.Errorf("%s.%s: %v", op.Service, op.Operation, err)
			}
		}
		if !found {
			t.Errorf("missing training operation %s.%s", op.Service, op.Operation)
		}
	}
	if count != 68 {
		t.Fatalf("operation coverage %d", count)
	}
}
