package retrogames

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	requestPause      = time.Second
	maxPagesPerSource = 20
	// maxItemsPerSource bounds the memory of one source sync. A normalized
	// Game costs up to ~4 KB (title, creator, system and tags are all
	// capped), so the cap is ~400 MB in the worst case; real sources hold
	// 20-30 MB.
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
	// ErrRateLimited stops a whole sync run: archive.org asked us to back off
	// for longer than a sync is willing to wait, or kept answering 429.
	ErrRateLimited = errors.New("retrogames: archive.org rate limit")
	errEmptySource = errors.New("retrogames: source returned no playable games")
	// errStore marks failures of the local index; they abort the whole run.
	errStore = errors.New("retrogames: index store failure")
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
	// acceptShrink lets a user-requested sync accept a shrink that the guard
	// already rejected once (the source's last stored error says so). A source
	// that was not rejected before stays guarded, so one truncated answer
	// cannot halve a healthy source. Automatic syncs never set it.
	acceptShrink bool
}

// recordFailure stores a failed attempt of one source and reports it. A
// failure to store is only logged: it must not hide the original error.
func (s *syncer) recordFailure(ctx context.Context, key, msg string) {
	s.logger.Warn("retro games source sync failed", "source", key, "error", msg)
	if recErr := s.store.RecordSyncFailure(ctx, key, s.now(), msg); recErr != nil {
		s.logger.Warn("retro games could not record sync failure", "source", key, "error", recErr)
	}
	s.progress(SourceProgress{Source: key, State: StateFailed, Err: msg})
}

// run syncs every source in order. A failing source is recorded and skipped;
// the returned error lists all failed sources. Cancellation aborts at once.
// A rate limit marks every remaining source failed without contacting
// archive.org again, and a failing local store aborts the run.
func (s *syncer) run(ctx context.Context, srcs []Source) error {
	var failed []string
	rateLimited := false
	for i, src := range srcs {
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
			s.recordFailure(ctx, src.Key, msg)
			if errors.Is(err, errStore) {
				return fmt.Errorf("retrogames: sync aborted at %s: %w", src.Key, err)
			}
			failed = append(failed, src.Key)
			if errors.Is(err, ErrRateLimited) {
				rateLimited = true
				for _, rest := range srcs[i+1:] {
					s.recordFailure(ctx, rest.Key, msg)
					failed = append(failed, rest.Key)
				}
				break
			}
			continue
		}
		s.progress(SourceProgress{Source: src.Key, State: StateDone, Fetched: n})
	}
	if len(failed) > 0 {
		if rateLimited {
			return fmt.Errorf("retrogames: sync failed for %s: %w", strings.Join(failed, ", "), ErrRateLimited)
		}
		return fmt.Errorf("retrogames: sync failed for %s", strings.Join(failed, ", "))
	}
	return nil
}

func (s *syncer) syncSource(ctx context.Context, src Source) (int, error) {
	states, err := s.store.SyncStates(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", errStore, err)
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
		if p.Cursor != "" && p.Cursor == cursor {
			return 0, fmt.Errorf("retrogames: archive.org repeated cursor for %s", src.Key)
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
		// run stores a rejection as err.Error(), which starts with the
		// ErrShrinkGuard text, so the stored message tells a repeat shrink.
		rejectedBefore := strings.Contains(states[src.Key].LastError, ErrShrinkGuard.Error())
		if !(s.acceptShrink && rejectedBefore) {
			return 0, fmt.Errorf("%w (%d now, %d before)", ErrShrinkGuard, len(games), previous)
		}
	}
	if err := s.store.ReplaceSource(ctx, src.Key, games, s.now()); err != nil {
		return 0, fmt.Errorf("%w: %w", errStore, err)
	}
	return len(games), nil
}

func (s *syncer) fetchPage(ctx context.Context, query, cursor string) (Page, error) {
	var lastErr error
	// A retry wait already spaces the requests; do not add the regular pause.
	skipPause := false
	for attempt := 0; attempt <= len(retryBackoff); attempt++ {
		if s.requests > 0 && !skipPause {
			if err := s.sleep(ctx, requestPause); err != nil {
				return Page{}, err
			}
		}
		skipPause = false
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
		if retry && wait > maxRetryWait {
			// The server wants a longer break than a sync waits for; retrying
			// now would only extend the ban.
			return Page{}, fmt.Errorf("%w: server asked to wait %s", ErrRateLimited, wait)
		}
		if !retry || attempt == len(retryBackoff) {
			break
		}
		if wait <= 0 {
			wait = retryBackoff[attempt]
		}
		if err := s.sleep(ctx, wait); err != nil {
			return Page{}, err
		}
		skipPause = true
	}
	var se *StatusError
	if errors.As(lastErr, &se) && se.Status == http.StatusTooManyRequests {
		return Page{}, fmt.Errorf("%w: %w", ErrRateLimited, lastErr)
	}
	return Page{}, lastErr
}
