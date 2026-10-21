package decisions

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDockerCloseConfirmsRemovalAndRetainsFailedSession(t *testing.T) {
	for _, failRemove := range []bool{false, true} {
		docker := newDecisionsDocker("")
		removes, cleanups := 0, 0
		docker.client.HTTPClient().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			status := http.StatusInternalServerError
			switch {
			case strings.HasSuffix(r.URL.Path, "/stop"): // A successful forced removal supersedes this failure.
			case r.Method == http.MethodDelete:
				removes++
				if !failRemove || removes > 1 {
					status = http.StatusNoContent
				}
			case strings.HasSuffix(r.URL.Path, "/json"):
				status = http.StatusNotFound
			default:
				t.Fatalf("unexpected request: %s", r.URL)
			}
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
		})
		session := &dockerRuntimeSession{nativeRuntimeSession: &nativeRuntimeSession{}, docker: docker, containerID: "owned", cleanup: func(_ context.Context) { cleanups++ }}
		err := session.Close(t.Context())
		if failRemove {
			if err == nil || session.containerID == "" || cleanups != 0 {
				t.Fatal("failed removal lost ownership")
			}
			if err := session.Close(t.Context()); err != nil {
				t.Fatal(err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if session.containerID != "" || cleanups != 1 {
			t.Fatal("confirmed removal did not settle cleanup exactly once")
		}
	}
}
