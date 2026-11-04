package agentmail

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/httporigin"
)

func TestClientRejectsInsecureBaseURLs(t *testing.T) {
	for _, base := range []string{"http://mail.example.test", "https://user:private@mail.example.test", "https://mail.example.test?key=private", "https://mail.example.test#private"} {
		if _, err := NewClient(ClientConfig{BaseURL: base, APIKey: "fixture-only"}); err == nil || strings.Contains(err.Error(), "private") {
			t.Fatalf("unsafe base accepted or echoed: %v", err)
		}
	}
}

func TestClientDefaultBlocksPrivateTargets(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "0")
	c, err := NewClient(ClientConfig{BaseURL: "https://127.0.0.1:1", APIKey: "fixture-only", DisableRetries: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := c.ListInboxes(ctx, ListInboxesOptions{}); err == nil || !strings.Contains(err.Error(), "SSRF") {
		t.Fatalf("default client did not block loopback before dialing: %v", err)
	}
}

func TestClientBindsInjectedClientRedirectsWithoutMutation(t *testing.T) {
	for _, location := range []string{"http://api.example.test/v0/inboxes", "https://foreign.example.test/v0/inboxes", "https://api.example.test:444/v0/inboxes", "https://user@api.example.test/v0/inboxes"} {
		t.Run(location, func(t *testing.T) {
			calls := 0
			original := &http.Client{Transport: attachmentRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {location}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
			})}
			c, err := NewClient(ClientConfig{BaseURL: "https://api.example.test", APIKey: "fixture-only", HTTPClient: original, DisableRetries: true})
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.ListInboxes(context.Background(), ListInboxesOptions{})
			if !errors.Is(err, httporigin.ErrCrossOriginRedirect) || calls != 1 || original.CheckRedirect != nil {
				t.Fatalf("redirect escaped or shared client mutated: calls=%d err=%v", calls, err)
			}
		})
	}
	strict := errors.New("custom redirect denied")
	original := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return strict }}
	c, err := NewClient(ClientConfig{BaseURL: "https://api.example.test", APIKey: "fixture-only", HTTPClient: original})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest(http.MethodGet, "https://api.example.test/v0/inboxes", nil)
	if !errors.Is(c.httpClient.CheckRedirect(r, []*http.Request{r}), strict) {
		t.Fatal("stricter injected redirect policy was lost")
	}
}

func TestAPIErrorKeepsProviderTextPrivate(t *testing.T) {
	err := parseAPIError(http.StatusForbidden, []byte(`{"message":"private-key private-recipient"}`))
	if strings.Contains(err.Error(), "private") || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Message, "private-key") {
		t.Fatal("provider fields were exposed or internal diagnostics lost")
	}
}
