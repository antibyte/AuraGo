package server

import (
	"encoding/json"
	"testing"

	"aurago/internal/mailer"
)

func TestMailerStateAdvertisesExistingAttachmentLimits(t *testing.T) {
	s, _ := newMailerHTTPTestServer(t)
	for _, readonly := range []bool{false, true} {
		s.Cfg.VirtualDesktop.ReadOnly = readonly
		response := mailerHTTPRequest(s, "GET", "state", "")
		if response.Code != 200 {
			t.Fatalf("state: %d %s", response.Code, response.Body)
		}
		var state struct {
			Readonly bool                    `json:"readonly"`
			Limits   mailer.AttachmentLimits `json:"attachment_limits"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		if state.Readonly != readonly || state.Limits.MaxFileBytes != 25<<20 || state.Limits.MaxMessageBytes != 25<<20 {
			t.Fatalf("unexpected state: %+v", state)
		}
	}
}
