package webhooks

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
)

func TestDeliverMessageDoesNotFollowRedirects(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect, http.StatusFound, http.StatusMultipleChoices} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var sourceCalls, targetCalls atomic.Int32
			var forwardedToken atomic.Bool
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				targetCalls.Add(1)
				forwardedToken.Store(r.Header.Get("X-Internal-Token") != "")
				w.WriteHeader(http.StatusOK)
			}))
			defer target.Close()
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sourceCalls.Add(1)
				http.Redirect(w, r, target.URL, status)
			}))
			defer source.Close()
			endpoint, err := url.Parse(source.URL)
			if err != nil {
				t.Fatal(err)
			}
			port, err := strconv.Atoi(endpoint.Port())
			if err != nil {
				t.Fatal(err)
			}
			h := &Handler{serverPort: port}
			err = h.deliverMessage("private webhook payload", "test-only-internal-token")
			var responseErr *webhookDeliveryStatusError
			if !errors.As(err, &responseErr) || responseErr.status != status || sourceCalls.Load() != 1 || targetCalls.Load() != 0 {
				t.Fatalf("status=%d err=%v source=%d redirected=%d token_forwarded=%v", status, err, sourceCalls.Load(), targetCalls.Load(), forwardedToken.Load())
			}
		})
	}
}
