package server

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/flows"
	"aurago/internal/i18n"
	"aurago/internal/planner"
	"aurago/internal/tools"
)

// Bounds of the planner issue and the failure notifications of a flow.
const (
	// flowNotifyNameRunes bounds the flow name in the planner issue title and in failure
	// notifications.
	flowNotifyNameRunes = 80
	// flowNotifyMessageRunes bounds the run error a failure notification carries. The engine
	// caps error messages at 1000 runes, too long for a push or Telegram message.
	flowNotifyMessageRunes = 300
	// flowFailureNotifyInterval is how often a flow that keeps failing notifies again (see
	// flowFailureNotifier).
	flowFailureNotifyInterval = time.Hour
	// flowNotifyMaxInFlight bounds the sends that have not returned, per channel (push and
	// Telegram each; see sendFlowNotification).
	flowNotifyMaxInFlight = 4
)

// flowDisplayName names a run's flow in the planner issue title and in notifications: the
// flow's name cut to flowNotifyNameRunes runes, or its id when the name is not known (the
// flow and its documents are gone).
func flowDisplayName(info flows.RunFinishedInfo) string {
	name := strings.TrimSpace(info.FlowName)
	if name == "" {
		name = info.Record.FlowID
	}
	return flowBoundRunes(name, flowNotifyNameRunes)
}

// flowIssue records (or resolves) the planner operational issue of a flow, like missions
// do. The fingerprint is per flow; a run without a flow id has none and records nothing.
// Reference is the mission id and may be empty (the planner stores it trimmed, unchecked).
func (s *Server) flowIssue(info flows.RunFinishedInfo, failed bool, message string) {
	if s.PlannerDB == nil || info.Record.FlowID == "" {
		return
	}
	fingerprint := "flow|" + info.Record.FlowID
	if !failed {
		if _, err := planner.ResolveOperationalIssue(s.PlannerDB, fingerprint, "The flow ran successfully.", time.Now()); err != nil {
			s.Logger.Warn("Flow operational issue could not be resolved", "flow", info.Record.FlowID, "error", err)
		}
		return
	}
	issue := planner.OperationalIssue{Source: "flow", Context: info.Record.FlowID, Title: fmt.Sprintf("Flow %s failed", flowDisplayName(info)),
		Detail: message, Severity: "error", Kind: planner.OperationalIssueKindRuntimeFailure, Reference: info.MissionID,
		Fingerprint: fingerprint, OccurredAt: time.Now()}
	issueID, err := planner.RecordOperationalIssue(s.PlannerDB, issue)
	if err != nil {
		s.Logger.Warn("Flow operational issue could not be recorded", "flow", info.Record.FlowID, "error", err)
		return
	}
	// Every recorded failure fires the planner_operational_issue trigger, not only the first
	// one: the flood rule (flowFailureNotifier) applies to notifications, not here. That is
	// what agent missions do (recordMissionIssue in Start, server.go), and a prompt
	// mission on this trigger is bounded on its own side: by its min_interval_seconds, and by
	// the mission queue, which holds at most one entry per mission.
	if s.MissionManagerV2 != nil {
		s.MissionManagerV2.NotifyPlannerOperationalIssue(issueID, issue.Source, issue.Severity, issue.Title)
	}
}

// flowFailureNotification builds the desktop notification for a failed live run; push and
// Telegram send its title and message. The message names the flow (flowDisplayName, at
// most 80 runes, the flow id when the name is unknown) and the run error, cut to
// flowNotifyMessageRunes runes. Both are filled into the translation in one pass, so a
// name that contains "{error}" (or an error that contains "{name}") is not filled in a
// second time, as i18n.T's map parameters would in map order.
func flowFailureNotification(lang string, info flows.RunFinishedInfo, message string) map[string]any {
	name := flowDisplayName(info)
	errText := flowBoundRunes(strings.TrimSpace(message), flowNotifyMessageRunes)
	fill := strings.NewReplacer("{{name}}", name, "{{error}}", errText, "{name}", name, "{error}", errText)
	return map[string]any{
		"title":   i18n.T(lang, "easydrag.notify.run_failed_title"),
		"message": fill.Replace(i18n.T(lang, "easydrag.notify.run_failed_message")),
		"type":    "error",
		"appId":   "easydrag",
		"context": map[string]any{"flow_id": info.Record.FlowID, "run_id": info.Record.ID},
	}
}

// notifyFlowFailure sends the failure notification chosen by settings.notify_on_error:
// desktop (default), push, telegram or off. The flood rule of flowFailureNotifier decides
// whether this failure notifies at all.
//
// A push or Telegram notification takes its channel's send slot before the flood rule is
// consulted: a notification dropped because every slot is busy leaves the flow's failing
// state as it was, so the flow's next failure can notify. When the flood rule then
// suppresses the notification, the slot is given back. A send that was accepted counts as
// a notification even when it fails later on its goroutine. Desktop notifications are
// always accepted.
func (s *Server) notifyFlowFailure(info flows.RunFinishedInfo, message string) {
	cfg := s.ConfigSnapshot()
	if cfg == nil || info.NotifyOnError == "off" {
		return
	}
	channel := info.NotifyOnError
	remote := channel == "push" || channel == "telegram"
	if remote && !s.flowNotify.acquire(channel) {
		s.Logger.Warn("Flow failure notification dropped; earlier notifications are still being sent",
			"channel", channel, "in_flight", flowNotifyMaxInFlight, "flow", info.Record.FlowID)
		return
	}
	if !s.flowNotify.alert(info.Record.FlowID, time.Now()) {
		if remote {
			s.flowNotify.release(channel)
		}
		s.Logger.Debug("Flow failure not notified; the flow notified that it is failing already",
			"flow", info.Record.FlowID, "run", info.Record.ID)
		return
	}
	payload := flowFailureNotification(cfg.Server.UILanguage, info, message)
	if remote {
		title, _ := payload["title"].(string)
		body, _ := payload["message"].(string)
		s.sendFlowNotification(cfg, channel, title, body)
		return
	}
	broadcastDesktopEvent(s, s.DesktopHub, desktop.Event{Type: "notification", Payload: payload, CreatedAt: time.Now().UTC()})
}

// flowSendNotification is tools.SendNotification; tests replace it.
var flowSendNotification = tools.SendNotification

// sendFlowNotification hands a push or Telegram failure notification to
// tools.SendNotification on a goroutine of its own, which gives back the channel's send
// slot when it returns; the caller has taken that slot (flowFailureNotifier.acquire).
//
// SendNotification takes no context. Its Telegram channel has a 15 s client timeout
// (telegramMessageClient), but its web push channel has none: push.Manager.SendPush calls
// webpush-go without an HTTP client, and webpush-go then sends with a zero http.Client, so
// one stalled push endpoint can hold a send for good. Each channel therefore has its own
// flowNotifyMaxInFlight slots: stalled push sends cannot silence Telegram, and a channel
// whose slots are all busy drops further notifications instead of adding goroutines.
func (s *Server) sendFlowNotification(cfg *config.Config, channel, title, body string) {
	send, logger := flowSendNotification, s.Logger
	go func() {
		defer s.flowNotify.release(channel)
		send(cfg, logger, channel, title, body, "high", nil)
	}()
}

// flowFailureNotifier holds the state of the flow failure notifications. The zero value
// is ready to use.
//
// Flood rule: a failed live run notifies (desktop, push or Telegram, as the flow's
// notify_on_error says) only when its flow was not failing before, or when the flow's last
// failure notification is flowFailureNotifyInterval (one hour) or more ago. A successful
// run ends the failing state, so the next failure notifies at once. A flow on a one-minute
// schedule that keeps failing thus notifies once and then at most once an hour, instead of
// 1440 times a day. Cancelled runs neither notify nor end the failing state, and a failure
// of a flow set to "off" records nothing, nor does a push or Telegram notification dropped
// for want of a send slot. The state lives in memory, so after a restart the first failure
// notifies again. The planner issue is recorded for every failure; it is deduplicated per
// flow (fingerprint "flow|<flow id>").
type flowFailureNotifier struct {
	mu       sync.Mutex
	alerted  map[string]time.Time // flow id → last failure notification, while the flow keeps failing
	inFlight map[string]int       // channel → sends that have not returned
}

// alert reports whether a failure of flowID at now may notify, and records the
// notification when it may. Entries older than flowFailureNotifyInterval are dropped on
// the way, so flows that stopped failing without a success (deleted, switched off) do not
// stay in the map.
func (n *flowFailureNotifier) alert(flowID string, now time.Time) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	for id, at := range n.alerted {
		if now.Sub(at) >= flowFailureNotifyInterval {
			delete(n.alerted, id)
		}
	}
	if _, failing := n.alerted[flowID]; failing {
		return false
	}
	if n.alerted == nil {
		n.alerted = map[string]time.Time{}
	}
	n.alerted[flowID] = now
	return true
}

// recovered ends the failing state of flowID after a successful run.
func (n *flowFailureNotifier) recovered(flowID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.alerted, flowID)
}

// acquire takes one of the flowNotifyMaxInFlight send slots of channel; false when all of
// them are busy.
func (n *flowFailureNotifier) acquire(channel string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.inFlight[channel] >= flowNotifyMaxInFlight {
		return false
	}
	if n.inFlight == nil {
		n.inFlight = map[string]int{}
	}
	n.inFlight[channel]++
	return true
}

// release gives back a slot of channel that acquire took.
func (n *flowFailureNotifier) release(channel string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.inFlight[channel] <= 1 {
		delete(n.inFlight, channel)
		return
	}
	n.inFlight[channel]--
}
