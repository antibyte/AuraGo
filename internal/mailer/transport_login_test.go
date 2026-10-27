package mailer

import (
	"strings"
	"testing"

	"aurago/internal/security"
)

// The SMTP and IMAP user name is the user's mailbox address: redacted from output but
// not a credential, so a mail the agent writes may quote it. The password is a secret.
func TestRegisterLoginRegistersTheUserNameAsAnIdentity(t *testing.T) {
	const username = "mailer-login-owner@example.invalid"
	const password = "mailer-login-pw-5c1e8a7f"
	registerLogin(username, password)

	text := "from " + username + " with " + password
	if got := security.Scrub(text); strings.Contains(got, username) || strings.Contains(got, password) {
		t.Fatalf("Scrub left the login in the output: %q", got)
	}
	if got := security.ScrubSecrets("from " + username); got != "from "+username {
		t.Fatalf("the user name was reported as a secret: %q", got)
	}
	if got := security.ScrubSecrets(text); strings.Contains(got, password) {
		t.Fatalf("the password was not reported as a secret: %q", got)
	}
}
