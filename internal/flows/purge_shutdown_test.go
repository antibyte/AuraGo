package flows

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCredentialPurgeShutdownDrainsEveryStep(t *testing.T) {
	for _, step := range []PurgeStep{PurgeStepBackup, PurgeStepVersions, PurgeStepRedact, PurgeStepCompact, PurgeStepSnapshots, PurgeStepVerify} {
		t.Run(string(step), func(t *testing.T) {
			svc, store := newPurgeService(t)
			seedCredentialStore(t, store)
			entered, resume := make(chan struct{}), make(chan struct{})
			released := false
			t.Cleanup(func() {
				if !released {
					close(resume)
				}
			})
			svc.purge.stepHook = func(at PurgeStep) {
				if at == step {
					close(entered)
					<-resume
				}
			}
			if _, err := svc.StartCredentialPurge(nil); err != nil {
				t.Fatal(err)
			}
			select {
			case <-entered:
			case <-time.After(30 * time.Second):
				t.Fatalf("purge did not reach step: %+v", svc.CredentialPurgeState())
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			if err := svc.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("shutdown returned before the blocked purge drained: %v", err)
			}
			if err := store.db.Ping(); err != nil {
				t.Fatalf("store closed under purge: %v", err)
			}
			close(resume)
			released = true
			if err := svc.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
			state := svc.CredentialPurgeState()
			if state.Status != "failed" || state.FailedStep != step || !strings.Contains(state.Error, "canceled") {
				t.Fatalf("purge did not stop at canceled step: %+v", state)
			}
			svc.cfg.PurgeKey = nil
			if _, err := svc.StartCredentialPurge(nil); !errors.Is(err, ErrRunnerClosed) {
				t.Fatalf("post-shutdown admission touched preflight: %v", err)
			}
		})
	}
}
