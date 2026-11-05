package flows

import (
	"testing"
	"time"
)

func TestDurableLocationRejectsUnreconstructableDateContext(t *testing.T) {
	b := newFlow("Location")
	start := b.node("start", TypeTriggerManual, nil)
	wait := b.node("wait", durableWaitType, map[string]any{"seconds": 1.0})
	b.edge(start, PortOut, wait)
	doc := b.build()
	custom := time.FixedZone("Custom/Private", 3600)
	policy, err := durableLocation(doc, custom)
	if err != nil || policy != durableZoneIndependent {
		t.Fatalf("zone independent: %q %v", policy, err)
	}
	if _, err := restoreDurableLocation(doc, policy); err != nil {
		t.Fatal(err)
	}
	date := b.node("date", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "x", "type": "text", "value": "{{trigger.data.date | date('YYYY')}}"}}})
	b.edge(wait, PortOut, date)
	dateDoc := b.build()
	if _, err := durableLocation(dateDoc, custom); err == nil {
		t.Fatal("date filter accepted in custom zone")
	}
	if _, err := restoreDurableLocation(dateDoc, durableZoneIndependent); err == nil {
		t.Fatal("changed graph restored with independent policy")
	}
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("zone data unavailable: %v", err)
	}
	policy, err = durableLocation(dateDoc, berlin)
	if err != nil || policy != "iana:Europe/Berlin" {
		t.Fatalf("named zone: %q %v", policy, err)
	}
	restored, err := restoreDurableLocation(dateDoc, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, historical := range []string{"1999-01-01 12:00", "1999-07-01 12:00", "2050-01-01 12:00"} {
		a, okA := toTime(historical, berlin)
		b, okB := toTime(historical, restored)
		if !okA || !okB || !a.Equal(b) {
			t.Fatalf("historical zone differs: %s", historical)
		}
	}
	spoof := time.FixedZone("Europe/Berlin", 0)
	if _, err := durableLocation(dateDoc, spoof); err == nil {
		t.Fatal("custom zone masqueraded as IANA")
	}
	if _, err := restoreDurableLocation(dateDoc, "iana:Missing/Unknown"); err == nil {
		t.Fatal("unknown zone accepted")
	}
}

func TestDurableLocationRejectsDynamicAndDateConditions(t *testing.T) {
	b := newFlow("Conditions")
	start := b.node("start", TypeTriggerManual, nil)
	check := b.node("check", TypeIf, map[string]any{"condition": cond("x", "eq", "y")})
	wait := b.node("wait", durableWaitType, map[string]any{"seconds": 1.0})
	b.edge(start, PortOut, check)
	b.edge(check, PortFalse, wait)
	doc := b.build()
	custom := time.FixedZone("LocalCustom", 0)
	if _, err := durableLocation(doc, custom); err != nil {
		t.Fatalf("ordinary condition: %v", err)
	}
	for _, condition := range []any{cond("2020-01-01", "before", "2021-01-01"), map[string]any{"rows": []any{map[string]any{"left": "x", "op": "eq", "right": "y", "type": "date"}}}, "{{trigger.data.condition}}"} {
		next, _ := doc.Clone()
		next.NodeByID(check).Params["condition"] = condition
		if _, err := durableLocation(next, custom); err == nil {
			t.Fatalf("unsafe condition accepted: %+v", condition)
		}
	}
}
