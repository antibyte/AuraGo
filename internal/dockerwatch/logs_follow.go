package dockerwatch

import (
	"context"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// logEnv is everything a log follower takes from the outside. The hub fills it
// (Hub.logEnv); the tests replace the Engine calls, the event feed, the stream
// budget and the timing with fakes.
type logEnv struct {
	hosts   Hosts
	acquire func(host string) (release func(), err error)
	watch   func(host string, fn func(Event)) (cancel func())
	follow  func(ctx context.Context, c *dockerutil.Client, ref string, opt dockerhost.LogOptions, fn func(dockerhost.LogLine) error) error
	list    func(ctx context.Context, c *dockerutil.Client) ([]dockerhost.ContainerSummary, error)
	after   func(d time.Duration, f func()) logTimer

	flushEvery     time.Duration                                 // a batch leaves this long after its first line ...
	flushLines     int                                           // ... or when it has this many lines
	settle         time.Duration                                 // pause before a project re-checks its containers after a stream ended
	historyTimeout time.Duration                                 // a history read of a project follower is given up after this long
	acquireProject func(host string) (release func(), err error) // a project or stack follow takes one of the host's project follows; nil means no cap
}

// logEnv returns the environment of the hub's followers.
func (h *Hub) logEnv() logEnv {
	return logEnv{
		hosts:          h.hosts,
		acquire:        h.Acquire,
		watch:          h.WatchHost,
		follow:         dockerhost.FollowLogs,
		list:           dockerhost.ListContainers,
		after:          logAfterFunc,
		flushEvery:     logFlushInterval,
		flushLines:     logFlushLines,
		settle:         logSettle,
		historyTimeout: logHistoryTimeout,
		acquireProject: h.acquireProjectFollow,
	}
}

func (e logEnv) batcher(out func(LogBatch) error, notice func(Notice) error, onFail func(error)) *logBatcher {
	return newLogBatcher(out, notice, onFail, e.flushLines, e.flushEvery, e.after)
}

// FollowContainer follows one container's log: the last opt.Tail lines first,
// then the new ones until the container's log ends. Lines reach out in batches
// of at most 200, at the latest 100 ms after the first of a batch; out is never
// called concurrently. It takes one stream slot of the host (ErrTooManyStreams
// when none is free). It returns nil when the log ends, ctx.Err() when ctx
// ended, the error of out (or notice) when the receiver failed, and otherwise
// the Engine or transport error unchanged (the server classifies it); a
// *dockerhost.StatusError carries the Engine's status. notice is reserved for
// the project follower and is not called for a single container.
func (h *Hub) FollowContainer(ctx context.Context, host, ref string, opt FollowOptions, out func(LogBatch) error, notice func(Notice) error) error {
	return h.logEnv().followContainer(ctx, host, ref, opt, out, notice)
}

func (e logEnv) followContainer(ctx context.Context, host, ref string, opt FollowOptions, out func(LogBatch) error, notice func(Notice) error) error {
	release, err := e.acquire(host)
	if err != nil {
		return err
	}
	defer release()
	c, err := e.hosts.Client(ctx, host)
	if err != nil {
		return err
	}
	inner, cancel := context.WithCancel(ctx)
	defer cancel()
	b := e.batcher(out, notice, func(error) { cancel() })
	err = e.follow(inner, c, ref, dockerhost.LogOptions{Tail: opt.tail(), Since: opt.Since, Follow: true, Opened: opt.Opened}, func(l dockerhost.LogLine) error {
		return b.add(logEntryOf(l))
	})
	flushErr := b.close()
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case b.failure() != nil:
		return b.failure()
	case err != nil:
		return err
	}
	return flushErr
}
