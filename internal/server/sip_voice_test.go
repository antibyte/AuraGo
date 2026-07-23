package server

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestVoiceTurnCancellationGenerationKeepsNewestTurn(t *testing.T) {
	runner := NewVoiceActionRunner(nil)
	var firstCancelled atomic.Bool
	var secondCancelled atomic.Bool
	firstCtx, firstCancel := context.WithCancel(context.Background())
	firstGeneration := runner.installVoiceTurnCancel("call-1", func() {
		firstCancelled.Store(true)
		firstCancel()
	})
	secondGeneration := runner.installVoiceTurnCancel("call-1", func() {
		secondCancelled.Store(true)
	})
	if !firstCancelled.Load() || firstCtx.Err() == nil {
		t.Fatal("installing a replacement turn did not cancel the previous turn")
	}

	runner.releaseVoiceTurnCancel("call-1", firstGeneration)
	runner.CancelVoiceTurn("call-1")
	if !secondCancelled.Load() {
		t.Fatal("stale turn cleanup removed the newest cancellation handle")
	}
	runner.releaseVoiceTurnCancel("call-1", secondGeneration)
}
