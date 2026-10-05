package tools

import (
	"testing"
	"time"
)

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

func TestWebhookMissionQuarantineEligibilityIsLiveAndReadOnly(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	defer mm.Stop()
	webhooks := &fakeWebhookTriggerManager{}
	mm.SetWebhookManager(webhooks)
	mission := &MissionV2{
		ID: "eligibility", Name: "Eligibility", Prompt: "run", Enabled: true,
		ExecutionType: ExecutionTriggered, TriggerType: TriggerWebhook,
		TriggerConfig: &TriggerConfig{WebhookID: "hook-a", MinIntervalSeconds: 300},
	}
	if err := mm.Create(mission); err != nil {
		t.Fatal(err)
	}
	key := mission.ID + "|" + string(TriggerWebhook)
	eligible := webhooks.eligible[key]
	if eligible == nil || !eligible() {
		t.Fatal("active local mission should be eligible for a safe notice")
	}
	if _, consumed := mm.lastTriggerFire[mission.ID+"|webhook"]; consumed {
		t.Fatal("eligibility check consumed mission cooldown state")
	}

	updated, ok := mm.Get(mission.ID)
	if !ok {
		t.Fatal("created mission could not be loaded")
	}
	updated.TriggerConfig = &TriggerConfig{WebhookID: "hook-b", MinIntervalSeconds: 300}
	if err := mm.Update(mission.ID, updated); err != nil {
		t.Fatal(err)
	}
	if eligible() {
		t.Fatal("stale registration remained eligible after webhook target changed")
	}
	currentEligible := webhooks.eligible[key]
	if currentEligible == nil || !currentEligible() {
		t.Fatal("updated webhook registration should be eligible")
	}

	mm.mu.Lock()
	mm.lastTriggerFire = map[string]time.Time{mission.ID + "|webhook": time.Now()}
	mm.mu.Unlock()
	if currentEligible() {
		t.Fatal("rate-limited mission should not count as a notice target")
	}
	if got := mm.lastTriggerFire[mission.ID+"|webhook"]; time.Since(got) > time.Second {
		t.Fatal("eligibility query modified the existing cooldown timestamp")
	}

	updated, ok = mm.Get(mission.ID)
	if !ok {
		t.Fatal("updated mission could not be loaded")
	}
	updated.Enabled = false
	if err := mm.Update(mission.ID, updated); err != nil {
		t.Fatal(err)
	}
	if currentEligible() {
		t.Fatal("disabled mission remained eligible for a safe notice")
	}
}
