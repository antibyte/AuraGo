package flows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// homeCheckArgs describes what is wrong with the arguments a smart home or planner tool
// got, or "". Whatever the parameters were, the tool only sees bounded, well formed
// values.
func homeCheckArgs(c ToolRequest) string {
	text := func(key string) string { s, _ := c.Args[key].(string); return s }
	switch c.Tool {
	case "home_assistant":
		switch op := text("operation"); op {
		case "get_state":
			if len(c.Args) != 2 {
				return "get_state arguments " + fmt.Sprint(c.Args)
			}
		case "call_service":
			if !isHAIdent(text("domain")) || !isHAIdent(text("service")) || len(text("domain"))+len(text("service")) > maxHAServiceBytes {
				return "a malformed domain or service"
			}
			if data, has := c.Args["service_data"]; has {
				m, ok := data.(map[string]any)
				if !ok || len(m) == 0 {
					return "bad service_data"
				}
				budget := maxHAServiceDataValues
				if s, err := marshalCompact(m); err != nil || len(s) > maxHAServiceDataBytes || !valueFits(m, maxHAServiceDataDepth, &budget) {
					return "service_data out of bounds"
				}
			}
		default:
			return "operation " + op
		}
		if id := text("entity_id"); len(id) > maxHAEntityBytes || !strings.Contains(id, ".") || !isHAIdent(strings.Replace(id, ".", "_", 1)) {
			return "a malformed entity id"
		}
	case "mqtt_publish":
		if _, err := mqttTopic(c.Args["topic"]); err != nil || text("topic") != strings.TrimSpace(text("topic")) {
			return "a malformed topic"
		}
		if p, ok := c.Args["payload"].(string); !ok || len(p) > maxMQTTPayloadBytes || !utf8.ValidString(p) {
			return "a malformed payload"
		}
		if qos, ok := c.Args["qos"].(int); !ok || qos < 0 || qos > 2 {
			return "a malformed qos"
		}
		if _, ok := c.Args["retain"].(bool); !ok {
			return "retain is not a bool"
		}
	case "manage_appointments", "manage_todos":
		if c.Args["operation"] != "add" {
			return "operation " + fmt.Sprint(c.Args["operation"])
		}
		if _, err := plannerTitle(c.Args["title"]); err != nil {
			return "a malformed title"
		}
		if d, has := c.Args["description"]; has && (len(text("description")) > maxPlannerDescriptionBytes || d == "") {
			return "a malformed description"
		}
		for _, key := range []string{"date_time", "notification_at", "due_date"} {
			if v, has := c.Args[key]; has {
				if _, err := time.Parse(time.RFC3339, fmt.Sprint(v)); err != nil {
					return key + " is not RFC 3339: " + err.Error()
				}
			}
		}
		if c.Tool == "manage_appointments" && c.Args["date_time"] == nil {
			return "an appointment without a date"
		}
		if c.Tool == "manage_todos" && !slices.Contains(todoPriorities, text("priority")) {
			return "a bad priority"
		}
	}
	return ""
}

// Validate and Execute see raw parameters of any type. No hook may panic or produce
// unbounded text, everything Validate rejects Execute must reject too, a parameter problem
// is found before the tool is called, and what reaches a tool is bounded and well formed.
func TestHomeHooksSurviveOddParams(t *testing.T) {
	reg := homeRegistry(t)
	vc := ValidateContext{Mode: ModePublish}
	var failures []string
	fail := func(format string, args ...any) { failures = append(failures, fmt.Sprintf(format, args...)) }
	clean := func(msg string) bool {
		return len(msg) <= maxEchoMessageBytes && utf8.ValidString(msg) && !strings.ContainsAny(msg, homeLF+homeCR+homeNUL)
	}
	runs := 0

	check := func(def *NodeDef, params map[string]any, label string) {
		runs++
		node := &Node{ID: testNodeID(1), Key: "home", Type: def.Type, Params: params}
		tools := &fakeTools{respond: homeAnswers}
		var (
			issues  []Issue
			res     ExecResult
			execErr error
		)
		for _, step := range []struct {
			name string
			fn   func()
		}{
			{"Validate", func() { issues = def.Validate(node, vc) }},
			{"Availability", func() { _ = def.Availability() }},
			{"FieldsOf", func() { _ = def.FieldsOf(node) }},
			{"OutputPorts", func() { _ = def.OutputPorts(node) }},
			{"EffectsOf", func() { _ = def.EffectsOf(node) }},
			{"Execute", func() { res, execErr = execDef(def, params, &Services{Tools: tools, Location: time.UTC}) }},
		} {
			if p := catchPanic(step.fn); p != nil {
				fail("%s: %s panicked: %v", label, step.name, p)
				return
			}
		}
		declared := map[string]bool{}
		for _, p := range def.Params {
			declared[p.Name] = true
		}
		if len(issues) > len(def.Params) {
			fail("%s: %d issues", label, len(issues))
		}
		for _, is := range issues {
			if is.Code != IssueParamInvalid || is.Severity != SeverityError || is.NodeID != node.ID || !declared[is.Param] || !clean(is.Message) {
				fail("%s: bad issue %.200q", label, fmt.Sprintf("%+v", is))
			}
			if execErr == nil {
				fail("%s: Validate rejected %s but Execute succeeded", label, is.Param)
			}
		}
		if execErr != nil {
			ne := asNodeError(execErr)
			if ne == nil || ne.Code == "" || !clean(ne.Message) {
				fail("%s: bad Execute error %T %.200q", label, execErr, execErr.Error())
				return
			}
			if ne.Code == "FLOW_PARAM_INVALID" && tools.count() != 0 {
				fail("%s: rejected the parameters after %d tool calls", label, tools.count())
			}
		} else if raw, err := json.Marshal(res.Output); err != nil || !utf8.Valid(raw) || tools.count() != 1 {
			fail("%s: bad success: %v %.100s after %d calls", label, err, raw, tools.count())
		}
		for _, c := range tools.allCalls() {
			if _, err := json.Marshal(c.Args); err != nil || len(c.AllowedTools) != 1 || c.AllowedTools[0] != c.Tool || c.Tool != def.Tool {
				fail("%s: malformed tool request %v: %+v", label, err, c.Tool)
			}
			if problem := homeCheckArgs(c); problem != "" {
				fail("%s: %s", label, problem)
			}
		}
	}

	values := oddParamValues()
	for _, def := range reg.All() {
		base := homeBases[def.Type]
		names := []string{"unknown_param"}
		for _, p := range def.Params {
			names = append(names, p.Name)
		}
		for _, ov := range values {
			for _, name := range names {
				params := make(map[string]any, len(base)+1)
				for k, v := range base {
					params[k] = v
				}
				params[name] = ov.v
				check(def, params, fmt.Sprintf("%s %s=%s", def.Type, name, ov.name))
			}
			all := make(map[string]any, len(names))
			for _, name := range names {
				all[name] = ov.v
			}
			check(def, all, fmt.Sprintf("%s all=%s", def.Type, ov.name))
		}
		// get_state with the same odd values in the rest of the parameters.
		if def.Type == TypeHomeAssistant {
			for _, ov := range values {
				check(def, map[string]any{"operation": "get_state", "entity": "sensor.t", "service": ov.v, "service_data": ov.v}, "get_state hidden="+ov.name)
			}
		}
		check(def, nil, def.Type+" nil params")
		check(def, map[string]any{}, def.Type+" empty params")
		check(def, base, def.Type+" plain params")

		if p := catchPanic(func() {
			_ = def.Validate(nil, vc)
			_ = def.FieldsOf(nil)
			_ = def.OutputPorts(nil)
			_ = def.EffectsOf(nil)
		}); p != nil {
			fail("%s: a hook panicked on a nil node: %v", def.Type, p)
		}
	}
	if runs < 600 {
		t.Fatalf("only %d combinations ran", runs)
	}
	if len(failures) > 0 {
		if len(failures) > 15 {
			failures = append(failures[:15], fmt.Sprintf("... and %d more", len(failures)-15))
		}
		t.Fatalf("%d of %d combinations failed:\n%s", len(failures), runs, strings.Join(failures, "\n"))
	}
}

// An invoker that fails, a run that ended before the call and a run without tools all
// fail the node, and nothing reaches a tool in the last two.
func TestHomeNodesFailClosed(t *testing.T) {
	for _, typ := range []string{TypeHomeAssistant, TypeMQTTPublish, TypeAppointmentAdd, TypeTodoAdd} {
		def := lookupDef(t, homeRegistry(t), typ)
		params := homeParams(typ, nil)
		tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) {
			return ToolResponse{}, errors.New(strings.Repeat("down ", 500))
		}}
		if _, err := execDef(def, params, &Services{Tools: tools}); asNodeError(err).Code != "FLOW_NODE_FAILED" || len(asNodeError(err).Message) > maxEchoMessageBytes {
			t.Errorf("%s: invoker error: %v", typ, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		tools = &fakeTools{respond: homeAnswers}
		if _, err := execDefCtx(ctx, def, withDefaults(def, params), &Services{Tools: tools}); err == nil || tools.count() != 0 {
			t.Errorf("%s: a cancelled run called the tool: %v after %d calls", typ, err, tools.count())
		}
		if _, err := execDef(def, params, &Services{}); asNodeError(err).Code != "FLOW_TOOLS_UNAVAILABLE" {
			t.Errorf("%s: no tools: %v", typ, err)
		}
		// The call is made with exactly this tool allowed.
		tools = &fakeTools{respond: homeAnswers}
		if _, err := execDef(def, params, &Services{Tools: tools}); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if c := tools.last(t); c.Tool != def.Tool || !reflect.DeepEqual(c.AllowedTools, []string{def.Tool}) || c.FlowID != "flow_test" || c.RunID != "run_test" {
			t.Errorf("%s: request %+v", typ, c)
		}
	}
}

func TestHomeNodesAreRegisteredWithTheirTools(t *testing.T) {
	want := map[string]string{
		TypeHomeAssistant: "home_assistant", TypeMQTTPublish: "mqtt_publish", TypeAppointmentAdd: "manage_appointments", TypeTodoAdd: "manage_todos",
	}
	categories := map[string]string{TypeHomeAssistant: "smart_home", TypeMQTTPublish: "smart_home", TypeAppointmentAdd: "planner", TypeTodoAdd: "planner"}
	reg := homeRegistry(t)
	if n := len(reg.All()); n != len(want) {
		t.Errorf("%d types registered, want %d", n, len(want))
	}
	for typ, tool := range want {
		def := lookupDef(t, reg, typ)
		if def.Tool != tool || def.Category != categories[typ] || def.Trigger || def.Execute == nil || def.Validate == nil {
			t.Errorf("%s: tool %q category %q", typ, def.Tool, def.Category)
		}
		// Without an environment the node needs setup; the environment's word is passed on.
		if a := def.Availability(); a.State != NeedsSetupState {
			t.Errorf("%s: availability without an environment %+v", typ, a)
		}
		withEnv := NewRegistry()
		if err := registerHomeNodes(withEnv, StaticEnv{tool: {}}); err != nil {
			t.Fatal(err)
		}
		if a := lookupDef(t, withEnv, typ).Availability(); a.State != AvailableState {
			t.Errorf("%s: availability with its tool %+v", typ, a)
		}
	}
	// The effects the publish dialog lists.
	for typ, want := range map[string][]Effect{TypeMQTTPublish: {EffectControlsDevices}, TypeHomeAssistant: {EffectControlsDevices}, TypeAppointmentAdd: nil, TypeTodoAdd: nil} {
		got := lookupDef(t, reg, typ).EffectsOf(&Node{Params: map[string]any{}})
		if len(got) != len(want) || len(want) > 0 && got[0] != want[0] {
			t.Errorf("%s: effects %v, want %v", typ, got, want)
		}
	}
}
