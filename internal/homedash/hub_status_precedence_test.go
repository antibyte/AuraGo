package homedash_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

// gatedSource is a source whose Run reports a scripted status once runGate closes and whose Catalog
// fails once catGate closes (a nil gate does not wait).
type gatedSource struct {
	*sourcetest.FakeSource
	runGate, catGate chan struct{}
	status           homedash.SourceStatus
}

func (g *gatedSource) Run(ctx context.Context, sink homedash.Sink) error {
	select {
	case <-g.runGate:
	case <-ctx.Done():
		return nil
	}
	sink.Status(g.status)
	<-ctx.Done()
	return nil
}

func (g *gatedSource) Catalog(ctx context.Context) ([]homedash.Entity, error) {
	if g.catGate != nil {
		select {
		case <-g.catGate:
		case <-ctx.Done():
		}
	}
	return nil, errors.New("catalog unreachable")
}

func newGatedHub(t *testing.T, src *gatedSource) *homedash.Hub {
	t.Helper()
	h := homedash.NewHub(context.Background(), homedash.Options{IdleStop: time.Second, CatalogTTL: time.Minute, CatalogTimeout: 2 * time.Second})
	t.Cleanup(h.Close)
	h.Register("fa", func(homedash.RuntimeConfig) (homedash.Source, bool) { return src, true })
	h.Reconfigure(homedash.RuntimeConfig{Enabled: true, Sources: map[homedash.SourceID]homedash.SourceSettings{"fa": {Enabled: true}}})
	sourcetest.WaitFor(t, time.Second, func() bool { return h.Sources()[0].State != homedash.StatusDisabled }, "configured")
	return h
}

func statusIs(h *homedash.Hub, state, reason string) func() bool {
	return func() bool {
		st := h.Sources()[0]
		return st.State == state && st.Reason == reason
	}
}

func TestHubRunReasonSurvivesCatalogFailureEitherOrder(t *testing.T) {
	for _, reason := range []string{"auth_failed", "invalid_url"} {
		for _, order := range []string{"status first", "catalog first"} {
			t.Run(reason+"/"+order, func(t *testing.T) {
				src := &gatedSource{
					FakeSource: sourcetest.New("fa"),
					runGate:    make(chan struct{}),
					catGate:    make(chan struct{}),
					status:     homedash.SourceStatus{Source: "fa", State: homedash.StatusUnavailable, Reason: reason},
				}
				h := newGatedHub(t, src)
				sub, _ := h.Attach(session("s"))
				sub.Set(ids("fa:x"))
				done := make(chan struct{})
				go func() {
					h.Entities(context.Background(), homedash.Filter{})
					close(done)
				}()
				if order == "status first" {
					close(src.runGate)
					sourcetest.WaitFor(t, 2*time.Second, statusIs(h, homedash.StatusUnavailable, reason), "run status")
					close(src.catGate)
					<-done
				} else {
					close(src.catGate)
					<-done
					sourcetest.WaitFor(t, 2*time.Second, statusIs(h, homedash.StatusUnavailable, homedash.ReasonCatalogFailed), "catalog failed")
					close(src.runGate)
					sourcetest.WaitFor(t, 2*time.Second, statusIs(h, homedash.StatusUnavailable, reason), "run status replaces catalog_failed")
				}
				time.Sleep(50 * time.Millisecond)
				if st := h.Sources()[0]; st.State != homedash.StatusUnavailable || st.Reason != reason {
					t.Fatalf("final status %+v, want unavailable/%s", st, reason)
				}
			})
		}
	}
}

func TestHubStoppedSourceWithFailingCatalogStillReportsCatalogFailed(t *testing.T) {
	src := &gatedSource{FakeSource: sourcetest.New("fa"), runGate: make(chan struct{})}
	h := newGatedHub(t, src)
	h.Entities(context.Background(), homedash.Filter{})
	if st := h.Sources()[0]; st.State != homedash.StatusUnavailable || st.Reason != homedash.ReasonCatalogFailed {
		t.Fatalf("status %+v, want unavailable/catalog_failed", st)
	}
}
