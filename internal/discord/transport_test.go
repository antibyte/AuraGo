package discord

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/security"
	"github.com/bwmarrin/discordgo"
)

type discordRoundTripper func(*http.Request) (*http.Response, error)

func (f discordRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func discordHTTPFixture(t *testing.T, roundTrip discordRoundTripper) (*discordgo.Session, *discordGeneration) {
	t.Helper()
	s, err := discordgo.New("Bot fixture-discord-token")
	if err != nil {
		t.Fatal(err)
	}
	s.Client = &http.Client{Transport: roundTrip}
	g := newDiscordGeneration(s)
	configureDiscordHTTP(s, g)
	t.Cleanup(g.stop)
	return s, g
}

func discordResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}
}

func TestDiscordPostRetryPolicyAndReplay(t *testing.T) {
	security.RegisterSensitive("fixture-discord-token")
	for _, tc := range []struct {
		name     string
		statuses []int
		want     int
		success  bool
	}{
		{"two 429 retries then success", []int{429, 429, 200}, 3, true},
		{"429 exhaustion", []int{429, 429, 429}, 3, false},
		{"502 is not replayed", []int{502, 200}, 1, false},
		{"redirect is not followed", []int{302, 200}, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempts := 0
			var firstBody string
			s, _ := discordHTTPFixture(t, func(r *http.Request) (*http.Response, error) {
				if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) > discordRequestBudget {
					t.Error("request lacks overall budget")
				}
				body, _ := io.ReadAll(r.Body)
				if attempts == 0 {
					firstBody = string(body)
				} else if string(body) != firstBody {
					t.Error("retried POST body changed")
				}
				status := tc.statuses[min(attempts, len(tc.statuses)-1)]
				attempts++
				res := discordResponse(status, `{"retry_after":0.001,"message":"fixture-discord-token","id":"message"}`)
				res.Header.Set("Retry-After", "0.001")
				res.Header.Set("X-RateLimit-Reset-After", "60")
				res.Header.Set("X-RateLimit-Remaining", "0")
				res.Header.Set("Location", "https://foreign.example/collect")
				return res, nil
			})
			_, err := s.ChannelMessageSendComplex("channel", agentTextMessage("answer"))
			if attempts != tc.want || (err == nil) != tc.success {
				t.Fatalf("attempts=%d err=%v", attempts, err)
			}
			if err != nil && strings.Contains(err.Error(), "fixture-discord-token") {
				t.Fatal("provider error exposed credential")
			}
		})
	}
}

func TestDiscordRateLimitWaitHonorsCancellationAndOverallBudget(t *testing.T) {
	for _, stopGeneration := range []bool{false, true} {
		t.Run(map[bool]string{false: "deadline", true: "generation stop"}[stopGeneration], func(t *testing.T) {
			first := make(chan struct{})
			var attempts atomic.Int32
			s, g := discordHTTPFixture(t, func(r *http.Request) (*http.Response, error) {
				if attempts.Add(1) == 1 {
					close(first)
				}
				res := discordResponse(429, `{"retry_after":0.001}`)
				res.Header.Set("Retry-After", "60")
				return res, nil
			})
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- sendMessage(ctx, s, "channel", "answer", slog.New(slog.NewTextHandler(io.Discard, nil)))
			}()
			<-first
			if stopGeneration {
				g.stop()
			}
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("wait did not cancel: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("rate limit wait blocked shutdown")
			}
			if attempts.Load() != 1 {
				t.Fatal("retried before Retry-After")
			}
			transport := s.Client.Transport.(*discordTransport)
			transport.mu.Lock()
			remaining := time.Until(transport.retryAt)
			transport.mu.Unlock()
			if remaining < 59*time.Second {
				t.Fatal("server Retry-After was shortened to the request budget")
			}
		})
	}
}

func TestDiscordSDKDoesNotKeepUncancellableBucketWaits(t *testing.T) {
	s, _ := discordHTTPFixture(t, func(*http.Request) (*http.Response, error) {
		res := discordResponse(200, `{"id":"message"}`)
		res.Header.Set("X-RateLimit-Remaining", "0")
		res.Header.Set("X-RateLimit-Reset-After", "60")
		return res, nil
	})
	done := make(chan error, 1)
	go func() {
		for range 2 {
			if _, err := s.ChannelMessageSendComplex("channel", agentTextMessage("answer")); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("SDK retained an uncancellable bucket sleep")
	}
}

func TestDiscordTransportRejectsForeignOrigin(t *testing.T) {
	var calls atomic.Int32
	s, _ := discordHTTPFixture(t, func(*http.Request) (*http.Response, error) { calls.Add(1); return discordResponse(200, "{}"), nil })
	for _, target := range []string{"https://foreign.example/api", "http://discord.com/api", "https://user@discord.com/api"} {
		req, _ := http.NewRequest(http.MethodPost, target, strings.NewReader("payload"))
		req.Header.Set("Authorization", "Bot fixture-discord-token")
		if res, err := s.Client.Do(req); err == nil {
			res.Body.Close()
			t.Fatal("foreign credential target accepted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("foreign target received request")
	}
}
