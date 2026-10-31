package notebooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
)

type unavailableImageRuntime struct {
	Runtime
	creates atomic.Int32
}

func (*unavailableImageRuntime) Cleanup(context.Context) error { return nil }
func (*unavailableImageRuntime) EnsureImage(context.Context) error {
	return errors.New("fixture image verification failed")
}
func (r *unavailableImageRuntime) Create(context.Context, string, config.KaggleLocalConfig, bool) (string, error) {
	r.creates.Add(1)
	return "", errors.New("container must not be created")
}

func TestNotebookImageVerificationFailurePreventsContainerStart(t *testing.T) {
	s, d, p, _, _ := documentFixture(t)
	if _, err := d.Save(t.Context(), p.ID, "book.ipynb", "*", []byte(preservedNotebook)); err != nil {
		t.Fatal(err)
	}
	runtime := &unavailableImageRuntime{}
	k := NewKernels(t.Context(), s, d, runtime)
	defer k.Close()
	if _, err := k.Start(p.ID, "book.ipynb", nil, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		session := k.List()[0]
		if session.Status == "stopped" {
			if runtime.creates.Load() != 0 || !strings.Contains(session.Error, "image verification failed") {
				t.Fatal("image failure did not prevent container creation", session)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("failed session did not stop")
}

func TestNotebookDockerUsesPinnedImageWithoutFallback(t *testing.T) {
	if !regexp.MustCompile(`^ghcr\.io/antibyte/aurago-kaggle-notebook@sha256:[0-9a-f]{64}$`).MatchString(RuntimeImage) {
		t.Fatal("runtime must use the published OCI index digest")
	}
	for _, mode := range []string{"cached", "pull", "inspect-failure", "pull-failure", "stream-failure", "missing-after-pull"} {
		t.Run(mode, func(t *testing.T) {
			var present atomic.Bool
			present.Store(mode == "cached")
			var creates, pulls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.Path, "/v1.45")
				switch {
				case path == "/version":
					fmt.Fprint(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
				case strings.HasPrefix(path, "/images/") && strings.HasSuffix(path, "/json"):
					if path != "/images/"+RuntimeImage+"/json" {
						t.Error("inspection used a different image", path)
					}
					if mode == "inspect-failure" {
						w.WriteHeader(500)
					} else if !present.Load() {
						w.WriteHeader(404)
					} else {
						fmt.Fprint(w, `{}`)
					}
				case path == "/images/create":
					pulls.Add(1)
					if r.URL.Query().Get("fromImage") != RuntimeImage {
						t.Error("pull used a different image")
					}
					switch mode {
					case "pull-failure":
						w.WriteHeader(404)
					case "stream-failure":
						fmt.Fprintln(w, `{"error":"fixture digest unavailable"}`)
					default:
						present.Store(mode != "missing-after-pull")
						fmt.Fprintln(w, `{"status":"complete"}`)
					}
				case path == "/containers/create":
					creates.Add(1)
					var body struct{ Image string }
					if json.NewDecoder(r.Body).Decode(&body) != nil || body.Image != RuntimeImage {
						t.Error("container used a different image")
					}
					w.WriteHeader(201)
					fmt.Fprint(w, `{"Id":"fixture"}`)
				case path == "/containers/fixture/start":
					w.WriteHeader(204)
				default:
					t.Error("unexpected Docker call", r.Method, path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			d := NewDocker("tcp://"+strings.TrimPrefix(server.URL, "http://"), t.TempDir())
			err := d.EnsureImage(context.Background())
			wantOK := mode == "cached" || mode == "pull"
			if (err == nil) != wantOK {
				t.Fatalf("image readiness: %v", err)
			}
			if err == nil {
				s, _, _, _, _ := documentFixture(t)
				if _, err = d.Create(context.Background(), "fixture", s.Snapshot().Config.Local, false); err != nil {
					t.Fatal(err)
				}
			}
			if (creates.Load() == 1) != wantOK {
				t.Fatal("incorrect container admission")
			}
			if (mode == "cached" || mode == "inspect-failure") && pulls.Load() != 0 {
				t.Fatal("unexpected image pull")
			}
			if pulls.Load() > 1 {
				t.Fatal("failed image pull retried with a fallback")
			}
		})
	}
}
