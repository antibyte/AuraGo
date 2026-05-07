package desktop

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

func TestLooperRunStateHolderTryStartAllowsOneConcurrentRun(t *testing.T) {
	t.Parallel()

	holder := NewLooperRunStateHolder()
	start := make(chan struct{})
	var wg sync.WaitGroup
	var successes int32

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, cancel := context.WithCancel(context.Background())
			if err := holder.TryStart(3, cancel); err != nil {
				cancel()
				return
			}
			atomic.AddInt32(&successes, 1)
		}()
	}

	close(start)
	wg.Wait()
	if successes != 1 {
		t.Fatalf("successful TryStart calls = %d, want 1", successes)
	}
	if state := holder.State(); !state.Running || state.MaxIterations != 3 {
		t.Fatalf("state after TryStart = %+v, want running with max iterations", state)
	}
	holder.SetIdle()
	if state := holder.State(); state.Running {
		t.Fatalf("state after SetIdle = %+v, want idle", state)
	}
}
