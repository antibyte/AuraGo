package flows

import (
	"reflect"
	"testing"
	"time"
)

func homeRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	if err := registerHomeNodes(reg, nil); err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestHomeAssistantNode(t *testing.T) {
	tools := &fakeTools{respond: toolReply(`Tool Output: {"status":"success","service":"light.turn_off","affected_entities":["light.flur"],"count":1}`)}
	def := lookupDef(t, homeRegistry(t), TypeHomeAssistant)
	res, err := execDef(def, map[string]any{"entity": "light.flur", "service": "turn_off", "service_data": map[string]any{"transition": 2.0}}, &Services{Tools: tools})
	if err != nil || res.Output["ok"] != true || res.Output["service"] != "light.turn_off" || !reflect.DeepEqual(res.Output["affected_entities"], []any{"light.flur"}) {
		t.Fatalf("call_service = %#v, %v", res.Output, err)
	}
	want := map[string]any{"operation": "call_service", "domain": "light", "service": "turn_off", "entity_id": "light.flur", "service_data": map[string]any{"transition": 2.0}}
	if args := tools.last(t).Args; !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v", args)
	}
	if _, err := execDef(def, map[string]any{"entity": "switch.a", "service": "homeassistant.toggle"}, &Services{Tools: tools}); err != nil {
		t.Fatal(err)
	}
	if args := tools.last(t).Args; args["domain"] != "homeassistant" || args["service"] != "toggle" {
		t.Fatalf("explicit domain = %#v", args)
	}

	tools.respond = toolReply(`Tool Output: {"status":"success","entity":{"entity_id":"sensor.temp","state":"21.5","attributes":{"unit_of_measurement":"°C"},"last_changed":"2026-10-03T06:00:00Z"}}`)
	res, err = execDef(def, map[string]any{"operation": "get_state", "entity": "sensor.temp"}, &Services{Tools: tools})
	if err != nil || res.Output["state"] != "21.5" || res.Output["entity_id"] != "sensor.temp" {
		t.Fatalf("get_state = %#v, %v", res.Output, err)
	}
	if len(def.EffectsOf(&Node{Params: map[string]any{"operation": "get_state"}})) != 0 ||
		!reflect.DeepEqual(def.EffectsOf(&Node{Params: map[string]any{}}), []Effect{EffectControlsDevices}) {
		t.Fatal("HA effects mismatch")
	}
	if _, err := execDef(def, map[string]any{"entity": "light.x"}, &Services{Tools: tools}); asNodeError(err).Code != "FLOW_PARAM_INVALID" {
		t.Fatalf("missing service = %v", err)
	}
}

func TestMQTTPublishNode(t *testing.T) {
	tools := &fakeTools{respond: toolReply(`Tool Output: {"status":"success","topic":"home/a","qos":1,"retained":true}`)}
	def := lookupDef(t, homeRegistry(t), TypeMQTTPublish)
	res, err := execDef(def, map[string]any{"topic": "home/a", "payload": map[string]any{"on": true}, "qos": "1", "retain": true}, &Services{Tools: tools})
	if err != nil || res.Output["published"] != true || res.Output["topic"] != "home/a" {
		t.Fatalf("mqtt = %#v, %v", res.Output, err)
	}
	want := map[string]any{"topic": "home/a", "payload": `{"on":true}`, "qos": 1, "retain": true}
	if args := tools.last(t).Args; !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v", args)
	}
}

func TestPlannerNodes(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("tzdata missing")
	}
	tools := &fakeTools{respond: toolReply(`Tool Output: {"id":"apt_1","message":"Appointment created","status":"success"}`)}
	svc := &Services{Tools: tools, Location: berlin}
	appt := lookupDef(t, homeRegistry(t), TypeAppointmentAdd)
	res, err := execDef(appt, map[string]any{"title": "Zahnarzt", "date_time": "2026-10-05 09:00", "remind_minutes": 30.0}, svc)
	if err != nil || res.Output["id"] != "apt_1" || res.Output["date_time"] != "2026-10-05T09:00:00+02:00" {
		t.Fatalf("appointment = %#v, %v", res.Output, err)
	}
	want := map[string]any{"operation": "add", "title": "Zahnarzt", "date_time": "2026-10-05T09:00:00+02:00", "notification_at": "2026-10-05T08:30:00+02:00"}
	if args := tools.last(t).Args; !reflect.DeepEqual(args, want) {
		t.Fatalf("appointment args = %#v", args)
	}
	if _, err := execDef(appt, map[string]any{"title": "x", "date_time": "bald"}, svc); asNodeError(err).Code != "FLOW_PARAM_INVALID" {
		t.Fatalf("bad date = %v", err)
	}

	tools.respond = toolReply(`Tool Output: {"id":"todo_1","message":"Todo created","status":"success"}`)
	todo := lookupDef(t, homeRegistry(t), TypeTodoAdd)
	res, err = execDef(todo, map[string]any{"title": "Müll", "due_date": "2026-10-06"}, svc)
	if err != nil || res.Output["id"] != "todo_1" {
		t.Fatalf("todo = %#v, %v", res.Output, err)
	}
	if args := tools.last(t).Args; args["operation"] != "add" || args["priority"] != "medium" || args["due_date"] != "2026-10-06T00:00:00+02:00" {
		t.Fatalf("todo args = %#v", args)
	}
}

func TestRegisterActionNodes(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterActionNodes(reg, nil); err != nil {
		t.Fatal(err)
	}
	if n := len(reg.All()); n != 15 {
		t.Fatalf("RegisterActionNodes registered %d types, want 15", n)
	}
}
