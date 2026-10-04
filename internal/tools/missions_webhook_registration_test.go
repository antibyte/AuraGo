package tools

import "testing"

func TestWebhookMissionRoundtripAndDisableRemoveRegistration(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	webhooks := &fakeWebhookTriggerManager{}
	mm.SetWebhookManager(webhooks)
	if err := mm.Create(&MissionV2{ID: "roundtrip", Name: "Fixture", Prompt: "run", Enabled: true, ExecutionType: ExecutionTriggered, TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: "a"}}); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"b", "a"} {
		next, _ := mm.Get("roundtrip")
		next.TriggerConfig = &TriggerConfig{WebhookID: target}
		if err := mm.Update("roundtrip", next); err != nil {
			t.Fatal(err)
		}
	}
	if len(webhooks.callbacks["a"]) != 1 || len(webhooks.callbacks["b"]) != 0 {
		t.Fatal("trigger roundtrip accumulated callbacks")
	}
	next, _ := mm.Get("roundtrip")
	next.Enabled = false
	if err := mm.Update("roundtrip", next); err != nil {
		t.Fatal(err)
	}
	if len(webhooks.callbacks) != 0 {
		t.Fatal("disabled mission retained callback")
	}
	next.Enabled = true
	if err := mm.Update("roundtrip", next); err != nil {
		t.Fatal(err)
	}
	replacement := &fakeWebhookTriggerManager{}
	mm.SetWebhookManager(replacement)
	if len(webhooks.callbacks) != 0 || len(replacement.callbacks["a"]) != 1 {
		t.Fatal("manager replacement retained stale callbacks")
	}
	mm.Stop()
	if len(replacement.callbacks) != 0 {
		t.Fatal("shutdown retained callbacks")
	}
}
