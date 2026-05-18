package main

import (
	"log/slog"
	"sync"
	"testing"
	"time"

	"aurago/internal/remote"
)

func TestClientStopIsIdempotentUnderConcurrentCalls(t *testing.T) {
	client := &Client{
		done:   make(chan struct{}),
		logger: slog.Default(),
	}

	const goroutines = 64
	start := make(chan struct{})
	panicCh := make(chan interface{}, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			defer func() {
				panicCh <- recover()
			}()
			<-start
			client.Stop()
		}()
	}

	close(start)
	wg.Wait()
	close(panicCh)

	for recovered := range panicCh {
		if recovered != nil {
			t.Fatalf("Stop panicked under concurrent calls: %v", recovered)
		}
	}

	select {
	case <-client.done:
	default:
		t.Fatal("Stop did not close done channel")
	}
}

func TestHeartbeatLoopContinuesAfterTransientSendFailure(t *testing.T) {
	client := &Client{
		cfg:      clientConfig{DeviceID: "dev-1"},
		done:     make(chan struct{}),
		logger:   slog.Default(),
		executor: NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB),
	}

	finished := make(chan struct{})
	go func() {
		client.heartbeatLoopWithInterval(10 * time.Millisecond)
		close(finished)
	}()

	select {
	case <-finished:
		t.Fatal("heartbeat loop exited after transient send failure")
	case <-time.After(50 * time.Millisecond):
	}

	client.Stop()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("heartbeat loop did not exit after Stop")
	}
}
