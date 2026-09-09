package desktop

import (
	"context"
	"testing"
)

func TestStickyNoteWidgetPersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	cfg := svc.Config()
	note := Widget{ID: "sticky-roundtrip", Title: "Sticky note", Type: "sticky-note", Icon: "notes", X: 124, Y: 88, W: 220, H: 220, Config: map[string]interface{}{"text": "First line\n<img src=x>\nGrüße ☕", "auto_size": false}}
	if err := svc.UpsertWidget(ctx, note, SourceUser); err != nil {
		t.Fatal(err)
	}
	// The fixture has no Docker backend; Close still closes the registry.
	_ = svc.Close()
	svc = testServiceWithConfig(t, cfg)
	widgets, err := svc.ListAllWidgets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, stored := range widgets {
		if stored.ID != note.ID {
			continue
		}
		found = true
		if stored.Type != note.Type || stored.Config["text"] != note.Config["text"] || stored.Config["auto_size"] != false || stored.X != note.X || stored.Y != note.Y || !stored.Visible || stored.Builtin {
			t.Fatalf("sticky note changed across restart: %+v", stored)
		}
	}
	if !found {
		t.Fatal("sticky note missing after restart")
	}
	_ = svc.Close()
	cfg.ReadOnly = true
	svc = testServiceWithConfig(t, cfg)
	if err := svc.UpsertWidget(ctx, note, SourceUser); err == nil {
		t.Fatal("read-only service accepted note edit")
	}
	if err := svc.DeleteWidget(ctx, note.ID, SourceUser); err == nil {
		t.Fatal("read-only service accepted note deletion")
	}
}
