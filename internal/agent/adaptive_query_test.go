package agent

import (
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestAdaptiveRankingQueryKeepsOnlyTrustedText(t *testing.T) {
	email := "[EMAIL NOTIFICATION] Account: Work (me@example.com) - 2 new email(s) in INBOX.\n\nEmail summary:\n" +
		security.IsolateExternalData("newsletter: music, pixel art, Jellyfin tips") +
		"\n\nYou can use fetch_email with account \"work\" for full content or send_email to reply."
	twoFences := "Please check the first mail and then the second mail carefully.\n" +
		security.IsolateExternalData("alpha secret text") + "\nand afterwards summarize everything for me\n" +
		security.IsolateExternalData("omega secret text")
	cases := []struct {
		name      string
		in        string
		unchanged bool
		keep      []string
		drop      []string
	}{
		{name: "email notification", in: email, keep: []string{"fetch_email", "send_email"}, drop: []string{"Jellyfin", "music", "pixel art"}},
		{name: "everything fenced", in: security.IsolateExternalData("schreib einen Tagebucheintrag"), unchanged: true},
		{name: "short prefix", in: "Telegram: " + security.IsolateExternalData("schreib einen Tagebucheintrag"), unchanged: true},
		{name: "two fences", in: twoFences, keep: []string{"first mail", "summarize everything"}, drop: []string{"alpha", "omega"}},
		{name: "unclosed fence", in: "Check the mail from the boss today please <external_data>\nhidden text", unchanged: true},
		{name: "no fence", in: "Restart the docker container and check the cpu usage", unchanged: true},
	}
	for _, tc := range cases {
		got, external := adaptiveRankingQuery(tc.in)
		if tc.unchanged && external != "" {
			t.Errorf("%s: external = %q for an unchanged message", tc.name, external)
		}
		for _, want := range tc.drop {
			if !strings.Contains(external, want) {
				t.Errorf("%s: external %q lacks %q", tc.name, external, want)
			}
		}
		if tc.unchanged && got != tc.in {
			t.Errorf("%s: changed to %q", tc.name, got)
		}
		for _, want := range tc.keep {
			if !strings.Contains(got, want) {
				t.Errorf("%s: %q lacks %q", tc.name, got, want)
			}
		}
		for _, bad := range tc.drop {
			if strings.Contains(got, bad) {
				t.Errorf("%s: %q still has %q", tc.name, got, bad)
			}
		}
	}
}
