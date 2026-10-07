package server

import (
	"encoding/json"
	"testing"
)

// The status issue must use the same code the gated endpoints return, so the
// editor can show the "disabled in Settings" guidance instead of a generic one.
func TestVideoStudioStatusIssueMatchesGatedEndpointCodes(t *testing.T) {
	s, readToken, _ := videoStudioAPIFixture(t)
	for _, tc := range []struct {
		name    string
		studio  bool
		desktop bool
		want    string
	}{
		{name: "studio disabled", studio: false, desktop: true, want: "video_studio_disabled"},
		{name: "desktop disabled", studio: true, desktop: false, want: "desktop_disabled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s.Cfg.VideoStudio.Enabled = tc.studio
			s.Cfg.VirtualDesktop.Enabled = tc.desktop
			status := videoStudioAPIRequest(s, readToken, "GET", "/status", nil, nil)
			if status.Code != 200 {
				t.Fatalf("status: %d %s", status.Code, status.Body.String())
			}
			var body struct {
				Issue string `json:"issue"`
			}
			if err := json.Unmarshal(status.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Issue != tc.want {
				t.Fatalf("status issue = %q, want %q", body.Issue, tc.want)
			}
			projects := videoStudioAPIRequest(s, readToken, "GET", "/projects", nil, nil)
			var gated struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(projects.Body.Bytes(), &gated); err != nil {
				t.Fatal(err)
			}
			if projects.Code != 503 || gated.Code != body.Issue {
				t.Fatalf("projects gate = %d %q, want 503 with status issue %q", projects.Code, gated.Code, body.Issue)
			}
		})
	}
}
