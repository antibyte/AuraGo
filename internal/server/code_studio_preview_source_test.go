package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCodeStudioPreviewWorkspaceIDFromRequest(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	h := codeStudioHandlers{server: s}
	for _, tc := range []struct {
		header, query, want string
		notFound            bool
	}{
		{"", "", "local", false},
		{"local", "", "local", false},
		{"", "local", "local", false},
		{"ssh-1", "", "", true},
		{"", "ssh-1", "", true},
	} {
		target := "/api/code-studio/preview"
		if tc.query != "" {
			target += "?ws=" + tc.query
		}
		req := httptest.NewRequest(http.MethodPost, target, nil)
		if tc.header != "" {
			req.Header.Set("X-Code-Studio-Workspace", tc.header)
		}
		got, err := h.previewWorkspaceID(req)
		if tc.notFound {
			if !errors.Is(err, errCodeStudioPreviewWorkspaceNotFound) || !errors.Is(err, errCodeStudioWorkspaceNotFound) {
				t.Fatalf("%+v: err = %v, want workspace not found", tc, err)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("%+v: got %q, %v; want %q", tc, got, err, tc.want)
		}
	}
	if _, err := h.previewSource(context.Background(), "ssh-gone"); !errors.Is(err, errCodeStudioPreviewWorkspaceNotFound) {
		t.Fatalf("previewSource unknown id = %v, want workspace not found", err)
	}
}
