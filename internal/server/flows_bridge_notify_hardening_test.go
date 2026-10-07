package server

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/tools"
)

// The planner issue title carries at most 80 runes of the flow's name.
func TestC15PlannerIssueTitleIsBounded(t *testing.T) {
	e := c15NewEnv(t)
	name := strings.Repeat("Ü", 300)
	e.bridge.FlowRunFinished(flows.RunFinishedInfo{Started: true, FlowName: name, NotifyOnError: "off",
		Record: flows.RunRecord{ID: "run_c15longname1", FlowID: "flow_c15longname"},
		Result: flows.RunResult{Status: flows.RunError, ErrorMessage: "kaputt"}})
	issues := e.c15Issues(t)
	if len(issues) != 1 {
		t.Fatalf("planner issues = %+v", issues)
	}
	if n := strings.Count(issues[0].Title, "Ü"); n != flowNotifyNameRunes {
		t.Fatalf("the title holds %d runes of the name: %q", n, issues[0].Title)
	}
}

// The notification texts are bounded (name 80 runes, error 300 runes), use the flow id
// when the name is unknown, and fill the placeholders in one pass.
func TestC15FailureNotificationTexts(t *testing.T) {
	c15LoadI18n()
	info := c15Failure("flow_c15texts001", "run_c15texts0001", "desktop")
	info.FlowName = strings.Repeat("N", 200)
	msg, _ := flowFailureNotification("en", info, strings.Repeat("E", 2000))["message"].(string)
	if want := strings.Repeat("N", flowNotifyNameRunes) + "…: " + strings.Repeat("E", flowNotifyMessageRunes) + "…"; msg != want {
		t.Fatalf("message = %q (%d runes)", msg, utf8.RuneCountInString(msg))
	}
	info.FlowName = "{error}"
	if msg, _ := flowFailureNotification("en", info, "kaputt {name}")["message"].(string); msg != "{error}: kaputt {name}" {
		t.Fatalf("placeholders filled twice: %q", msg)
	}
	info.FlowName = "  "
	if msg, _ := flowFailureNotification("en", info, "kaputt")["message"].(string); msg != "flow_c15texts001: kaputt" {
		t.Fatalf("without a name: %q", msg)
	}
	payload := flowFailureNotification("en", info, "kaputt")
	if title, _ := payload["title"].(string); title != "Flow failed" {
		t.Fatalf("title = %q", title)
	}
}

// The translations come from ui/lang/easydrag; the German payload reads German.
func TestC15FailureNotificationGerman(t *testing.T) {
	c15LoadI18n()
	info := flows.RunFinishedInfo{FlowName: "Bericht", Record: flows.RunRecord{ID: "run_aaaaaaaaaaaa", FlowID: "flow_aaaaaaaaaa"}}
	payload := flowFailureNotification("de", info, "FLOW_TOOL_ERROR: kaputt")
	if payload["title"] != "Flow fehlgeschlagen" || payload["message"] != "Bericht: FLOW_TOOL_ERROR: kaputt" {
		t.Fatalf("payload = %+v", payload)
	}
	// The server language setting is what notifyFlowFailure uses.
	e := c15NewEnv(t)
	e.s.Cfg.Server.UILanguage = "de"
	e.s.notifyFlowFailure(c15Failure("flow_c15german01", "run_c15german001", "desktop"), "FLOW_TOOL_ERROR: kaputt")
	events := e.c15Drain()
	if len(events) != 1 {
		t.Fatalf("events = %+v", events)
	}
	if got, _ := events[0].Payload.(map[string]any); got["title"] != "Flow fehlgeschlagen" {
		t.Fatalf("notification = %+v", got)
	}
}

// Flood rule: a flow that keeps failing notifies once; a success ends the failing state,
// so the next failure notifies again. The planner issue still counts every failure.
func TestC15FailureNotificationFlood(t *testing.T) {
	e := c15NewEnv(t)
	for i := range 3 {
		e.bridge.FlowRunFinished(c15Failure("flow_c15flood001", fmt.Sprintf("run_c15flood00%d", i), "desktop"))
	}
	if n := c15Count(e.c15Drain(), "notification"); n != 1 {
		t.Fatalf("three failures sent %d notifications", n)
	}
	// Another flow fails on its own account.
	e.bridge.FlowRunFinished(c15Failure("flow_c15flood002", "run_c15flood010", "desktop"))
	if n := c15Count(e.c15Drain(), "notification"); n != 1 {
		t.Fatalf("the second flow sent %d notifications", n)
	}
	e.bridge.FlowRunFinished(flows.RunFinishedInfo{Started: true, Record: flows.RunRecord{ID: "run_c15flood004", FlowID: "flow_c15flood001"},
		Result: flows.RunResult{Status: flows.RunSuccess}})
	e.bridge.FlowRunFinished(c15Failure("flow_c15flood001", "run_c15flood005", "desktop"))
	if n := c15Count(e.c15Drain(), "notification"); n != 1 {
		t.Fatalf("a failure after a success sent %d notifications", n)
	}
	issues := e.c15Issues(t)
	occurrences := 0
	for _, is := range issues {
		occurrences += is.Occurrences
	}
	if len(issues) != 2 || occurrences != 5 {
		t.Fatalf("planner issues = %+v", issues)
	}

	// The interval: a flow that keeps failing notifies again after an hour.
	var n flowFailureNotifier
	t0 := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	if !n.alert("f", t0) || n.alert("f", t0.Add(59*time.Minute)) || !n.alert("f", t0.Add(time.Hour+59*time.Minute)) {
		t.Fatal("the interval rule is off")
	}
	n.recovered("f")
	if !n.alert("f", t0.Add(2*time.Hour)) {
		t.Fatal("a recovered flow must notify at once")
	}
	if !n.alert("g", t0.Add(4*time.Hour)) || len(n.alerted) != 1 {
		t.Fatalf("stale entries are kept: %v", n.alerted)
	}
	// A flow set to "off" records nothing, so switching it to desktop notifies at once.
	e.s.notifyFlowFailure(c15Failure("flow_c15flood003", "run_c15flood020", "off"), "x")
	e.s.notifyFlowFailure(c15Failure("flow_c15flood003", "run_c15flood021", "desktop"), "x")
	if n := c15Count(e.c15Drain(), "notification"); n != 1 {
		t.Fatalf("after off: %d notifications", n)
	}
}

// c15BlockingSender replaces flowSendNotification with a sender that reports each send on
// started ("channel|title|message|priority") and returns once unblock is closed. The
// original sender comes back when the test ends.
func c15BlockingSender(t *testing.T) (started chan string, unblock chan struct{}) {
	t.Helper()
	started, unblock = make(chan string, 64), make(chan struct{})
	old := flowSendNotification
	t.Cleanup(func() { flowSendNotification = old })
	flowSendNotification = func(_ *config.Config, _ *slog.Logger, channel, title, message, priority string, _ tools.DiscordSendFunc, _ ...tools.TelnyxSendFunc) string {
		started <- channel + "|" + title + "|" + message + "|" + priority
		<-unblock
		return `{"status":"success"}`
	}
	return started, unblock
}

// c15Started waits for n sends of channel.
func c15Started(t *testing.T, started <-chan string, channel string, n int) {
	t.Helper()
	for range n {
		select {
		case got := <-started:
			if !strings.HasPrefix(got, channel+"|") || !strings.HasSuffix(got, "|high") {
				t.Fatalf("send = %q, want a %s send", got, channel)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("the %s sends did not start", channel)
		}
	}
}

// c15NoSend fails when a send starts within a short while.
func c15NoSend(t *testing.T, started <-chan string) {
	t.Helper()
	select {
	case got := <-started:
		t.Fatalf("unexpected send %q", got)
	case <-time.After(50 * time.Millisecond):
	}
}

// c15Busy returns how many sends of channel hold a slot.
func c15Busy(e *c15Env, channel string) int {
	e.s.flowNotify.mu.Lock()
	defer e.s.flowNotify.mu.Unlock()
	return e.s.flowNotify.inFlight[channel]
}

// c15WaitIdle waits until no send of channel holds a slot.
func c15WaitIdle(t *testing.T, e *c15Env, channel string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for c15Busy(e, channel) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d %s sends still hold their slots", c15Busy(e, channel), channel)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Push and Telegram sends run on their own goroutines, at most flowNotifyMaxInFlight at a
// time per channel; a send that never returns cannot pile up goroutines, and stalled push
// sends do not silence Telegram.
func TestC15NotificationSendsAreBounded(t *testing.T) {
	e := c15NewEnv(t)
	logs := &c15LogBuffer{}
	e.s.Logger = c15Logger(logs)
	started, unblock := c15BlockingSender(t)
	for i := range flowNotifyMaxInFlight + 2 {
		e.bridge.FlowRunFinished(c15Failure(fmt.Sprintf("flow_c15slots%03d", i), fmt.Sprintf("run_c15slots%03d", i), "push"))
	}
	c15Started(t, started, "push", flowNotifyMaxInFlight)
	c15NoSend(t, started)
	if n := strings.Count(logs.String(), "Flow failure notification dropped"); n != 2 {
		t.Fatalf("dropped warnings = %d\n%s", n, logs.String())
	}
	// Telegram has slots of its own.
	e.bridge.FlowRunFinished(c15Failure("flow_c15slotstg1", "run_c15slotstg1", "telegram"))
	c15Started(t, started, "telegram", 1)
	close(unblock)
	c15WaitIdle(t, e, "push")
	c15WaitIdle(t, e, "telegram")
	e.bridge.FlowRunFinished(c15Failure("flow_c15slotsnew", "run_c15slotsnew", "push"))
	c15Started(t, started, "push", 1)
}

// A notification dropped for want of a send slot does not count for the flood rule: the
// flow's next failure notifies once a slot is free. A notification the flood rule
// suppresses gives its slot back.
func TestC15DroppedNotificationKeepsTheFloodStateOpen(t *testing.T) {
	e := c15NewEnv(t)
	started, unblock := c15BlockingSender(t)
	for i := range flowNotifyMaxInFlight {
		e.bridge.FlowRunFinished(c15Failure(fmt.Sprintf("flow_c15busy%03d", i), fmt.Sprintf("run_c15busy%03d", i), "telegram"))
	}
	c15Started(t, started, "telegram", flowNotifyMaxInFlight)
	// Flow X fails while every slot is busy: dropped.
	e.bridge.FlowRunFinished(c15Failure("flow_c15victim01", "run_c15victim01", "telegram"))
	c15NoSend(t, started)
	close(unblock)
	c15WaitIdle(t, e, "telegram")
	// Flow X fails again with free slots: it notifies.
	e.bridge.FlowRunFinished(c15Failure("flow_c15victim01", "run_c15victim02", "telegram"))
	c15Started(t, started, "telegram", 1)
	c15WaitIdle(t, e, "telegram")
	// A busy flow fails again: the flood rule suppresses it, and its slot is free again.
	e.bridge.FlowRunFinished(c15Failure("flow_c15busy000", "run_c15busy100", "telegram"))
	c15NoSend(t, started)
	if n := c15Busy(e, "telegram"); n != 0 {
		t.Fatalf("a suppressed notification holds %d slots", n)
	}
}
