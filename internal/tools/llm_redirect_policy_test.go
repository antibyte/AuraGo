package tools

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/httporigin"
)

// Same-origin redirect policy: the summary and transcription clients carry the provider
// key plus page content or audio, so they must not follow a redirect off the
// configured origin. Server A is the configured provider and answers with a 307
// to server B on another loopback port; B must never be reached.

type crossOriginRedirectFixture struct {
	providerURL string
	secondHits  atomic.Int32
}

func newCrossOriginRedirectFixture(t *testing.T) *crossOriginRedirectFixture {
	t.Helper()
	f := &crossOriginRedirectFixture{}
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.secondHits.Add(1)
	}))
	t.Cleanup(second.Close)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", second.URL+r.URL.Path)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	t.Cleanup(provider.Close)
	f.providerURL = provider.URL
	return f
}

func (f *crossOriginRedirectFixture) assertRejected(t *testing.T, err error) {
	t.Helper()
	if hits := f.secondHits.Load(); hits != 0 {
		t.Fatalf("second origin received %d request(s)", hits)
	}
	if !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
		t.Fatalf("error = %v, want httporigin.ErrCrossOriginRedirect", err)
	}
}

func TestSummariseContentDoesNotFollowCrossOriginRedirect(t *testing.T) {
	f := newCrossOriginRedirectFixture(t)
	_, err := SummariseContent(context.Background(), SummaryLLMConfig{
		APIKey:  "sk-summary",
		BaseURL: f.providerURL + "/v1",
		Model:   "m",
	}, nil, "SECRET-PAGE-CONTENT", "query", "web page")
	f.assertRejected(t, err)
}

func TestWhisperTranscriptionDoesNotFollowCrossOriginRedirect(t *testing.T) {
	f := newCrossOriginRedirectFixture(t)
	cfg := &config.Config{}
	cfg.Whisper.BaseURL = f.providerURL + "/v1"
	cfg.Whisper.APIKey = "sk-whisper"
	cfg.Whisper.Model = "whisper-1"
	_, _, err := TranscribeAudio(context.Background(), "call.wav", []byte("RIFF-secret-audio"), cfg)
	f.assertRejected(t, err)
}

func TestMultimodalTranscriptionDoesNotFollowCrossOriginRedirect(t *testing.T) {
	f := newCrossOriginRedirectFixture(t)
	cfg := &config.Config{}
	cfg.Whisper.Mode = "multimodal"
	cfg.Whisper.BaseURL = f.providerURL + "/v1"
	cfg.Whisper.APIKey = "sk-whisper"
	cfg.Whisper.Model = "test-model"
	_, _, err := TranscribeAudio(context.Background(), "call.wav", []byte("RIFF-secret-audio"), cfg)
	f.assertRejected(t, err)
}
