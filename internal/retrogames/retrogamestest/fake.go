// Package retrogamestest provides fakes for tests outside internal/retrogames.
package retrogamestest

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	"aurago/internal/retrogames"
)

// Fetcher serves fixed items per platform. Platforms without items get one
// filler game so a sync succeeds; Err makes every call fail.
type Fetcher struct {
	mu       sync.Mutex
	BySource map[string][]retrogames.Item
	Err      error
	Calls    int
}

// FetchPage implements retrogames.Fetcher.
func (f *Fetcher) FetchPage(ctx context.Context, query, cursor string) (retrogames.Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls++
	if err := ctx.Err(); err != nil {
		return retrogames.Page{}, err
	}
	if f.Err != nil {
		return retrogames.Page{}, f.Err
	}
	for _, src := range retrogames.Sources() {
		if src.Query() != query {
			continue
		}
		if items, ok := f.BySource[src.Key]; ok {
			return retrogames.Page{Items: append([]retrogames.Item(nil), items...)}, nil
		}
		return retrogames.Page{Items: []retrogames.Item{Item(src.Key+"_filler", "Filler "+src.Label, "dosbox", 1990, 1)}}, nil
	}
	return retrogames.Page{}, errors.New("retrogamestest: unknown query")
}

// Item builds a scrape item.
func Item(id, title, emulator string, year int, downloads int64) retrogames.Item {
	return retrogames.Item{
		Identifier: id,
		Title:      retrogames.FlexText{title},
		Year:       retrogames.FlexText{strconv.Itoa(year)},
		Emulator:   retrogames.FlexText{emulator},
		Downloads:  retrogames.FlexText{strconv.FormatInt(downloads, 10)},
	}
}

// NewService returns an enabled service on a temp dir that never sleeps and
// shuts down at test end.
func NewService(t testing.TB, f retrogames.Fetcher) *retrogames.Service {
	t.Helper()
	svc := retrogames.NewService(retrogames.Deps{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Fetcher: f,
		Sleep:   func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	})
	svc.Configure(retrogames.Settings{Enabled: true, AgentAccess: true, DataDir: t.TempDir()})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = svc.Shutdown(ctx)
	})
	return svc
}
