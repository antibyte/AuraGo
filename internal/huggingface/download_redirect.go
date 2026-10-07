package huggingface

import (
	"fmt"
	"net/http"

	"aurago/internal/security"
)

// API redirects stay at their configured origin. Only file downloads may move
// to a different, freshly DNS-pinned public HTTPS origin, without credentials.
func (c *Client) downloadResponse(initial *http.Request) (*http.Response, error) {
	client := *c.httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	current := initial
	foreign := false
	for hop := 0; hop < 6; hop++ {
		response, err := client.Do(current)
		if err != nil {
			return nil, err
		}
		if response.StatusCode < 300 || response.StatusCode > 399 {
			return response, nil
		}
		next, err := response.Location()
		response.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("Hugging Face download redirect: %w", err)
		}
		request, err := http.NewRequestWithContext(initial.Context(), http.MethodGet, next.String(), nil)
		if err != nil {
			return nil, err
		}
		foreign = foreign || !security.SameHTTPOrigin(next, initial.URL)
		if foreign {
			if next.Scheme != "https" {
				return nil, fmt.Errorf("Hugging Face download redirect requires public HTTPS")
			}
			pinned, err := security.NewStrictPublicHTTPClientForURL(next.String(), client.Timeout)
			if err != nil {
				return nil, err
			}
			client = *pinned
			if transport, ok := client.Transport.(*http.Transport); ok {
				transport.DisableKeepAlives = true
			}
		} else {
			request.Header = initial.Header.Clone()
		}
		current = request
	}
	return nil, fmt.Errorf("too many Hugging Face download redirects")
}
