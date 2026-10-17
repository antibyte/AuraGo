package mailer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/httporigin"
)

type gmailFixtureTransport func(*http.Request) (*http.Response, error)

func (f gmailFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGmailRequestsBindRedirectsToOrigin(t *testing.T) {
	for _, target := range []string{"https://other.example/profile", "http://gmail.googleapis.com/profile", "https://gmail.googleapis.com:444/profile"} {
		t.Run(target, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: gmailFixtureTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusTemporaryRedirect, Header: http.Header{"Location": {target}}, Body: io.NopCloser(strings.NewReader(""))}, nil
			})}
			a := &gmailAdapter{cfg: TransportConfig{HTTPClient: client, Token: func(context.Context, string, bool) (string, error) { return "fixture-token", nil }}}
			if err := a.Test(context.Background()); !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
				t.Fatalf("redirect error = %v", err)
			}
			if calls != 1 || client.CheckRedirect != nil {
				t.Fatal("redirect escaped the origin or mutated the shared client")
			}
		})
	}
}

func TestGmailExpiredHistoryStartsFreshSnapshot(t *testing.T) {
	var historyCalls, profileCalls, messageListCalls int
	client := &http.Client{Transport: gmailFixtureTransport(func(r *http.Request) (*http.Response, error) {
		var status int
		var body string
		switch {
		case strings.HasSuffix(r.URL.Path, "/history"):
			historyCalls++
			status, body = http.StatusNotFound, `{"error":{"status":"NOT_FOUND"}}`
		case strings.HasSuffix(r.URL.Path, "/profile"):
			profileCalls++
			status, body = http.StatusOK, `{"historyId":"98765"}`
		case strings.HasSuffix(r.URL.Path, "/messages"):
			messageListCalls++
			status, body = http.StatusOK, `{"messages":[]}`
		default:
			t.Fatalf("unexpected Gmail endpoint %s", r.URL)
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	a := &gmailAdapter{cfg: TransportConfig{ID: "google", Source: "gog", HTTPClient: client, Token: func(context.Context, string, bool) (string, error) { return "fixture-token", nil }}}
	result, err := a.Sync(context.Background(), "", encodeGmailCursor(gmailSyncCursor{Version: 1, HistoryID: "old-history"}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.ResetSnapshot || !result.RemoteSnapshotComplete || result.HasMore || historyCalls != 1 || profileCalls != 1 || messageListCalls != 1 {
		t.Fatalf("expired history did not restart a complete snapshot: result=%#v history=%d profile=%d messages=%d", result, historyCalls, profileCalls, messageListCalls)
	}
	state, err := decodeGmailCursor(result.NextCursor)
	if err != nil || state.HistoryID != "98765" || state.Snapshot {
		t.Fatalf("snapshot cursor did not advance to fresh history ID: %#v err=%v", state, err)
	}
}

func TestGmailMetadataIncludesReplyTo(t *testing.T) {
	client := &http.Client{Transport: gmailFixtureTransport(func(r *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(r.URL.Path, "/messages/msg-1") || !containsMailString(r.URL.Query()["metadataHeaders"], "Reply-To") {
			t.Fatalf("Gmail metadata request omitted Reply-To: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		body := `{"threadId":"thread-1","payload":{"headers":[{"name":"Reply-To","value":"Support <support@example.test>, Ops <ops@example.test>"}]}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	adapter := &gmailAdapter{cfg: TransportConfig{ID: "google", Source: "gog", HTTPClient: client, Token: func(context.Context, string, bool) (string, error) { return "fixture-token", nil }}}
	message, err := adapter.readMetadata(context.Background(), "msg-1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"support@example.test", "ops@example.test"}
	if !reflect.DeepEqual(message.ReplyTo, want) {
		t.Fatalf("Gmail Reply-To = %#v, want %#v", message.ReplyTo, want)
	}
}

func TestGmailCreateFolderUsesCreateGrant(t *testing.T) {
	var seenGrant string
	client := &http.Client{Transport: gmailFixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/labels") {
			t.Fatalf("unexpected Gmail label-create request: %s %s", r.Method, r.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"id":"Label_42","name":"Fixture"}`)), Header: make(http.Header)}, nil
	})}
	adapter := &gmailAdapter{cfg: TransportConfig{ID: "google", Source: "gog", HTTPClient: client, Token: func(_ context.Context, grant string, _ bool) (string, error) {
		seenGrant = grant
		return "fixture-token", nil
	}}}
	folder, err := adapter.CreateFolder(context.Background(), "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	if seenGrant != "create" || folder.ProviderID != "Label_42" {
		t.Fatalf("grant=%q folder=%#v", seenGrant, folder)
	}
}

func TestGmailSpamTrashSearchAndInitialSnapshotIncludeHiddenMessages(t *testing.T) {
	for _, mode := range []string{"search", "snapshot"} {
		for _, folder := range []string{"SPAM", "TRASH"} {
			t.Run(mode+"/"+folder, func(t *testing.T) {
				var listCalls int
				client := &http.Client{Transport: gmailFixtureTransport(func(r *http.Request) (*http.Response, error) {
					var body string
					switch {
					case strings.HasSuffix(r.URL.Path, "/profile"):
						body = `{"historyId":"123"}`
					case strings.HasSuffix(r.URL.Path, "/messages"):
						listCalls++
						q := r.URL.Query()
						if q.Get("labelIds") != folder || q.Get("includeSpamTrash") != "true" {
							t.Fatalf("folder listing query = %s, want labelIds=%s and includeSpamTrash=true", r.URL.RawQuery, folder)
						}
						body = `{"messages":[]}`
					default:
						t.Fatalf("unexpected Gmail endpoint %s", r.URL)
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
				})}
				a := &gmailAdapter{cfg: TransportConfig{ID: "google", Source: "gog", HTTPClient: client, Token: func(context.Context, string, bool) (string, error) { return "fixture-token", nil }}}
				var err error
				if mode == "search" {
					_, err = a.Search(context.Background(), folder, MessageQuery{Query: "newer_than:7d"})
				} else {
					_, err = a.Sync(context.Background(), folder, "")
				}
				if err != nil {
					t.Fatal(err)
				}
				if listCalls != 1 {
					t.Fatalf("message list calls = %d, want 1", listCalls)
				}
			})
		}
	}
}
