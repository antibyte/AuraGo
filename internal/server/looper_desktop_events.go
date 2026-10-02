package server

import (
	"encoding/json"

	"aurago/internal/agent"
)

// looperDesktopBroker carries the desktop events of Looper tool calls to the
// desktop, which opens an app window only when such an event reaches it over
// SSE. Other tool feedback is dropped, so no chat window shows a Looper run.
//
// App windows open only in the finish step (allowOpen): the work prompt forbids
// opening the artifact, and a window opened early would merely be focused again
// by the finish step, still showing an old draft.
type looperDesktopBroker struct {
	agent.NoopBroker
	sse       *SSEBroadcaster
	allowOpen bool
}

func (b looperDesktopBroker) SendJSON(jsonStr string) {
	if b.sse == nil {
		return
	}
	var msg struct {
		Type    string `json:"type"`
		Payload struct {
			Type string `json:"type"`
		} `json:"payload"`
	}
	if json.Unmarshal([]byte(jsonStr), &msg) != nil || msg.Type != string(EventVirtualDesktop) {
		return
	}
	if !b.allowOpen && (msg.Payload.Type == "open_app" || msg.Payload.Type == "open_widget") {
		return
	}
	b.sse.SendJSON(jsonStr)
}

// looperFinishDispatch returns the dispatch context of the finish step, the one
// step that may open the result in a desktop app.
func looperFinishDispatch(dc *agent.DispatchContext) *agent.DispatchContext {
	if dc == nil {
		return nil
	}
	b, ok := dc.Broker.(looperDesktopBroker)
	if !ok {
		return dc
	}
	b.allowOpen = true
	finish := *dc
	finish.Broker = b
	return &finish
}
