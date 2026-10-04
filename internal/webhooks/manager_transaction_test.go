package webhooks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestManagerRejectsCorruptConfigurationWithoutOverwriting(t *testing.T) {
	for _, body := range []string{"", "null", "[", `{"wrong":"shape"}`} {
		dir := t.TempDir()
		path := filepath.Join(dir, "webhooks.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if mgr, err := NewManager(path, filepath.Join(dir, "log.json")); err == nil || mgr != nil {
			t.Fatal("corrupt webhook configuration accepted")
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != body {
			t.Fatal("corrupt source was overwritten")
		}
	}
}

func TestManagerFailedMutationsPreservePublishedState(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(filepath.Join(dir, "webhooks.json"), filepath.Join(dir, "log.json"))
	if err != nil {
		t.Fatal(err)
	}
	original, err := m.Create(Webhook{Name: "original", Slug: "original", Enabled: true, Format: WebhookFormat{Fields: []FieldMapping{{Source: "one"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Update(original.ID, Webhook{Name: "changed", Enabled: false, Slug: "!"}); err == nil {
		t.Fatal("invalid slug accepted")
	}
	if got, _ := m.Get(original.ID); !reflect.DeepEqual(got, original) {
		t.Fatal("validation failure changed state")
	}
	if _, err = m.Update(original.ID, Webhook{Name: "changed", Delivery: DeliveryConfig{PromptTemplate: "{{unsupported}}"}}); err == nil {
		t.Fatal("invalid template accepted")
	}
	if got, _ := m.Get(original.ID); !reflect.DeepEqual(got, original) {
		t.Fatal("template failure changed state")
	}
	// Point the writer at an existing directory to force a deterministic failure.
	m.filePath = dir
	if _, err = m.Update(original.ID, Webhook{Name: "changed"}); err == nil {
		t.Fatal("expected update write failure")
	}
	if err = m.Delete(original.ID); err == nil {
		t.Fatal("expected delete write failure")
	}
	if _, err = m.Create(Webhook{Name: "new", Slug: "new-hook"}); err == nil {
		t.Fatal("expected create write failure")
	}
	if got := m.List(); len(got) != 1 || !reflect.DeepEqual(got[0], original) {
		t.Fatal("write failure changed state")
	}
	copy, _ := m.Get(original.ID)
	copy.Format.Fields[0].Source = "changed"
	if got, _ := m.Get(original.ID); got.Format.Fields[0].Source != "one" {
		t.Fatal("caller mutated stored state")
	}
}

func TestKeyedWebhookMissionRegistrationRoundtrip(t *testing.T) {
	m := &Manager{}
	one, two := 0, 0
	m.RegisterMissionTriggerForKey("mission-one", "a", func([]byte) { one++ })
	m.RegisterMissionTriggerForKey("mission-two", "a", func([]byte) { two++ })
	m.RegisterMissionTriggerForKey("mission-one", "b", func([]byte) { one++ })
	m.RegisterMissionTriggerForKey("mission-one", "a", func([]byte) { one++ })
	m.NotifyWebhookFired("a", []byte("fixture"))
	if one != 1 || two != 1 {
		t.Fatalf("duplicate/missing callback: %d/%d", one, two)
	}
	m.UnregisterMissionTrigger("mission-one")
	m.NotifyWebhookFired("a", nil)
	if one != 1 || two != 2 {
		t.Fatalf("unregister affected wrong mission: %d/%d", one, two)
	}
	m.NotifyWebhookFired("b", nil)
	if one != 1 {
		t.Fatal("stale registration retained")
	}
}
