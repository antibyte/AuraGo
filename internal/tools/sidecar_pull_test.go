package tools

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const go2rtcTestImage = "alexxit/go2rtc:1.9.10"

func go2rtcImageDockerHost(t *testing.T, pullStatus int, pullBody string, pulls *atomic.Int32) string {
	t.Helper()
	return fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		switch {
		case r.Method == http.MethodGet && path == "/images/"+go2rtcTestImage+"/json":
			writeDockerJSON(w, http.StatusNotFound, map[string]string{"message": "No such image"})
		case r.Method == http.MethodPost && path == "/images/create":
			pulls.Add(1)
			w.WriteHeader(pullStatus)
			_, _ = io.WriteString(w, pullBody)
		default:
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestEnsureGo2RTCImageFailsOnPullStreamError(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	var pulls atomic.Int32
	host := go2rtcImageDockerHost(t, http.StatusOK, pullStreamFailure, &pulls)
	err := ensureGo2RTCImage(context.Background(), DockerConfig{Host: host}, go2rtcTestImage)
	if err == nil || err.Error() != "pull go2rtc image: failed to register layer: no space left on device" || pulls.Load() != 1 {
		t.Fatalf("ensureGo2RTCImage() = %v after %d pulls, want the stream error", err, pulls.Load())
	}
}

func TestEnsureGo2RTCImageKeepsStatusText(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	var pulls atomic.Int32
	host := go2rtcImageDockerHost(t, http.StatusNotFound, `{"message":"manifest unknown"}`, &pulls)
	err := ensureGo2RTCImage(context.Background(), DockerConfig{Host: host}, go2rtcTestImage)
	if err == nil || err.Error() != "pull go2rtc image returned HTTP 404: manifest unknown" {
		t.Fatalf("ensureGo2RTCImage() = %v", err)
	}
}

func TestEnsureGo2RTCImageSkipsPresentImage(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected Docker request for a present image: %s %s", r.Method, r.URL.String())
		}
		_, _ = io.WriteString(w, `{"Id":"sha256:go2rtc"}`)
	})
	if err := ensureGo2RTCImage(context.Background(), DockerConfig{Host: host}, go2rtcTestImage); err != nil {
		t.Fatalf("ensureGo2RTCImage() = %v, want nil", err)
	}
}

// slowPullHandler streams a successful pull that takes 200 ms.
func slowPullHandler(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `{"status":"Pulling fs layer"}`+"\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	time.Sleep(200 * time.Millisecond)
	_, _ = io.WriteString(w, pullStreamSuccess)
}

func TestEnsureGo2RTCImageOutlivesShortCallerDeadline(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		slowPullHandler(w)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := ensureGo2RTCImage(ctx, DockerConfig{Host: host}, go2rtcTestImage); err != nil {
		t.Fatalf("ensureGo2RTCImage() = %v, want the pull to finish as it did on the request client", err)
	}
}

const manifestTestName, manifestTestImage = "aurago-manifest-fp6", "manifestdotbuild/manifest:5"

func manifestTestPayload() ([]byte, error) {
	return []byte(`{"Image":"` + manifestTestImage + `"}`), nil
}

func TestEnsureManifestContainerNamesPullFailureWhenCreateFindsNoImage(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	var creates atomic.Int32
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		switch {
		case r.Method == http.MethodGet && (path == "/containers/"+manifestTestName+"/json" || path == "/images/"+manifestTestImage+"/json"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && path == "/images/create":
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, pullStreamFailure)
		case r.Method == http.MethodPost && path == "/containers/create":
			creates.Add(1)
			writeDockerJSON(w, http.StatusNotFound, map[string]string{"message": "No such image: " + manifestTestImage})
		default:
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
	err := ensureManifestContainer(context.Background(), DockerConfig{Host: host}, manifestTestName, manifestTestImage, manifestTestPayload)
	if err == nil || !strings.Contains(err.Error(), `create container "`+manifestTestName+`" returned HTTP 404`) || !strings.Contains(err.Error(), "no space left on device") {
		t.Fatalf("ensureManifestContainer() = %v, want the create failure with the pull's reason", err)
	}
	if creates.Load() != 1 {
		t.Fatalf("creates = %d, want the create to run after a failed pull", creates.Load())
	}
}

func TestEnsureManifestContainerStillCreatesAfterFailedPull(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		switch {
		case r.Method == http.MethodGet && path == "/containers/"+manifestTestName+"/json":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && path == "/images/"+manifestTestImage+"/json":
			w.WriteHeader(http.StatusInternalServerError) // the image check failed; the image may still exist
		case r.Method == http.MethodPost && path == "/images/create":
			writeDockerJSON(w, http.StatusNotFound, map[string]string{"message": "registry unreachable"})
		case r.Method == http.MethodPost && path == "/containers/create":
			writeDockerJSON(w, http.StatusCreated, map[string]string{"Id": "manifest-id"})
		case r.Method == http.MethodPost && path == "/containers/"+manifestTestName+"/start":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
	if err := ensureManifestContainer(context.Background(), DockerConfig{Host: host}, manifestTestName, manifestTestImage, manifestTestPayload); err != nil {
		t.Fatalf("ensureManifestContainer() = %v, want nil: the create succeeded", err)
	}
}

func TestEnsureManifestContainerPullOutlivesShortCallerDeadline(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		switch {
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case path == "/images/create":
			slowPullHandler(w)
		case path == "/containers/create":
			writeDockerJSON(w, http.StatusCreated, map[string]string{"Id": "manifest-id"})
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := ensureManifestContainer(ctx, DockerConfig{Host: host}, manifestTestName, manifestTestImage, manifestTestPayload); err != nil {
		t.Fatalf("ensureManifestContainer() = %v, want the pull to finish as it did on the request client", err)
	}
}
