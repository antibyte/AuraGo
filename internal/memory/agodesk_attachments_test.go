package memory

import (
	"log/slog"
	"testing"
	"time"
)

func TestAgoDeskAttachmentLifecycle(t *testing.T) {
	stm, err := NewSQLiteMemory(":memory:", slog.Default())
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	record := AgoDeskAttachmentRecord{
		AttachmentID:       "att-1",
		TransportSessionID: "agodesk:dev-1",
		ConversationID:     "sess-1",
		Filename:           "diagram.png",
		MimeType:           "image/png",
		Kind:               "image",
		DeclaredSizeBytes:  1234,
		ExpectedSHA256:     "expected-sha",
		ExpiresAt:          expiresAt,
	}
	if err := stm.PrepareAgoDeskAttachment(record); err != nil {
		t.Fatalf("PrepareAgoDeskAttachment: %v", err)
	}

	prepared, err := stm.GetAgoDeskAttachment("att-1")
	if err != nil {
		t.Fatalf("GetAgoDeskAttachment prepared: %v", err)
	}
	if prepared == nil || prepared.Status != AgoDeskAttachmentStatusPrepared || prepared.ExpiresAt.IsZero() {
		t.Fatalf("prepared record = %+v", prepared)
	}

	uploaded, err := stm.MarkAgoDeskAttachmentUploaded("att-1", 1200, "actual-sha", "attachments/agodesk/sess-1/att-1/diagram.png", "image", "image/png")
	if err != nil {
		t.Fatalf("MarkAgoDeskAttachmentUploaded: %v", err)
	}
	if uploaded.Status != AgoDeskAttachmentStatusUploaded || uploaded.SizeBytes != 1200 || uploaded.SHA256 != "actual-sha" {
		t.Fatalf("uploaded record = %+v", uploaded)
	}

	messageID, err := stm.InsertMessage("sess-1", "user", "please inspect", false, false)
	if err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	if err := stm.BindAgoDeskAttachmentsToMessage("sess-1", messageID, []string{"att-1"}); err != nil {
		t.Fatalf("BindAgoDeskAttachmentsToMessage: %v", err)
	}

	byMessage, err := stm.ListAgoDeskAttachmentsForMessages([]int64{messageID})
	if err != nil {
		t.Fatalf("ListAgoDeskAttachmentsForMessages: %v", err)
	}
	got := byMessage[messageID]
	if len(got) != 1 || got[0].AttachmentID != "att-1" || got[0].Status != AgoDeskAttachmentStatusAccepted {
		t.Fatalf("attachments by message = %+v", byMessage)
	}
}

func TestCleanupExpiredAgoDeskAttachmentsOnlyRemovesUnboundPendingRecords(t *testing.T) {
	stm, err := NewSQLiteMemory(":memory:", slog.Default())
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	now := time.Now().UTC()
	if err := stm.PrepareAgoDeskAttachment(AgoDeskAttachmentRecord{
		AttachmentID:       "expired-prepared",
		TransportSessionID: "agodesk:dev-1",
		ConversationID:     "sess-1",
		Filename:           "old.txt",
		MimeType:           "text/plain",
		DeclaredSizeBytes:  12,
		ExpiresAt:          now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("Prepare expired: %v", err)
	}
	if err := stm.PrepareAgoDeskAttachment(AgoDeskAttachmentRecord{
		AttachmentID:       "fresh-prepared",
		TransportSessionID: "agodesk:dev-1",
		ConversationID:     "sess-1",
		Filename:           "fresh.txt",
		MimeType:           "text/plain",
		DeclaredSizeBytes:  12,
		ExpiresAt:          now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("Prepare fresh: %v", err)
	}
	if err := stm.PrepareAgoDeskAttachment(AgoDeskAttachmentRecord{
		AttachmentID:       "accepted-expired",
		TransportSessionID: "agodesk:dev-1",
		ConversationID:     "sess-1",
		Filename:           "kept.txt",
		MimeType:           "text/plain",
		DeclaredSizeBytes:  12,
		ExpiresAt:          now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("Prepare accepted: %v", err)
	}
	if _, err := stm.MarkAgoDeskAttachmentUploaded("accepted-expired", 12, "sha", "attachments/agodesk/sess-1/accepted-expired/kept.txt", "text", "text/plain"); err != nil {
		t.Fatalf("Mark accepted upload: %v", err)
	}
	messageID, err := stm.InsertMessage("sess-1", "user", "with accepted file", false, false)
	if err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	if err := stm.BindAgoDeskAttachmentsToMessage("sess-1", messageID, []string{"accepted-expired"}); err != nil {
		t.Fatalf("Bind accepted: %v", err)
	}

	removed, err := stm.CleanupExpiredAgoDeskAttachments(now)
	if err != nil {
		t.Fatalf("CleanupExpiredAgoDeskAttachments: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	for _, id := range []string{"fresh-prepared", "accepted-expired"} {
		got, err := stm.GetAgoDeskAttachment(id)
		if err != nil {
			t.Fatalf("Get %s: %v", id, err)
		}
		if got == nil {
			t.Fatalf("%s was removed unexpectedly", id)
		}
	}
	expired, err := stm.GetAgoDeskAttachment("expired-prepared")
	if err != nil {
		t.Fatalf("Get expired-prepared: %v", err)
	}
	if expired != nil {
		t.Fatalf("expired-prepared still present: %+v", expired)
	}
}
