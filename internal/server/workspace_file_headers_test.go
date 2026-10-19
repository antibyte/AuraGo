package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkspaceFilesDownloadUnknownAndActiveContent(t *testing.T) {
	for _, path := range []string{"/files/unknown", "/files/payload.foo", "/files/page.html", "/files/vector.svg", "/files/script.js"} {
		response := httptest.NewRecorder()
		setWorkspaceFileResponseHeaders(response, httptest.NewRequest("GET", path, nil))
		if response.Header().Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(response.Header().Get("Content-Disposition"), "attachment") || !strings.Contains(response.Header().Get("Content-Security-Policy"), "sandbox") {
			t.Fatalf("unsafe headers for %s", path)
		}
	}
	for _, path := range []string{"/files/picture.png", "/files/music.mp3", "/files/movie.mp4", "/files/document.pdf"} {
		response := httptest.NewRecorder()
		setWorkspaceFileResponseHeaders(response, httptest.NewRequest("GET", path, nil))
		if response.Header().Get("Content-Disposition") != "" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("media download forced for %s", path)
		}
	}
}
