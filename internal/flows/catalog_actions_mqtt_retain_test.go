package flows

import (
	"strings"
	"testing"
	"time"
)

// An empty retained message clears the topic. A payload written out as blank, or left
// out, does that on purpose (the MQTT idiom); a payload template that resolved to nothing
// must not do it by accident, but only with retain on.
func TestMQTTRetainedNilPayload(t *testing.T) {
	publish := func(raw, resolved map[string]any) (*fakeTools, error) {
		tools := &fakeTools{respond: homeAnswers}
		_, err := homeExecRaw(t, TypeMQTTPublish, raw, resolved, tools)
		return tools, err
	}
	template := "{{trigger.data.value}}"
	for _, c := range []struct {
		name     string
		raw      map[string]any
		resolved map[string]any
		ok       bool
		payload  any
	}{
		{"template found nothing, retain on", map[string]any{"topic": "a", "payload": template, "retain": true}, map[string]any{"topic": "a", "payload": nil, "retain": true}, false, nil},
		{"template found nothing, retain from a template", map[string]any{"topic": "a", "payload": template, "retain": "{{x.r}}"}, map[string]any{"topic": "a", "payload": nil, "retain": "yes"}, false, nil},
		{"template found nothing, retain off", map[string]any{"topic": "a", "payload": template}, map[string]any{"topic": "a", "payload": nil}, true, ""},
		{"template found nothing, retain false", map[string]any{"topic": "a", "payload": template, "retain": false}, map[string]any{"topic": "a", "payload": nil, "retain": false}, true, ""},
		{"template found an empty text, retain on", map[string]any{"topic": "a", "payload": template, "retain": true}, map[string]any{"topic": "a", "payload": "", "retain": true}, false, nil},
		{"template found an empty text, retain off", map[string]any{"topic": "a", "payload": template, "retain": false}, map[string]any{"topic": "a", "payload": "", "retain": false}, true, ""},
		// A filter turns a missing field into empty text; that must not clear the topic either.
		{"trim of nothing, retain on", map[string]any{"topic": "a", "payload": "{{trigger.data.value | trim}}", "retain": true}, map[string]any{"topic": "a", "payload": "", "retain": true}, false, nil},
		{"lower of nothing, retain on", map[string]any{"topic": "a", "payload": "{{trigger.data.value | lower}}", "retain": true}, map[string]any{"topic": "a", "payload": "", "retain": true}, false, nil},
		{"upper of nothing, retain on", map[string]any{"topic": "a", "payload": "{{trigger.data.value | upper}}", "retain": true}, map[string]any{"topic": "a", "payload": "", "retain": true}, false, nil},
		{"truncate of nothing, retain on", map[string]any{"topic": "a", "payload": "{{trigger.data.value | truncate(10)}}", "retain": true}, map[string]any{"topic": "a", "payload": "", "retain": true}, false, nil},
		{"two templates that render nothing, retain on", map[string]any{"topic": "a", "payload": "{{x.a}}{{x.b}}", "retain": true}, map[string]any{"topic": "a", "payload": "", "retain": true}, false, nil},
		{"trim of nothing, retain off", map[string]any{"topic": "a", "payload": "{{trigger.data.value | trim}}", "retain": false}, map[string]any{"topic": "a", "payload": "", "retain": false}, true, ""},
		{"trim of a value, retain on", map[string]any{"topic": "a", "payload": "{{trigger.data.value | trim}}", "retain": true}, map[string]any{"topic": "a", "payload": "on", "retain": true}, true, "on"},
		{"template found a value, retain on", map[string]any{"topic": "a", "payload": template, "retain": true}, map[string]any{"topic": "a", "payload": "on", "retain": true}, true, "on"},
		{"text with a template is never nothing", map[string]any{"topic": "a", "payload": "v={{x.y}}", "retain": true}, map[string]any{"topic": "a", "payload": "v=", "retain": true}, true, "v="},
		{"blank payload, retain on, clears on purpose", map[string]any{"topic": "a", "payload": "", "retain": true}, map[string]any{"topic": "a", "payload": "", "retain": true}, true, ""},
		{"payload left out, retain on, clears on purpose", map[string]any{"topic": "a", "retain": true}, map[string]any{"topic": "a", "retain": true}, true, ""},
		{"payload null in the document, retain on", map[string]any{"topic": "a", "payload": nil, "retain": true}, map[string]any{"topic": "a", "payload": nil, "retain": true}, true, ""},
	} {
		tools, err := publish(c.raw, c.resolved)
		if c.ok {
			if err != nil || tools.count() != 1 || tools.last(t).Args["payload"] != c.payload {
				t.Errorf("%s: %v after %d calls", c.name, err, tools.count())
			}
			continue
		}
		homeWantInvalid(t, c.name, tools, err, "")
		if ne := asNodeError(err); ne == nil || !strings.Contains(ne.Message, "resolved to nothing") {
			t.Errorf("%s: message %v", c.name, err)
		}
	}
}

// The same with the real template resolution, not hand-made values: a missing field
// becomes null, and trim, lower, upper and truncate make it empty text. Both would clear
// the topic of a retained message.
func TestMQTTRetainedMissingFieldThroughFilters(t *testing.T) {
	env := &Env{Roots: map[string]any{"trigger": map[string]any{"data": map[string]any{"present": " On "}}}, Location: time.UTC}
	for _, c := range []struct {
		payload string
		want    any // what the payload resolves to when the field is missing
	}{
		{"{{trigger.data.value}}", nil},
		{"{{trigger.data.value | trim}}", ""},
		{"{{trigger.data.value | lower}}", ""},
		{"{{trigger.data.value | upper}}", ""},
		{"{{trigger.data.value | truncate(10)}}", ""},
	} {
		raw := map[string]any{"topic": "a", "payload": c.payload, "retain": true}
		resolved, err := ResolveParams(raw, env)
		if err != nil {
			t.Fatalf("%s: %v", c.payload, err)
		}
		if resolved["payload"] != c.want {
			t.Fatalf("%s resolved to %#v, expected %#v (the premise of this test)", c.payload, resolved["payload"], c.want)
		}
		tools := &fakeTools{respond: homeAnswers}
		if _, err := homeExecRaw(t, TypeMQTTPublish, raw, resolved, tools); asNodeError(err) == nil || asNodeError(err).Code != "FLOW_PARAM_INVALID" || tools.count() != 0 {
			t.Errorf("%s: published a retained empty message: %v after %d calls", c.payload, err, tools.count())
		}
		// Without retain it is an ordinary empty message.
		raw["retain"] = false
		resolved, err = ResolveParams(raw, env)
		tools = &fakeTools{respond: homeAnswers}
		if _, err := homeExecRaw(t, TypeMQTTPublish, raw, resolved, tools); err != nil || tools.count() != 1 {
			t.Errorf("%s without retain: %v", c.payload, err)
		}
	}
	// A field that is there gives a payload, retained.
	raw := map[string]any{"topic": "a", "payload": "{{trigger.data.present | trim}}", "retain": true}
	resolved, err := ResolveParams(raw, env)
	tools := &fakeTools{respond: homeAnswers}
	if _, execErr := homeExecRaw(t, TypeMQTTPublish, raw, resolved, tools); err != nil || execErr != nil || tools.last(t).Args["payload"] != "On" {
		t.Errorf("a field that is present: %v %v", err, execErr)
	}
}
