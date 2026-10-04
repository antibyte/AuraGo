package flows

import (
	"strings"
	"testing"
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
		{"template found an empty text, retain on", map[string]any{"topic": "a", "payload": template, "retain": true}, map[string]any{"topic": "a", "payload": "", "retain": true}, true, ""},
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
