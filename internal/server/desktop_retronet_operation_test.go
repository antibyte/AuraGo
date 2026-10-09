package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDesktopOperationClassifiesRetroNetRoutes(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         desktopOperation
	}{
		{http.MethodGet, "/api/desktop/retronet/connect", desktopExecute},
		{http.MethodGet, "/api/desktop/retronet/directory", desktopRead},
		{http.MethodPost, "/api/desktop/retronet/status", desktopWrite},
	} {
		if got := desktopRequestOperation(httptest.NewRequest(tc.method, tc.path+"?entry=telehack", nil)); got != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, got, tc.want)
		}
	}
	s, _, writeToken := testDesktopPermissionServer(t)
	s.Cfg.VirtualDesktop.ReadOnly = true
	r := httptest.NewRequest(http.MethodGet, "/api/desktop/retronet/connect?entry=telehack", nil)
	r.Header.Set("Authorization", "Bearer "+writeToken)
	w := httptest.NewRecorder()
	if requireDesktopPermission(s, w, r, desktopScopeWrite) || w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), `"code":"desktop_readonly"`) {
		t.Fatalf("readonly did not block Retro-Net connect: %d %s", w.Code, w.Body.String())
	}
}
