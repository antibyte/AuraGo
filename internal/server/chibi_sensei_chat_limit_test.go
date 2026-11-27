package server

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestChibiSenseiChatConcurrentRequestsAreCapped: four chat requests run at the same time, the fifth answers 409
// runtime_busy before the stream starts, and a slot is free again once the held requests ended.
func TestChibiSenseiChatConcurrentRequestsAreCapped(t *testing.T) {
	s, token, fake, rt, pid := chibiTestFixture(t, true)
	run := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	fake.Hold = make(chan struct{})
	var wg sync.WaitGroup
	var once sync.Once
	release := func() {
		once.Do(func() { close(fake.Hold) })
		wg.Wait()
	}
	t.Cleanup(release)
	// One request holds the instance; the others wait for it, and every one of them occupies a request slot.
	for i := 0; i < chibiMaxChatRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			chibiDo(t, s, token, http.MethodPost, "/runtime/chat", chibiChatReq(run, chibiTarget("tuned", "Hi")))
		}()
	}
	deadline := time.Now().Add(10 * time.Second)
	for len(rt.chatSlots) < chibiMaxChatRequests {
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d requests running", len(rt.chatSlots), chibiMaxChatRequests)
		}
		time.Sleep(5 * time.Millisecond)
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runtime/chat", chibiChatReq(run, chibiTarget("tuned", "Hi"))),
		http.StatusConflict, "runtime_busy")
	release()
	if len(rt.chatSlots) != 0 {
		t.Fatalf("%d slots still taken after every request ended", len(rt.chatSlots))
	}
	if w := chibiDo(t, s, token, http.MethodPost, "/runtime/chat", chibiChatReq(run, chibiTarget("tuned", "Hi"))); w.Code != http.StatusOK {
		t.Fatalf("request after release: status %d: %s", w.Code, w.Body.String())
	}
}
