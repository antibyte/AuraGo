package server

import (
	"testing"
	"time"
)

func resetSessionRequestLockForTest(sessionID string) {
	muSessionRequestLocks.Lock()
	delete(sessionRequestLocks, sessionID)
	muSessionRequestLocks.Unlock()
}

func TestLockSessionRequestSerializesSameSessionWaiters(t *testing.T) {
	const sessionID = "test-session-lock"
	resetSessionRequestLockForTest(sessionID)

	unlockFirst := lockSessionRequest(sessionID)
	secondEntered := make(chan struct{})
	releaseSecond := make(chan struct{})

	go func() {
		unlockSecond := lockSessionRequest(sessionID)
		defer unlockSecond()
		close(secondEntered)
		<-releaseSecond
	}()

	select {
	case <-secondEntered:
		t.Fatal("second request entered session before first request released lock")
	case <-time.After(20 * time.Millisecond):
	}

	unlockFirst()
	select {
	case <-secondEntered:
	case <-time.After(time.Second):
		t.Fatal("second request did not enter after first request released lock")
	}
	close(releaseSecond)
}

func TestLockSessionRequestKeepsSingleLockForQueuedNonDefaultSession(t *testing.T) {
	const sessionID = "test-session-lock-queued"
	resetSessionRequestLockForTest(sessionID)

	unlockFirst := lockSessionRequest(sessionID)

	releaseSecond := make(chan struct{})
	secondEntered := make(chan struct{})

	go func() {
		unlockSecond := lockSessionRequest(sessionID)
		defer unlockSecond()
		close(secondEntered)
		<-releaseSecond
	}()
	time.Sleep(20 * time.Millisecond)

	unlockFirst()
	<-secondEntered

	thirdEntered := make(chan func())
	go func() {
		thirdEntered <- lockSessionRequest(sessionID)
	}()

	select {
	case unlockThird := <-thirdEntered:
		unlockThird()
		t.Fatal("third request entered while second request still held the same session lock")
	case <-time.After(20 * time.Millisecond):
	}

	close(releaseSecond)
	select {
	case unlockThird := <-thirdEntered:
		unlockThird()
	case <-time.After(time.Second):
		t.Fatal("third request did not enter after second request released lock")
	}
}

func TestSessionRequestActiveReflectsHeldLock(t *testing.T) {
	const sessionID = "test-session-lock-active"
	resetSessionRequestLockForTest(sessionID)

	if sessionRequestActive(sessionID) {
		t.Fatal("session should not be active before lock is held")
	}
	unlock := lockSessionRequest(sessionID)
	if !sessionRequestActive(sessionID) {
		unlock()
		t.Fatal("session should be active while lock is held")
	}
	unlock()
	if sessionRequestActive(sessionID) {
		t.Fatal("session should not be active after lock is released")
	}
}
