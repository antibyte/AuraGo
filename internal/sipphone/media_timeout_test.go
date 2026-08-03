package sipphone

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMediaPumpReportsRTPIdleTimeoutOnce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errorsSeen := make(chan error, 2)
	pump := &mediaPump{
		idleTimeout: 10 * time.Millisecond,
		onError:     func(err error) { errorsSeen <- err },
	}
	pump.lastReceived.Store(time.Now().UnixNano())
	go pump.watchRTPIdle(ctx)
	select {
	case err := <-errorsSeen:
		if !errors.Is(err, ErrMediaTimeout) {
			t.Fatalf("media idle error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RTP idle timeout was not reported")
	}
	select {
	case err := <-errorsSeen:
		t.Fatalf("RTP idle timeout was reported more than once: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
}
