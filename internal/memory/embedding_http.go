package memory

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/httporigin"

	chromem "github.com/philippgille/chromem-go"
)

const (
	embeddingRequestTimeout   = 120 * time.Second
	maxEmbeddingResponseBytes = 16 << 20
)

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// newOpenAICompatEmbeddingFunc creates a bounded, origin-pinned OpenAI-compatible
// embedding request without exposing endpoint URLs or provider response bodies
// through retry logs.
func newOpenAICompatEmbeddingFunc(baseURL, apiKey, model string) chromem.EmbeddingFunc {
	return newOpenAICompatEmbeddingFuncWithClient(baseURL, apiKey, model, newEmbeddingHTTPClient(baseURL))
}

func newOpenAICompatEmbeddingFuncWithClient(baseURL, apiKey, model string, client *http.Client) chromem.EmbeddingFunc {
	endpoint, err := openAIEmbeddingEndpoint(baseURL)
	if err != nil {
		return func(context.Context, string) ([]float32, error) {
			return nil, err
		}
	}
	return func(ctx context.Context, text string) ([]float32, error) {
		if ctx == nil {
			ctx = context.Background()
		}
		requestCtx, cancel := context.WithTimeout(ctx, embeddingRequestTimeout)
		defer cancel()

		payload, err := json.Marshal(struct {
			Input string `json:"input"`
			Model string `json:"model"`
		}{Input: text, Model: model})
		if err != nil {
			return nil, fmt.Errorf("marshal embedding request: %w", err)
		}
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, errors.New("create embedding request: invalid endpoint")
		}
		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, &embeddingTransportError{cause: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, &embeddingHTTPStatusError{statusCode: resp.StatusCode}
		}
		body, err := readEmbeddingResponseBody(resp.Body)
		if err != nil {
			return nil, &embeddingTransportError{cause: err}
		}

		var result openAIEmbeddingResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("decode embedding response: %w", err)
		}
		if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
			return nil, errors.New("embedding response contained no vectors")
		}
		return normalizeEmbedding(result.Data[0].Embedding), nil
	}
}

func readEmbeddingResponseBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxEmbeddingResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxEmbeddingResponseBytes {
		return nil, errors.New("embedding response exceeded the size limit")
	}
	return data, nil
}

func openAIEmbeddingEndpoint(baseURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", errors.New("invalid embedding endpoint")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/embeddings"
	u.RawPath = ""
	u.Fragment = ""
	return u.String(), nil
}

// redactEmbeddingURL deliberately logs only the scheme and authority. Query
// parameters on compatible-provider URLs can contain API credentials.
func redactEmbeddingURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || u.Scheme == "" || u.Host == "" {
		return "invalid embedding URL"
	}
	return u.Scheme + "://" + u.Host
}

func newEmbeddingHTTPClient(baseURL string) *http.Client {
	return newEmbeddingHTTPClientWithTransport(baseURL, nil)
}

// newEmbeddingHTTPClientWithTransport keeps a transport seam for fixture tests.
// Production clients otherwise preserve httporigin's default proxy and TLS
// behavior; only loopback HTTPS receives the same local certificate exception
// used by the LLM client.
func newEmbeddingHTTPClientWithTransport(baseURL string, transport *http.Transport) *http.Client {
	client := httporigin.NewClient(embeddingRequestTimeout)
	switch {
	case isLoopbackEmbeddingHTTPS(baseURL):
		client.Transport = loopbackEmbeddingHTTPSTransport()
	case transport != nil:
		client.Transport = transport
	}
	return client
}

func isLoopbackEmbeddingHTTPS(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || (net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback())
}

// loopbackEmbeddingHTTPSTransport permits self-signed certificates only when
// the configured endpoint is loopback. Its dial guard prevents a redirect or
// hostname resolution from widening that exception to a remote host.
func loopbackEmbeddingHTTPSTransport() *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if strings.EqualFold(host, "localhost") {
				host = "127.0.0.1"
			}
			ip := net.ParseIP(host)
			if ip == nil || !ip.IsLoopback() {
				return nil, errors.New("non-loopback TLS target rejected")
			}
			return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		},
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // #nosec G402 — loopback dial guard only
		ForceAttemptHTTP2: false,
	}
}
