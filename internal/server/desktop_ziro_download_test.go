package server

import (
	"context"
	"net/http/httptest"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopBinaryDownloadETagMatchesDeliveredBytes(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	data := []byte{0, 255, 128, 1}
	if err := svc.WriteFileBytes(context.Background(), "Shared/model.bin", data, desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handleDesktopDownload(s)(w, httptest.NewRequest("GET", "/api/desktop/download?path=Shared/model.bin", nil))
	if w.Code != 200 || w.Header().Get("ETag") != desktop.NoteVersion(w.Body.Bytes()) || string(w.Body.Bytes()) != string(data) {
		t.Fatalf("binary ETag/body mismatch: status=%d etag=%q", w.Code, w.Header().Get("ETag"))
	}
}
