package server

import (
	"context"
	"net/http"
	"testing"
)

func TestStorymakerReviewSheetRoute(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	id := createStorymakerProject(t, s, writeToken).Project.ID
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	path := "/projects/" + id + "/review/contact-sheet.png"

	w := storymakerRequest(s, readToken, http.MethodGet, path, nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("no sheet yet: %d %s", w.Code, w.Body.String())
	}
	data := []byte("\x89PNG\r\n\x1a\nsheet")
	if err := rt.projects.SaveReviewSheet(context.Background(), id, data); err != nil {
		t.Fatal(err)
	}
	w = storymakerRequest(s, readToken, http.MethodGet, path, nil, nil)
	if w.Code != http.StatusOK || w.Body.String() != string(data) {
		t.Fatalf("GET = %d, %d bytes", w.Code, w.Body.Len())
	}
	for header, want := range map[string]string{
		"Content-Type": "image/png", "X-Content-Type-Options": "nosniff", "Content-Security-Policy": "sandbox; default-src 'none'", "Cache-Control": "private, no-cache",
	} {
		if got := w.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/"+id+"/review/other.png", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("another file name = %d", w.Code)
	}
	if w := storymakerRequest(s, writeToken, http.MethodPost, path, nil, nil); w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST = %d, Allow %q", w.Code, w.Header().Get("Allow"))
	}
}
