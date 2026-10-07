package deployer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/dockerutil"
)

var speechLabTestImage = "ghcr.io/antibyte/s2s-gateway@sha256:" + strings.Repeat("a", 64)

func speechLabPullOperation(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) operationSnapshot {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
			return
		}
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/images/create") {
			t.Errorf("unexpected Docker request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return operationSnapshot{docker: dockerutil.NewClient("tcp://"+strings.TrimPrefix(server.URL, "http://"), time.Second)}
}

func TestPullFailsOnErrorAfterLongProgressStream(t *testing.T) {
	op := speechLabPullOperation(t, func(w http.ResponseWriter, r *http.Request) {
		line := `{"status":"Downloading","progressDetail":{"current":1,"total":2},"id":"0123456789ab"}` + "\n"
		chunk := strings.Repeat(line, (1<<20)/len(line)+1)
		for written := 0; written < 9<<20; written += len(chunk) {
			_, _ = io.WriteString(w, chunk)
		}
		_, _ = io.WriteString(w, `{"errorDetail":{"message":"registry denied"},"error":"registry denied"}`+"\n")
	})
	err := (&Manager{}).pull(context.Background(), op, speechLabTestImage)
	if err == nil || err.Error() != "speech_lab_pull_failed: Docker pull failed: registry denied" {
		t.Fatalf("pull() = %v, want the error event after 9 MiB of progress", err)
	}
}

func TestPullRejectsStreamCutInsideMessage(t *testing.T) {
	op := speechLabPullOperation(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"status":"Downloading"}`+"\n"+`{"status":"Downlo`)
	})
	err := (&Manager{}).pull(context.Background(), op, speechLabTestImage)
	if Code(err) != "speech_lab_pull_failed" || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("pull() = %v, want speech_lab_pull_failed wrapping io.ErrUnexpectedEOF", err)
	}
}

func TestPullErrorDetailIsOneBoundedLine(t *testing.T) {
	message := "denied\n\u001b[31m" + strings.Repeat("x", 300) + "é"
	event, err := json.Marshal(map[string]string{"error": message})
	if err != nil {
		t.Fatal(err)
	}
	op := speechLabPullOperation(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(append(event, '\n'))
	})
	pullErr := (&Manager{}).pull(context.Background(), op, speechLabTestImage)
	want := "speech_lab_pull_failed: Docker pull failed: denied  [31m" + strings.Repeat("x", 244)
	if pullErr == nil || pullErr.Error() != want || !utf8.ValidString(pullErr.Error()) {
		t.Fatalf("pull() = %q, want %q", pullErr, want)
	}
}

func TestPullNon2xxNamesDockerReason(t *testing.T) {
	op := speechLabPullOperation(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"manifest unknown"}`)
	})
	err := (&Manager{}).pull(context.Background(), op, speechLabTestImage)
	if err == nil || err.Error() != "speech_lab_pull_failed: Docker pull returned HTTP 404: manifest unknown" {
		t.Fatalf("pull() = %v", err)
	}
}

func TestPullCompleteStreamSucceeds(t *testing.T) {
	op := speechLabPullOperation(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"status":"Status: Downloaded newer image"}`+"\n")
	})
	if err := (&Manager{}).pull(context.Background(), op, speechLabTestImage); err != nil {
		t.Fatalf("pull() = %v, want nil", err)
	}
}
