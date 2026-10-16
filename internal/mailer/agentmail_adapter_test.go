package mailer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/agentmail"
)

type agentMailFixtureTransport func(*http.Request) (*http.Response, error)

func (f agentMailFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAgentMailSyncCompletesOnlyOnTerminalPage(t *testing.T) {
	var requests int
	client := &http.Client{Transport: agentMailFixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v0/inboxes/inbox/messages" || r.URL.Query().Get("limit") != "100" {
			t.Fatalf("unexpected AgentMail list request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		requests++
		if r.URL.Query().Get("labels") != "received" {
			t.Fatalf("Inbox sync did not query the received system label: %s", r.URL.RawQuery)
		}
		body := `{"messages":[{"message_id":"m-1","thread_id":"t-1","labels":["received"]}],"next_page_token":"cursor-2"}`
		if r.URL.Query().Get("page_token") == "cursor-2" {
			body = `{"messages":[{"message_id":"m-2","thread_id":"t-2","labels":["received"]}]}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	adapter, err := newAgentMailAdapter(TransportConfig{ID: "inbox", Source: "agentmail", AgentMailInboxID: "inbox", AgentMailAPIKey: "fixture", AgentMailBaseURL: "https://api.agentmail.test", HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	first, err := adapter.Sync(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.RemoteSnapshotComplete || !first.HasMore || first.NextCursor != "cursor-2" {
		t.Fatalf("nonterminal page completed snapshot: %#v", first)
	}
	second, err := adapter.Sync(context.Background(), "", first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if !second.RemoteSnapshotComplete || second.HasMore || second.NextCursor != "" || requests != 2 {
		t.Fatalf("terminal page did not complete snapshot: %#v requests=%d", second, requests)
	}
}

func TestAgentMailSyncSeparatesVirtualFoldersByProviderLabels(t *testing.T) {
	cases := []struct {
		folder       string
		labelQuery   string
		includeSpam  string
		includeTrash string
		want         []string
	}{
		{folder: "inbox", labelQuery: "received", want: []string{"inbox-message"}},
		{folder: "sent", labelQuery: "sent", want: []string{"sent-message"}},
		{folder: "archive", labelQuery: "archive", want: []string{"archive-message"}},
		{folder: "trash", labelQuery: "trash", includeTrash: "true", want: []string{"trash-message"}},
		{folder: "spam", labelQuery: "spam", includeSpam: "true", want: []string{"spam-message"}},
		{folder: "junk", labelQuery: "junk", want: []string{"junk-message"}},
	}
	fixtureMessages := `[
		{"message_id":"inbox-message","labels":["received","unread"]},
		{"message_id":"archive-message","labels":["received","archive"]},
		{"message_id":"sent-message","labels":["sent"]},
		{"message_id":"trash-message","labels":["received","trash"]},
		{"message_id":"spam-message","labels":["received","spam"]},
		{"message_id":"junk-message","labels":["received","junk"]}
	]`
	for _, tc := range cases {
		t.Run(tc.folder, func(t *testing.T) {
			client := &http.Client{Transport: agentMailFixtureTransport(func(r *http.Request) (*http.Response, error) {
				q := r.URL.Query()
				if q.Get("labels") != tc.labelQuery || q.Get("include_spam") != tc.includeSpam || q.Get("include_trash") != tc.includeTrash {
					t.Fatalf("wrong provider folder query for %s: %s", tc.folder, r.URL.RawQuery)
				}
				body := `{"messages":` + fixtureMessages + `}`
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			adapter, err := newAgentMailAdapter(TransportConfig{ID: "inbox", Source: "agentmail", AgentMailInboxID: "folder-" + tc.folder, AgentMailAPIKey: "fixture", AgentMailBaseURL: "https://api.agentmail.test", HTTPClient: client})
			if err != nil {
				t.Fatal(err)
			}
			page, err := adapter.Sync(context.Background(), agentMailFolderID(adapter.(*agentMailAdapter).accountID(), tc.folder), "")
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, 0, len(page.Messages))
			for _, message := range page.Messages {
				got = append(got, message.RemoteID)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("folder %s got messages %#v, want %#v", tc.folder, got, tc.want)
			}
		})
	}
}

func TestAgentMailArchiveMutationUsesTheVirtualInboxFilter(t *testing.T) {
	client := &http.Client{Transport: agentMailFixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPatch && r.URL.Path == "/v0/inboxes/inbox/messages/inbox-message" {
			var payload agentmail.UpdateMessageRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(payload.AddLabels, []string{"archive"}) || !reflect.DeepEqual(payload.RemoveLabels, []string{"trash", "junk"}) {
				t.Fatalf("archive mutation did not use virtual archive labels: %#v", payload)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"message_id":"inbox-message","labels":["received","archive"]}`)), Header: make(http.Header)}, nil
		}
		if r.Method == http.MethodGet && r.URL.Path == "/v0/inboxes/inbox/messages" {
			if r.URL.Query().Get("labels") != "received" {
				t.Fatalf("virtual Inbox does not use received label: %s", r.URL.RawQuery)
			}
			body := `{"messages":[{"message_id":"inbox-message","labels":["received","archive"]}]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		t.Fatalf("unexpected AgentMail request: %s %s", r.Method, r.URL.Path)
		return nil, nil
	})}
	adapter, err := newAgentMailAdapter(TransportConfig{ID: "inbox", Source: "agentmail", AgentMailInboxID: "inbox", AgentMailAPIKey: "fixture", AgentMailBaseURL: "https://api.agentmail.test", HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Mutate(context.Background(), "", "inbox-message", Mutation{Kind: MutationArchive}); err != nil {
		t.Fatal(err)
	}
	page, err := adapter.Sync(context.Background(), agentMailFolderID(adapter.(*agentMailAdapter).accountID(), "inbox"), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 0 {
		t.Fatalf("archived message still appeared in Inbox: %#v", page.Messages)
	}
}

func TestAgentMailJunkMutationUsesCustomLabelAndRejectsSystemLabels(t *testing.T) {
	var updates int
	client := &http.Client{Transport: agentMailFixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v0/inboxes/inbox/messages/message-1" {
			t.Fatalf("unexpected AgentMail mutation request: %s %s", r.Method, r.URL.Path)
		}
		updates++
		var payload agentmail.UpdateMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(payload.AddLabels, []string{"junk"}) || !reflect.DeepEqual(payload.RemoveLabels, []string{"trash", "archive"}) {
			t.Fatalf("junk action attempted to mutate a provider system label: %#v", payload)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"message_id":"message-1","labels":["received","junk"]}`)), Header: make(http.Header)}, nil
	})}
	adapter, err := newAgentMailAdapter(TransportConfig{ID: "inbox", Source: "agentmail", AgentMailInboxID: "inbox", AgentMailAPIKey: "fixture", AgentMailBaseURL: "https://api.agentmail.test", HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Mutate(context.Background(), "", "message-1", Mutation{Kind: MutationJunk}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Mutate(context.Background(), "", "message-1", Mutation{Kind: MutationLabelAdd, Label: "received"}); err == nil {
		t.Fatal("system label assignment was not rejected")
	}
	if updates != 1 {
		t.Fatalf("system-label rejection made a provider mutation; update count=%d", updates)
	}
}

func TestAgentMailFoldersDiscoverLabelsAcrossEveryPage(t *testing.T) {
	var requests int
	client := &http.Client{Transport: agentMailFixtureTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		q := r.URL.Query()
		if label := q.Get("labels"); label == "trash" {
			if q.Get("include_trash") != "true" {
				t.Fatalf("trash folder probe omitted include_trash: %s", r.URL.RawQuery)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"messages":[]}`)), Header: make(http.Header)}, nil
		}
		if label := q.Get("labels"); label == "spam" {
			if q.Get("include_spam") != "true" {
				t.Fatalf("spam folder probe omitted include_spam: %s", r.URL.RawQuery)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"messages":[]}`)), Header: make(http.Header)}, nil
		}
		body := `{"messages":[{"message_id":"m1","labels":["received","unread","first-label"]}],"next_page_token":"p2"}`
		if q.Get("page_token") == "p2" {
			body = `{"messages":[{"message_id":"m2","labels":["sent","late-label"]}]}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	adapter, err := newAgentMailAdapter(TransportConfig{ID: "inbox", Source: "agentmail", AgentMailInboxID: "folder-discovery", AgentMailAPIKey: "fixture", AgentMailBaseURL: "https://api.agentmail.test", HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	folders, err := adapter.Folders(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(folders))
	for _, folder := range folders {
		names[folder.Name] = true
	}
	for _, name := range []string{"inbox", "sent", "archive", "junk", "trash", "spam", "first-label", "late-label"} {
		if !names[name] {
			t.Errorf("folder %q was not discovered: %#v", name, names)
		}
	}
	for _, name := range []string{"received", "unread"} {
		if names[name] {
			t.Errorf("system label %q was exposed as a folder", name)
		}
	}
	if requests != 4 {
		t.Fatalf("discovery/probe requests = %d, want 2 pages plus 2 protected-folder probes", requests)
	}
}

func TestAgentMailFoldersHideProtectedSpamAndTrashWhenKeyCannotReadThem(t *testing.T) {
	client := &http.Client{Transport: agentMailFixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("labels") == "spam" || r.URL.Query().Get("labels") == "trash" {
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(`{"error":"permission denied"}`)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"messages":[]}`)), Header: make(http.Header)}, nil
	})}
	adapter, err := newAgentMailAdapter(TransportConfig{ID: "inbox", Source: "agentmail", AgentMailInboxID: "folder-permissions", AgentMailAPIKey: "fixture", AgentMailBaseURL: "https://api.agentmail.test", HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	folders, err := adapter.Folders(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(folders))
	for _, folder := range folders {
		names[folder.Name] = true
	}
	if names["trash"] || names["spam"] || !names["inbox"] || !names["sent"] {
		t.Fatalf("folder visibility ignored provider label permissions: %#v", names)
	}
}

func TestAgentMailMessageCopiesReplyToWhenPresent(t *testing.T) {
	message := agentMailMessage(TransportConfig{ID: "inbox", Source: "agentmail"}, agentmail.Message{
		ID:      "message-1",
		ReplyTo: []agentmail.Address{{Name: "Support", Email: "support@example.test"}},
		Headers: map[string]string{"Message-ID": "<message-1@example.test>", "In-Reply-To": "<message-0@example.test>", "References": "<message-previous@example.test> <message-0@example.test>"},
		HTML:    `<p>safe</p><script>bad()</script>`,
	})
	want := []string{"support@example.test"}
	if !reflect.DeepEqual(message.ReplyTo, want) {
		t.Fatalf("AgentMail Reply-To = %#v, want %#v", message.ReplyTo, want)
	}
	if message.MessageID != "<message-1@example.test>" || message.InReplyTo != "<message-0@example.test>" || !reflect.DeepEqual(message.References, []string{"<message-previous@example.test>", "<message-0@example.test>"}) {
		t.Fatalf("AgentMail thread metadata was not copied: %#v", message)
	}
	if strings.Contains(strings.ToLower(message.HTML), "script") {
		t.Fatal("AgentMail HTML was not sanitized")
	}
}

func TestAgentMailUnreadLabelPrecedence(t *testing.T) {
	cases := []struct {
		name   string
		labels []string
		want   bool
	}{
		{name: "received defaults unread", labels: []string{"received"}, want: true},
		{name: "received and unread", labels: []string{"received", "unread"}, want: true},
		{name: "explicit read", labels: []string{"received", "read"}, want: false},
		{name: "read overrides conflicting unread", labels: []string{"received", "unread", "read"}, want: false},
		{name: "explicit unread without received", labels: []string{"unread"}, want: true},
		{name: "sent only remains read", labels: []string{"sent"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := agentMailMessage(TransportConfig{}, agentmail.Message{Labels: tc.labels}).Unread
			if got != tc.want {
				t.Fatalf("Unread for labels %v = %v, want %v", tc.labels, got, tc.want)
			}
		})
	}
}

func TestAgentMailReplyUsesReviewedRecipientsAndAttachments(t *testing.T) {
	client := &http.Client{Transport: agentMailFixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && r.URL.Path == "/v0/inboxes/inbox/messages/original-1" {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"message_id":"original-1","subject":"topic","headers":{"Message-ID":"<original@example.test>"}}`)), Header: make(http.Header)}, nil
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v0/inboxes/inbox/messages/original-1/reply" {
			t.Fatalf("reply did not use provider threading endpoint: %s %s", r.Method, r.URL.Path)
		}
		var payload agentmail.ReplyMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(payload.To, []string{"to@example.test"}) || !reflect.DeepEqual(payload.CC, []string{"cc@example.test"}) || !reflect.DeepEqual(payload.BCC, []string{"bcc@example.test"}) {
			t.Fatalf("reply recipients were changed: %#v", payload)
		}
		if payload.Text != "reply body" || strings.Contains(strings.ToLower(payload.HTML), "script") || len(payload.Attachments) != 1 || payload.Attachments[0].ContentBase64 != "YXR0YWNobWVudA==" {
			t.Fatalf("reply content or attachment was lost/unsafe: %#v", payload)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"message_id":"reply-1","thread_id":"thread-1"}`)), Header: make(http.Header)}, nil
	})}
	adapter, err := newAgentMailAdapter(TransportConfig{ID: "inbox", Source: "agentmail", AgentMailInboxID: "inbox", AgentMailAPIKey: "fixture", AgentMailBaseURL: "https://api.agentmail.test", HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	draft := Draft{ID: "draft-1", ReplyRemoteID: "original-1", InReplyTo: "<original@example.test>", To: []string{"to@example.test"}, CC: []string{"cc@example.test"}, BCC: []string{"bcc@example.test"}, Subject: "Re: topic", Text: "reply body", HTML: `<p>safe</p><script>bad()</script>`, Attachments: []Attachment{{Filename: "note.txt", ContentType: "text/plain", Data: []byte("attachment")}}}
	result, err := adapter.Send(context.Background(), Outgoing{Draft: draft})
	if err != nil || !result.Confirmed || result.RemoteID != "reply-1" || result.Message == nil || result.Message.ThreadID != "thread-1" {
		t.Fatalf("AgentMail reply failed: result=%#v err=%v", result, err)
	}
}

func TestAgentMailReplySubjectEditsAndAccountChangesUseValidatedRFCHeaders(t *testing.T) {
	cases := []struct {
		name          string
		replyRemoteID string
		subject       string
		wantGet       bool
	}{
		{name: "edited subject", replyRemoteID: "original-1", subject: "A reviewed new subject", wantGet: true},
		{name: "account changed", subject: "Re: topic"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: agentMailFixtureTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodGet {
					if !tc.wantGet || r.URL.Path != "/v0/inboxes/inbox/messages/original-1" {
						t.Fatalf("unexpected reply preflight: %s %s", r.Method, r.URL.Path)
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"message_id":"original-1","subject":"topic","headers":{"Message-ID":"<original@example.test>"}}`)), Header: make(http.Header)}, nil
				}
				if r.Method != http.MethodPost || r.URL.Path != "/v0/inboxes/inbox/messages/send" {
					t.Fatalf("reviewed subject/account-change reply did not use explicit send: %s %s", r.Method, r.URL.Path)
				}
				var payload agentmail.SendMessageRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload.Subject != tc.subject || !reflect.DeepEqual(payload.To, []string{"to@example.test"}) || !reflect.DeepEqual(payload.CC, []string{"cc@example.test"}) || !reflect.DeepEqual(payload.BCC, []string{"bcc@example.test"}) {
					t.Fatalf("reviewed subject or recipients changed: %#v", payload)
				}
				if payload.Headers["In-Reply-To"] != "<original@example.test>" || payload.Headers["References"] != "<root@example.test> <original@example.test>" {
					t.Fatalf("RFC thread headers were not preserved: %#v", payload.Headers)
				}
				if payload.Text != "reply body" || len(payload.Attachments) != 1 || payload.Attachments[0].ContentBase64 != "YXR0YWNobWVudA==" {
					t.Fatalf("reply body or attachment changed: %#v", payload)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"message_id":"sent-1","thread_id":"thread-new"}`)), Header: make(http.Header)}, nil
			})}
			adapter, err := newAgentMailAdapter(TransportConfig{ID: "inbox", Source: "agentmail", AgentMailInboxID: "inbox", AgentMailAPIKey: "fixture", AgentMailBaseURL: "https://api.agentmail.test", HTTPClient: client})
			if err != nil {
				t.Fatal(err)
			}
			draft := Draft{ID: "draft-1", ReplyRemoteID: tc.replyRemoteID, InReplyTo: "<original@example.test>", References: []string{"<root@example.test>"}, To: []string{"to@example.test"}, CC: []string{"cc@example.test"}, BCC: []string{"bcc@example.test"}, Subject: tc.subject, Text: "reply body", Attachments: []Attachment{{Filename: "note.txt", ContentType: "text/plain", Data: []byte("attachment")}}}
			result, err := adapter.Send(context.Background(), Outgoing{Draft: draft})
			if err != nil || !result.Confirmed || result.RemoteID != "sent-1" {
				t.Fatalf("threaded generic send failed: result=%#v err=%v", result, err)
			}
		})
	}
}
