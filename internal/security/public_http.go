package security

import (
	"fmt"
	"net/http"
	"time"
)

// NewPublicHTTPClient is for credential-free transfers. Every redirect receives
// a fresh strict DNS pin; configured LAN permissions never apply to downloads.
func NewPublicHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: publicHTTPTransport{}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("public transfer redirect limit exceeded")
		}
		req.Header.Del("Referer")
		return nil
	}}
}

type publicHTTPTransport struct{}

func (publicHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	client, err := NewStrictPublicHTTPClientForURLContext(req.Context(), req.URL.String(), 0)
	if err != nil {
		return nil, err
	}
	response, err := client.Transport.RoundTrip(req)
	if err != nil {
		client.CloseIdleConnections()
		return nil, err
	}
	response.Body = &contextBody{ReadCloser: response.Body, close: client.CloseIdleConnections}
	return response, nil
}
