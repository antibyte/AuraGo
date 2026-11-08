package desktop

import (
	"context"
	"testing"
)

func TestWidgetPinConfigPersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	cfg := svc.Config()
	widget := Widget{
		ID:    "pinned-widget",
		Title: "Pinned widget",
		Type:  "custom",
		Icon:  "widgets",
		X:     64,
		Y:     96,
		W:     320,
		H:     220,
		Config: map[string]interface{}{
			"pinned":    true,
			"auto_size": false,
		},
	}
	if err := svc.UpsertWidget(ctx, widget, SourceUser); err != nil {
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
		if stored.ID != widget.ID {
			continue
		}
		found = true
		pinned, ok := stored.Config["pinned"].(bool)
		if !ok || !pinned {
			t.Fatalf("pinned flag changed across restart: %+v", stored.Config)
		}
		if stored.X != widget.X || stored.Y != widget.Y {
			t.Fatalf("pinned widget moved across restart: %+v", stored)
		}
	}
	if !found {
		t.Fatal("pinned widget missing after restart")
	}
	_ = svc.Close()
}
