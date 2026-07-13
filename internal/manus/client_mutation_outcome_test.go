package manus

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestClientClassifiesUncertainMutationOutcomes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		transport http.RoundTripper
	}{
		{
			name: "transport error",
			transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("connection reset")
			}),
		},
		{
			name: "response read error",
			transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(errorReader{}), Header: make(http.Header)}, nil
			}),
		},
		{
			name: "server error",
			transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return responseWithBody(http.StatusInternalServerError, `{"ok":false,"error":{"code":"internal","message":"failed"}}`), nil
			}),
		},
		{
			name: "invalid success response",
			transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return responseWithBody(http.StatusOK, `{"ok":true`), nil
			}),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewClient("secret", ClientConfig{BaseURL: "https://api.manus.test", HTTPClient: &http.Client{Transport: tc.transport}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.SendMessage(context.Background(), SendMessageRequest{TaskID: "task-1", Content: "continue"})
			var unknown *OutcomeUnknownError
			if !errors.As(err, &unknown) || unknown.Operation != "/v2/task.sendMessage" {
				t.Fatalf("SendMessage() error = %#v", err)
			}
		})
	}
}

func TestClientDoesNotClassifyRejectedMutationAsUnknown(t *testing.T) {
	t.Parallel()

	client, err := NewClient("secret", ClientConfig{
		BaseURL: "https://api.manus.test",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return responseWithBody(http.StatusBadRequest, `{"ok":false,"error":{"code":"invalid","message":"bad request"}}`), nil
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.SendMessage(context.Background(), SendMessageRequest{TaskID: "task-1", Content: "continue"})
	var unknown *OutcomeUnknownError
	if err == nil || errors.As(err, &unknown) {
		t.Fatalf("SendMessage() error = %#v", err)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func responseWithBody(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
