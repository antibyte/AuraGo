package config

import "testing"

func TestEmailAccountAgentAccessPreservesLegacyAndDeniesUnsupportedTransports(t *testing.T) {
	allow, deny := true, false
	cases := []struct {
		name    string
		account EmailAccount
		want    bool
	}{
		{name: "legacy IMAP defaults to allowed", account: EmailAccount{ReceiveProtocol: "imap"}, want: true},
		{name: "legacy omitted protocol remains IMAP", account: EmailAccount{}, want: true},
		{name: "explicit denial", account: EmailAccount{ReceiveProtocol: "imap", AgentAccess: &deny}, want: false},
		{name: "explicit grant", account: EmailAccount{ReceiveProtocol: "imap", AgentAccess: &allow}, want: true},
		{name: "POP3 stays outside agent tools", account: EmailAccount{ReceiveProtocol: "pop3", AgentAccess: &allow}, want: false},
		{name: "OAuth stays outside legacy tools", account: EmailAccount{ReceiveProtocol: "imap", OAuthSource: "gog", AgentAccess: &allow}, want: false},
		{name: "XOAUTH2 stays outside legacy tools", account: EmailAccount{ReceiveProtocol: "imap", SMTPAuthMethod: "xoauth2", AgentAccess: &allow}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.account.AllowsAgentAccess(); got != tc.want {
				t.Fatalf("AllowsAgentAccess() = %t, want %t", got, tc.want)
			}
		})
	}
}
