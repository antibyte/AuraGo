package embeddings

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeLlamaServer drives embedWithRestarts without a real llama-server.
type fakeLlamaServer struct {
	up       bool
	starts   int
	stops    int
	embeds   int
	failures int // remaining embed attempts that fail with a backend error
	onEmbed  func() error
	onStart  func() error
}

func (server *fakeLlamaServer) ops() llamaServerOps {
	return llamaServerOps{
		name:    "fake-server",
		running: func() bool { return server.up },
		start: func(context.Context) error {
			server.starts++
			if server.onStart != nil {
				return server.onStart()
			}
			server.up = true
			return nil
		},
		embed: func(context.Context, []string) ([][]float32, error) {
			server.embeds++
			if server.onEmbed != nil {
				return nil, server.onEmbed()
			}
			if server.failures > 0 {
				server.failures--
				return nil, errors.New("connection refused")
			}
			return [][]float32{unitTestVector()}, nil
		},
		stop: func() {
			server.stops++
			server.up = false
		},
	}
}

// The old lifetime counter made the third restart of a process permanent. With
// a sliding window the embedder recovers once the window has passed.
func TestLlamaRestartBudgetIsATimeWindowNotALifetimeCap(t *testing.T) {
	now := time.Unix(2_000_000, 0)
	budget := newRestartBudgetWithClock(maxProcessRestarts, processRestartWindow, func() time.Time { return now })
	server := &fakeLlamaServer{failures: 3}
	texts := []string{"text"}

	// Call 1 burns two restarts, call 2 the third and then hits the limit.
	if _, err := embedWithRestarts(context.Background(), budget, server.ops(), texts); err == nil {
		t.Fatal("call 1 unexpectedly succeeded")
	}
	_, err := embedWithRestarts(context.Background(), budget, server.ops(), texts)
	if err == nil || !strings.Contains(err.Error(), "restart limit reached") {
		t.Fatalf("call 2 error = %v, want the restart limit inside the window", err)
	}

	now = now.Add(processRestartWindow + time.Minute)
	vectors, err := embedWithRestarts(context.Background(), budget, server.ops(), texts)
	if err != nil || len(vectors) != 1 {
		t.Fatalf("call after the window = %v, %v; want success", vectors, err)
	}
}

// A caller that gives up is not a server failure: no kill, no restart, and no
// budget spent.
func TestLlamaCallerContextEndLeavesServerAndBudgetAlone(t *testing.T) {
	newBudget := func() *restartBudget {
		return newRestartBudget(maxProcessRestarts, processRestartWindow)
	}

	t.Run("already canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		budget := newBudget()
		server := &fakeLlamaServer{} // down: a restart would be due
		_, err := embedWithRestarts(ctx, budget, server.ops(), []string{"text"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
		if server.starts+server.stops+server.embeds != 0 || budget.remaining() != maxProcessRestarts {
			t.Fatalf("canceled call touched the server: %+v, budget %d", server, budget.remaining())
		}
	})

	t.Run("canceled during embed", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		budget := newBudget()
		server := &fakeLlamaServer{up: true}
		server.onEmbed = func() error {
			cancel()
			return errors.Join(errors.New("call llama-server"), ctx.Err())
		}
		_, err := embedWithRestarts(ctx, budget, server.ops(), []string{"text"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
		if server.stops != 0 || server.starts != 0 || !server.up || budget.remaining() != maxProcessRestarts {
			t.Fatalf("cancel killed or restarted the server: %+v, budget %d", server, budget.remaining())
		}
	})

	t.Run("canceled during restart", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		budget := newBudget()
		server := &fakeLlamaServer{}
		server.onStart = func() error {
			cancel()
			return ctx.Err()
		}
		_, err := embedWithRestarts(ctx, budget, server.ops(), []string{"text"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
		if server.embeds != 0 || budget.remaining() != maxProcessRestarts {
			t.Fatalf("interrupted restart was charged: embeds=%d budget=%d", server.embeds, budget.remaining())
		}
	})

	t.Run("embedders do not restart for a canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		native := &llamaEmbedder{serverPath: "does-not-exist"}
		if _, err := native.Embed(ctx, []string{"text"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("native embedder error = %v, want context.Canceled", err)
		}
		if native.restarts.remaining() != maxProcessRestarts {
			t.Fatalf("native embedder spent budget: %d", native.restarts.remaining())
		}
		docker := &dockerLlamaEmbedder{needsRestart: true}
		if _, err := docker.Embed(ctx, []string{"text"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("docker embedder error = %v, want context.Canceled", err)
		}
		if docker.restarts.remaining() != maxProcessRestarts {
			t.Fatalf("docker embedder spent budget: %d", docker.restarts.remaining())
		}
	})
}
