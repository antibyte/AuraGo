package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"

	"github.com/bwmarrin/discordgo"
)

const discordRequestBudget = 30 * time.Second

type discordTransport struct {
	base       http.RoundTripper
	generation *discordGeneration
	mu         sync.Mutex
	retryAt    time.Time
}

func configureDiscordHTTP(s *discordgo.Session, g *discordGeneration) {
	base := s.Client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	client := *s.Client
	client.Timeout = discordRequestBudget
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client.Transport = &discordTransport{base: base, generation: g}
	s.Client = &client
	s.MaxRestRetries = 0
	s.ShouldRetryOnRateLimit = false
	s.ShouldReconnectOnError = false
}

func (t *discordTransport) wait(ctx context.Context) error {
	for {
		t.mu.Lock()
		delay := time.Until(t.retryAt)
		t.mu.Unlock()
		if delay <= 0 {
			return ctx.Err()
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func discordRetryDelay(header string, body []byte) time.Duration {
	var payload struct {
		RetryAfter *float64 `json:"retry_after"`
	}
	_ = json.Unmarshal(body, &payload)
	seconds := -1.0
	if payload.RetryAfter != nil {
		seconds = *payload.RetryAfter
	}
	if parsed, err := strconv.ParseFloat(header, 64); err == nil && parsed > seconds {
		seconds = parsed
	}
	if deadline, err := http.ParseTime(header); err == nil && time.Until(deadline).Seconds() > seconds {
		seconds = time.Until(deadline).Seconds()
	}
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds >= float64(math.MaxInt64)/float64(time.Second) {
		return time.Duration(math.MaxInt64)
	}
	if seconds < 0 {
		return time.Second
	}
	return time.Duration(seconds * float64(time.Second))
}

type discordResponseBody struct {
	io.ReadCloser
	finish func()
	once   sync.Once
}

func (b *discordResponseBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.finish)
	return err
}

func (t *discordTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.Host != "discord.com" || req.URL.User != nil {
		return nil, fmt.Errorf("Discord API request outside the configured origin")
	}
	g := t.generation
	if !g.begin() {
		return nil, context.Canceled
	}
	ctx, cancel := context.WithTimeout(req.Context(), discordRequestBudget)
	stop := context.AfterFunc(g.ctx, cancel)
	finish := func() { stop(); cancel(); g.workers.Done() }
	handedOff := false
	defer func() {
		if !handedOff {
			finish()
		}
	}()
	for attempt := 0; attempt < 3; attempt++ {
		if err := t.wait(ctx); err != nil {
			return nil, err
		}
		next := req.Clone(ctx)
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			next.Body = body
		}
		res, err := t.base.RoundTrip(next)
		if err != nil {
			return nil, err
		}
		if res == nil {
			return nil, fmt.Errorf("Discord transport returned no response")
		}
		var errorBody []byte
		if res.StatusCode >= 300 {
			body, err := io.ReadAll(io.LimitReader(res.Body, 64<<10))
			res.Body.Close()
			if err != nil {
				return nil, err
			}
			errorBody = []byte(security.Scrub(string(body)))
			res.Body = io.NopCloser(bytes.NewReader(errorBody))
		}
		if res.StatusCode == http.StatusTooManyRequests {
			retryAt := time.Now().Add(discordRetryDelay(res.Header.Get("Retry-After"), errorBody))
			t.mu.Lock()
			if retryAt.After(t.retryAt) {
				t.retryAt = retryAt
			}
			t.mu.Unlock()
			if attempt < 2 && (req.Body == nil || req.GetBody != nil) {
				res.Body.Close()
				continue
			}
		}
		// SDK bucket sleeps are not cancellable. This transport owns rate-limit
		// waits; do not let the SDK retain another independent waiting schedule.
		for key := range res.Header {
			if strings.HasPrefix(strings.ToLower(key), "x-ratelimit-") || strings.EqualFold(key, "Retry-After") {
				res.Header.Del(key)
			}
		}
		res.Body = &discordResponseBody{ReadCloser: res.Body, finish: finish}
		handedOff = true
		return res, nil
	}
	return nil, fmt.Errorf("Discord retry budget exhausted")
}
