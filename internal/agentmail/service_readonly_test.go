package agentmail

import (
	"context"
	"testing"
)

func TestReadOnlyRelayDoesNotMarkMessageRead(t *testing.T) {
	notified := false
	service := NewService(ServiceConfig{
		Config: Config{InboxID: "inbox-1", ReadOnly: true},
		Notify: func(context.Context, string) error { notified = true; return nil },
	})
	if err := service.handleMessage(context.Background(), Message{ID: "message-1"}); err != nil {
		t.Fatal(err)
	}
	if !notified {
		t.Fatal("read-only relay was not delivered")
	}
}
