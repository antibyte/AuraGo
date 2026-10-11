package retrogames

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	requestPause      = time.Second
	maxPagesPerSource = 20
	maxItemsPerSource = 100000
	maxRetryWait      = 60 * time.Second
)

// Source progress states.
const (
	StateIdle    = "idle"
	StatePending = "pending"
	StateRunning = "running"
	StateDone    = "done"
	StateFailed  = "failed"
)

var retryBackoff = []time.Duration{2 * time.Second, 5 * time.Second}

var (
	// ErrShrinkGuard keeps a source when archive.org suddenly returns far fewer games.
	ErrShrinkGuard = errors.New("retrogames: source shrank by more than half; kept the previous data")
	errEmptySource = errors.New("retrogames: source returned no playable games")
)

// SourceProgress is the live state of one source during a sync.
type SourceProgress struct {
	Source  string
	State   string
	Page    int
	Fetched int
	Err     string
}

type syncer struct {
	fetcher  Fetcher
	store    *Store
	now      func() time.Time
	sleep    func(context.Context, time.Duration) error
	logger   *slog.Logger
	progress func(SourceProgress)
	requests int
}

// run syncs every source in order. A failing source is recorded and skipped;
// the returned error lists all failed sources. Cancellation aborts at once.
func (s *syncer) run(ctx context.Context, srcs []Source) error {
	var failed []string
	for _, src := range srcs {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.progress(SourceProgress{Source: src.Key, State: StateRunning})
		n, err := s.syncSource(ctx, src)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			msg := err.Error()
			s.logger.Warn("retro games source sync failed", "source", src.Key, "error", msg)
			if recErr := s.store.RecordSyncFailure(ctx, src.Key, s.now(), msg); recErr != nil {
				s.logger.Warn("retro games could not record sync failure", "source", src.Key, "error", recErr)
			}
			s.progress(SourceProgress{Source: src.Key, State: StateFailed, Err: msg})
			failed = append(failed, src.Key)
			continue
		}
		s.progress(SourceProgress{Source: src.Key, State: StateDone, Fetched: n})
	}
	if len(failed) > 0 {
		return fmt.Errorf("retrogames: sync failed for %s", strings.Join(failed, ", "))
	}
	return nil
}

func (s *syncer) syncSource(ctx context.Context, src Source) (int, error) {
	states, err := s.store.SyncStates(ctx)
	if err != nil {
		return 0, err
	}
	previous := states[src.Key].ItemCount
	games := []Game{}
	seen := map[string]bool{}
	cursor := ""
	for page := 1; ; page++ {
		if page > maxPagesPerSource {
			return 0, fmt.Errorf("retrogames: source %s exceeded %d pages", src.Key, maxPagesPerSource)
		}
		p, err := s.fetchPage(ctx, src.Query(), cursor)
		if err != nil {
			return 0, err
		}
		for _, it := range p.Items {
			g, ok := normalizeItem(src, it)
			if !ok || seen[g.Identifier] {
				continue
			}
			seen[g.Identifier] = true
			games = append(games, g)
			if len(games) > maxItemsPerSource {
				return 0, fmt.Errorf("retrogames: source %s exceeded %d items", src.Key, maxItemsPerSource)
			}
		}
		s.progress(SourceProgress{Source: src.Key, State: StateRunning, Page: page, Fetched: len(games)})
		if p.Cursor == "" || len(p.Items) == 0 {
			break
		}
		cursor = p.Cursor
	}
	if len(games) == 0 {
		return 0, errEmptySource
	}
	if previous > 0 && len(games)*2 < previous {
		return 0, fmt.Errorf("%w (%d now, %d before)", ErrShrinkGuard, len(games), previous)
	}
	if err := s.store.ReplaceSource(ctx, src.Key, games, s.now()); err != nil {
		return 0, err
	}
	return len(games), nil
}

func (s *syncer) fetchPage(ctx context.Context, query, cursor string) (Page, error) {
	var lastErr error
	for attempt := 0; attempt <= len(retryBackoff); attempt++ {
		if s.requests > 0 {
			if err := s.sleep(ctx, requestPause); err != nil {
				return Page{}, err
			}
		}
		s.requests++
		page, err := s.fetcher.FetchPage(ctx, query, cursor)
		if err == nil {
			return page, nil
		}
		lastErr = err
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Page{}, ctxErr
		}
		retry, wait := retryable(err)
		if !retry || attempt == len(retryBackoff) {
			break
		}
		if wait <= 0 {
			wait = retryBackoff[attempt]
		}
		if wait > maxRetryWait {
			wait = maxRetryWait
		}
		if err := s.sleep(ctx, wait); err != nil {
			return Page{}, err
		}
	}
	return Page{}, lastErr
}
