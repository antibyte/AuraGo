package mailer

import (
	"context"
	"reflect"
	"testing"
)

func TestReplyToPersistsAndSurvivesMessageHydration(t *testing.T) {
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	indexed := Message{AccountID: "email:fixture", FolderID: "inbox", RemoteID: "message-1", ReplyTo: []string{"support@example.test"}}
	if _, err := store.PutMessages(context.Background(), []Message{indexed}); err != nil {
		t.Fatal(err)
	}
	stored, err := store.GetMessage(context.Background(), messageID(indexed.AccountID, indexed.FolderID, indexed.RemoteID))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored.ReplyTo, indexed.ReplyTo) {
		t.Fatalf("stored Reply-To = %#v, want %#v", stored.ReplyTo, indexed.ReplyTo)
	}

	hydrated := Message{}
	mergeHydratedMessage(&hydrated, stored)
	if !reflect.DeepEqual(hydrated.ReplyTo, indexed.ReplyTo) {
		t.Fatalf("hydrated Reply-To = %#v, want %#v", hydrated.ReplyTo, indexed.ReplyTo)
	}
}
