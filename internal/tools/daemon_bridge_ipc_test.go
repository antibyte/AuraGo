package tools

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestDaemonToolIPCRejectsReplayAndKeepsCommandsSeparate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	calls := 0
	run := newDaemonRunIPC(ctx, writer, &DaemonBridgeRun{Call: func(ctx context.Context, request DaemonToolRequest) DaemonToolResponse {
		calls++
		return DaemonToolResponse{Status: "success", Result: "fixture result"}
	}})
	commands := make(chan DaemonCommand, 4)
	go func() {
		decoder := json.NewDecoder(reader)
		for {
			var command DaemonCommand
			if decoder.Decode(&command) != nil {
				return
			}
			commands <- command
		}
	}()
	data := json.RawMessage(`{"id":1,"tool":"docker","parameters":{"operation":"list"},"timeout":1}`)
	run.toolRequest(data)
	run.toolRequest(data) // A duplicate ID never repeats a side effect.
	run.toolRequest(json.RawMessage(`{"id":2,"tool":"docker","parameters":{},"forged_session":"other"}`))
	if calls != 1 {
		t.Fatalf("replayed or malformed request dispatched: %d", calls)
	}
	var result DaemonCommand
	select {
	case result = <-commands:
	case <-time.After(time.Second):
		t.Fatal("tool result was not published")
	}
	var response DaemonToolResponse
	if result.Type != DaemonCmdToolResult || json.Unmarshal(result.Data, &response) != nil || response.ID != 1 || response.Status != "success" {
		t.Fatalf("invalid result envelope: %+v", result)
	}
	if !run.send(NewConfigUpdateCommand(map[string]string{"INTERVAL": "2"})) {
		t.Fatal("config command rejected")
	}
	select {
	case command := <-commands:
		if command.Type != DaemonCmdConfigUpdate {
			t.Fatal("config command was consumed as a tool response")
		}
	case <-time.After(time.Second):
		t.Fatal("config update not delivered")
	}
}

func TestDaemonToolIPCStopsOldGenerationBeforePublishing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	started, done := make(chan struct{}), make(chan struct{})
	run := newDaemonRunIPC(ctx, writer, &DaemonBridgeRun{Call: func(owner context.Context, request DaemonToolRequest) DaemonToolResponse {
		close(started)
		<-owner.Done()
		return DaemonToolResponse{Status: "success", Result: "late old generation"}
	}})
	go func() {
		defer close(done)
		run.toolRequest(json.RawMessage(`{"id":1,"tool":"docker","parameters":{},"timeout":60}`))
	}()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runner owner did not cancel callback")
	}
	var command DaemonCommand
	readDone := make(chan error, 1)
	go func() { readDone <- json.NewDecoder(reader).Decode(&command) }()
	select {
	case err := <-readDone:
		if err != io.EOF {
			t.Fatalf("old generation published a result: %+v, error=%v", command, err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled generation left its stdin open")
	}
	if run.send(DaemonCommand{Type: DaemonCmdToolResult}) {
		t.Fatal("cancelled generation accepted new output")
	}
}
