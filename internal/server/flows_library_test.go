package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFlowFoldersAdmission(t *testing.T) {
	s, admin := newFlowsTestServer(t)
	read := ff1Token(t, s, "folder read", desktopScopeRead)
	write := ff1Token(t, s, "folder write", desktopScopeRead, desktopScopeWrite)
	for _, path := range []string{flowFoldersPath, flowFoldersPath + "/folder_example"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			if !validRouteBearer(s, read, path, method) {
				t.Errorf("read scope refused %s %s", method, path)
			}
		}
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			if validRouteBearer(s, read, path, method) || validRouteBearer(s, write, path, method) || !validRouteBearer(s, admin, path, method) {
				t.Errorf("incorrect mutation scope for %s %s", method, path)
			}
		}
	}
	if isFlowFoldersAPIPath(flowFoldersPath+"-other") || isFlowFoldersAPIPath("/api/desktop/flows/folders") {
		t.Fatal("folder namespace escaped its path")
	}
	for _, tc := range []struct {
		origin string
		allow  bool
	}{{"http://example.com", true}, {"https://evil.example", false}} {
		r := httptest.NewRequest(http.MethodPost, flowFoldersPath, nil)
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		if ok := s.admitFlowFolderRequest(w, r); ok != tc.allow {
			t.Fatalf("origin %q admitted=%v status=%d", tc.origin, ok, w.Code)
		}
	}
	for _, readonly := range []func(bool){func(v bool) { s.Cfg.Tools.Missions.ReadOnly = v }, func(v bool) { s.Cfg.VirtualDesktop.ReadOnly = v }} {
		readonly(true)
		r := httptest.NewRequest(http.MethodPost, flowFoldersPath, nil)
		r.Header.Set("Authorization", "Bearer "+admin)
		w := httptest.NewRecorder()
		if s.admitFlowFolderRequest(w, r) || w.Code != http.StatusForbidden {
			t.Fatalf("readonly status=%d", w.Code)
		}
		readonly(false)
	}
	s.Cfg.Flows.Enabled = false
	r := httptest.NewRequest(http.MethodGet, flowFoldersPath, nil)
	r.Header.Set("Authorization", "Bearer "+admin)
	w := httptest.NewRecorder()
	if s.admitFlowFolderRequest(w, r) || w.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled status=%d", w.Code)
	}
}

func TestFlowLibraryAuditTypesAreRegistered(t *testing.T) {
	actions := []string{"flow_library_change", "flow_version_note"}
	markup := readDashboardMarkupForTest(t)
	for _, action := range actions {
		if !strings.Contains(markup, `<option value="`+action+`" data-i18n="dashboard.audit_type_`+action+`">`) {
			t.Errorf("dashboard audit filter is missing %s", action)
		}
	}
	locales, err := filepath.Glob(filepath.Join("..", "..", "ui", "lang", "dashboard", "*.json"))
	if err != nil || len(locales) != 16 {
		t.Fatalf("dashboard locales = %v, %v", locales, err)
	}
	for _, path := range locales {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var labels map[string]string
		if err := json.Unmarshal(data, &labels); err != nil {
			t.Fatal(err)
		}
		for _, action := range actions {
			if strings.TrimSpace(labels["dashboard.audit_type_"+action]) == "" {
				t.Errorf("%s has no audit label for %s", path, action)
			}
		}
	}
}

func TestFlowLibraryPatchPresenceAndClear(t *testing.T) {
	for _, tc := range []struct {
		body                   string
		ok, setFolder, setTags bool
		folder                 *string
		tags                   int
	}{
		{`{"base_revision":0,"folder_id":null}`, true, true, false, nil, 0},
		{`{"base_revision":2,"tags":[]}`, true, false, true, nil, 0},
		{`{"base_revision":2,"folder_id":"folder_a","tags":["one"]}`, true, true, true, strPtr("folder_a"), 1},
		{`{"base_revision":0}`, false, false, false, nil, 0},
		{`{"tags":[]}`, false, false, false, nil, 0},
		{`{"base_revision":0,"tags":null}`, false, false, false, nil, 0},
		{`{"base_revision":0,"folder_id":""}`, false, false, false, nil, 0},
		{`{"base_revision":0,"folder_id":3}`, false, false, false, nil, 0},
		{`{"base_revision":-1,"tags":[]}`, false, false, false, nil, 0},
	} {
		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		got, ok := decodeFlowLibraryPatch(w, r)
		if ok != tc.ok || ok && (got.SetFolder != tc.setFolder || got.SetTags != tc.setTags || len(got.Tags) != tc.tags || (got.FolderID == nil) != (tc.folder == nil) || got.FolderID != nil && *got.FolderID != *tc.folder) {
			t.Errorf("%s: got %+v ok=%v status=%d", tc.body, got, ok, w.Code)
		}
	}
}

func TestFlowPublishNotePresence(t *testing.T) {
	for _, tc := range []struct {
		raw         json.RawMessage
		ok, present bool
		value       string
	}{
		{nil, true, false, ""}, {json.RawMessage(`""`), true, true, ""},
		{json.RawMessage(`"note"`), true, true, "note"}, {json.RawMessage(`null`), false, false, ""},
		{json.RawMessage(`42`), false, false, ""},
	} {
		w := httptest.NewRecorder()
		value, ok := optionalFlowPublishNote(w, tc.raw)
		if ok != tc.ok || (value != nil) != tc.present || value != nil && *value != tc.value {
			t.Errorf("raw=%s value=%v ok=%v status=%d", tc.raw, value, ok, w.Code)
		}
	}
}

func TestFlowLibraryJSONTextRejectsInvalidUnicode(t *testing.T) {
	for _, tc := range []struct {
		raw   []byte
		valid bool
	}{
		{[]byte(`"ordinary \uD83D\uDE00"`), true},
		{[]byte(`"literal \\uD800"`), true},
		{[]byte("\"replacement � is literal\""), true},
		{[]byte(`"bad \uD800"`), false},
		{[]byte(`"bad \uDC00"`), false},
		{[]byte(`"bad \uD800\u0041"`), false},
		{append([]byte(`"bad `), 0xff, '"'), false},
	} {
		if got := validLibraryJSONText(tc.raw); got != tc.valid {
			t.Errorf("raw %q valid=%v want=%v", tc.raw, got, tc.valid)
		}
	}
	for _, body := range []string{
		`{"base_revision":0,"tags":["bad \uD800"]}`,
		string(append([]byte(`{"base_revision":0,"tags":["`), 0xff, '"', ']', '}')),
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
		if _, ok := decodeFlowLibraryPatch(w, r); ok || w.Code != http.StatusBadRequest {
			t.Errorf("invalid metadata accepted: %d", w.Code)
		}
	}
	for _, raw := range []json.RawMessage{json.RawMessage(`"bad \uD800"`), json.RawMessage(append([]byte{'"'}, 0xff, '"'))} {
		w := httptest.NewRecorder()
		if _, ok := optionalFlowPublishNote(w, raw); ok || w.Code != http.StatusBadRequest {
			t.Errorf("invalid note accepted: %d", w.Code)
		}
		w = httptest.NewRecorder()
		if _, ok := decodeFlowFolderName(w, raw); ok || w.Code != http.StatusBadRequest {
			t.Errorf("invalid folder name accepted: %d", w.Code)
		}
	}
}

func strPtr(s string) *string { return &s }
