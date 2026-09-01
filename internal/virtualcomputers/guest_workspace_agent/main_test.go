//go:build linux

package main

import "testing"

func TestTryAcquireDoesNotQueuePastCapacity(t *testing.T) {
	slots := make(chan struct{}, 1)
	if !tryAcquire(slots) {
		t.Fatal("first operation should acquire the available slot")
	}
	if tryAcquire(slots) {
		t.Fatal("operation beyond capacity must be rejected without queuing")
	}
	release(slots)
	if !tryAcquire(slots) {
		t.Fatal("released slot should be reusable")
	}
	release(slots)
}
