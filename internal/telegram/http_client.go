package telegram

import (
	"context"
	"io"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// contextBoundHTTPClient joins Telegram requests to both the SDK request
// context and the owning server context without changing HTTP transport policy.
type contextBoundHTTPClient struct {
	parent context.Context
	client tgbotapi.HTTPClient
}

func (c contextBoundHTTPClient) Do(req *http.Request) (*http.Response, error) {
	parent := c.parent
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(req.Context())
	stopParent := context.AfterFunc(parent, cancel)
	if parent.Err() != nil {
		cancel()
	}

	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil {
		stopParent()
		cancel()
		return resp, err
	}
	if resp == nil || resp.Body == nil {
		stopParent()
		cancel()
		return resp, nil
	}
	resp.Body = &contextBoundResponseBody{
		ReadCloser: resp.Body,
		cancel:     cancel,
		stopParent: stopParent,
	}
	return resp, nil
}

type contextBoundResponseBody struct {
	io.ReadCloser
	cancel     context.CancelFunc
	stopParent func() bool
}

func (b *contextBoundResponseBody) Close() error {
	b.stopParent()
	b.cancel()
	return b.ReadCloser.Close()
}
