package flows

import (
	"context"
	"strings"
	"time"
)

// Smart home and planner node types.
const (
	TypeHomeAssistant  = "home.assistant"
	TypeMQTTPublish    = "mqtt.publish"
	TypeAppointmentAdd = "planner.appointment_add"
	TypeTodoAdd        = "planner.todo_add"
)

// RegisterActionNodes registers the 15 curated action nodes of phase 1.
func RegisterActionNodes(reg *Registry, env CatalogEnv) error {
	for _, register := range []func(*Registry, CatalogEnv) error{registerDocNodes, registerWebNodes, registerNotifyNodes, registerHomeNodes} {
		if err := register(reg, env); err != nil {
			return err
		}
	}
	return nil
}

func registerHomeNodes(reg *Registry, env CatalogEnv) error {
	for _, def := range []*NodeDef{homeAssistantDef(env), mqttPublishDef(env), appointmentAddDef(env), todoAddDef(env)} {
		if err := reg.Register(def); err != nil {
			return err
		}
	}
	return nil
}

// homeAssistantDef defines home.assistant.
//
// Risk: a service call can do anything Home Assistant can: shell_command.*,
// python_script.*, script.*, automation.*, hassio.* and homeassistant.restart are
// services like light.turn_on. The home_assistant tool has no list of dangerous
// domains of its own. Its only guards are the settings home_assistant.readonly (no
// call_service at all) and home_assistant.blocked_services / allowed_services (exact
// "domain.service" names, no allow list means every service is allowed), which the
// dispatcher and tools.HACallService apply to every call, a flow's included. This node
// adds no list of its own: the service is the author's choice and has to be written
// out (it is not templatable, and Execute refuses a template there even though the run
// resolved it), and the settings are the place to forbid a service. What the node can
// do is show the author what a literal service does: shell_command and python_script
// count as running code, hassio and homeassistant.restart/stop as a system change (see
// homeAssistantEffects), which makes the publish dialog ask. With a plain
// service ("turn_on") the domain is the entity's, so an entity that comes from
// untrusted data chooses the domain: write "light.turn_on" to pin it. The entity and
// service_data are sensitive sinks; the service is a literal and so is no sink.
//
// Output: get_state returns the entity's state and attributes, which an outsider can
// shape (a sensor fed by mail, RSS, MQTT or a calendar, a media title), so the output
// is untrusted. That also marks the answer of a service call as untrusted, which only
// holds the service name and the ids of the changed entities; the lint cannot tell the
// two operations apart. The trigger.ha_state trigger is pinned as trusted by a plan
// test (catalog_triggers_test.go), which is inconsistent with this: the same states
// reach the flow there.
//
// Re-run: Retry runs the call again after a failure, and a service that is not
// idempotent (toggle, a counter, a script) then runs twice.
func homeAssistantDef(env CatalogEnv) *NodeDef {
	def := actionDef(TypeHomeAssistant, "smart_home", "home", "home_assistant", env)
	def.UntrustedOutput = true
	def.PrimaryInput = "entity"
	callService := &Visibility{Param: "operation", Equals: []string{"call_service"}}
	def.Params = []ParamSpec{
		{Name: "operation", Kind: ParamSegmented, LabelKey: "easydrag.param.ha_operation", Default: "call_service",
			Options: []Option{option("call_service", "ha_call_service"), option("get_state", "ha_get_state")}},
		{Name: "entity", Kind: ParamSelect, LabelKey: "easydrag.param.ha_entity", Required: true, Templatable: true, OptionsSource: "ha_entities", SensitiveSink: true},
		{Name: "service", Kind: ParamText, LabelKey: "easydrag.param.ha_service", HelpKey: "easydrag.help.ha_service", Required: true, VisibleIf: callService},
		{Name: "service_data", Kind: ParamJSON, LabelKey: "easydrag.param.ha_service_data", Templatable: true, SensitiveSink: true, VisibleIf: callService},
	}
	def.OutputFields = []FieldSpec{{Name: "state", Type: "text", Primary: true}, {Name: "attributes", Type: "object"},
		{Name: "affected_entities", Type: "list"}, {Name: "entity_id", Type: "text"},
		{Name: "last_changed", Type: "text"}, {Name: "ok", Type: "bool"}, {Name: "service", Type: "text"}}
	def.EffectsFunc = homeAssistantEffects
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		issues := choiceIssue(n, "operation", "call_service", haOperations)
		issues = append(issues, literalIssueIfSet(n, "entity", haEntityID)...)
		// The service and its data only matter for a service call. A get_state that is
		// written out ignores them (they may still be in the document, hidden).
		if op, ok := choiceParam(n.Params["operation"], "call_service", haOperations...); ok && op == "get_state" {
			return issues
		}
		if v := n.Params["service"]; !isEmptyValue(v) {
			if isTemplateText(v) {
				issues = append(issues, paramIssue(n, IssueParamInvalid, SeverityError, "service", asNodeError(errHAServiceTemplate()).Message))
			} else {
				issues = append(issues, literalIssue(n, "service", func(v any) (string, error) {
					_, _, err := haServiceName(v)
					return "", err
				})...)
			}
		}
		return append(issues, literalIssueIfSet(n, "service_data", readerCheck(haServiceData))...)
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		operation, ok := choiceParam(in.Params["operation"], "call_service", haOperations...)
		if !ok {
			return ExecResult{}, choiceError("operation", haOperations)
		}
		entity, err := haEntityID(in.Params["entity"])
		if err != nil {
			return ExecResult{}, err
		}
		if operation == "get_state" {
			return homeGetState(ctx, in, entity)
		}
		domain, service, err := haServiceParam(in)
		if err != nil {
			return ExecResult{}, err
		}
		if domain == "" {
			// The entity is a valid id, so it has a domain.
			domain, _, _ = strings.Cut(entity, ".")
		}
		data, err := haServiceData(in.Params["service_data"])
		if err != nil {
			return ExecResult{}, err
		}
		args := map[string]any{"operation": "call_service", "domain": domain, "service": service, "entity_id": entity}
		if data != nil {
			args["service_data"] = data
		}
		out, err := callTool(ctx, in, "home_assistant", args)
		if err != nil {
			return ExecResult{}, err
		}
		if err := requireSuccess(out, "Home Assistant tool"); err != nil {
			return ExecResult{}, err
		}
		// The ids of the entities Home Assistant changed. Only text entries are kept.
		affected := []any{}
		if list, ok := out["affected_entities"].([]any); ok {
			for _, item := range list {
				if id, ok := item.(string); ok {
					affected = append(affected, id)
				}
			}
		}
		return ExecResult{Output: map[string]any{"ok": true, "service": domain + "." + service, "affected_entities": affected}}, nil
	}
	return def
}

// homeAssistantEffects lists what a home.assistant node does outwardly. Only a
// get_state that is written out is read-only: a template, a value that cannot be read
// and a missing node are assumed to change something. A service call controls devices;
// a literal service of the shell_command or python_script domain also runs code, and one
// of the hassio domain or homeassistant.restart and homeassistant.stop changes the
// system, which the publish dialog then asks about (IsRisky). The domain of a plain
// service is the entity's, when that is written out too. A service that is a template
// or cannot be read stays a plain device control, as the run decides it. It is a hook:
// it copes with a nil node and any parameter.
func homeAssistantEffects(n *Node) []Effect {
	effects := []Effect{EffectControlsDevices}
	if n == nil {
		return effects
	}
	if op, ok := choiceParam(n.Params["operation"], "call_service", haOperations...); ok && op == "get_state" {
		return nil
	}
	domain, service, err := haServiceName(n.Params["service"])
	if err != nil {
		return effects
	}
	if domain == "" {
		id, err := haEntityID(n.Params["entity"])
		if err != nil {
			return effects
		}
		domain, _, _ = strings.Cut(id, ".")
	}
	switch domain {
	case "shell_command", "python_script":
		effects = append(effects, EffectRunsCode)
	case "hassio":
		effects = append(effects, EffectSystemChange)
	case "homeassistant":
		if service == "restart" || service == "stop" {
			effects = append(effects, EffectSystemChange)
		}
	}
	return effects
}

// homeGetState reads one entity. The answer must hold the entity with a text state;
// anything else (a plain-text refusal, a different object) is not a state. The attributes
// are cut off when they nest deeper than maxJSONDepth: the engine fails a node whose
// output nests beyond its own limit, which hostile attributes could use to fail a flow.
func homeGetState(ctx context.Context, in ExecInput, entity string) (ExecResult, error) {
	out, err := callTool(ctx, in, "home_assistant", map[string]any{"operation": "get_state", "entity_id": entity})
	if err != nil {
		return ExecResult{}, err
	}
	if err := requireSuccess(out, "Home Assistant tool"); err != nil {
		return ExecResult{}, err
	}
	e, _ := out["entity"].(map[string]any)
	state, hasState := e["state"].(string)
	if e == nil || !hasState {
		return ExecResult{}, NewNodeError("FLOW_TOOL_ERROR", "Home Assistant returned no state for %s", quoteForError(entity))
	}
	attributes, _ := e["attributes"].(map[string]any)
	if attributes == nil {
		attributes = map[string]any{}
	}
	if !nestingWithin(attributes, maxJSONDepth) {
		return ExecResult{}, NewNodeError("FLOW_TOOL_ERROR", "the attributes of %s are nested more than %d levels deep", quoteForError(entity), maxJSONDepth)
	}
	return ExecResult{Output: map[string]any{"entity_id": firstNonEmpty(outString(e, "entity_id"), entity), "state": state,
		"attributes": attributes, "last_changed": outString(e, "last_changed")}}, nil
}

// mqttPublishDef defines mqtt.publish. The topic and the payload are sensitive sinks:
// untrusted data that picks the topic or the message can switch a device, and a
// retained message stays on the broker. The payload never reaches the output (it is
// the message the node sends; the output is the topic the node was given), so it is
// OutputIndependent; being a sink it still warns. The output is made from the node's
// own parameters and is trusted.
//
// Re-run: after a failure Retry publishes again, and a broker that got the first message
// delivers it twice (at QoS 1 and 2 as well, the tool does not deduplicate).
func mqttPublishDef(env CatalogEnv) *NodeDef {
	def := actionDef(TypeMQTTPublish, "smart_home", "broadcast", "mqtt_publish", env)
	def.PrimaryInput = "payload"
	def.Effects = []Effect{EffectControlsDevices}
	def.Params = []ParamSpec{
		{Name: "topic", Kind: ParamText, LabelKey: "easydrag.param.mqtt_topic", Required: true, Templatable: true, SensitiveSink: true},
		{Name: "payload", Kind: ParamTextarea, LabelKey: "easydrag.param.mqtt_payload", Templatable: true, SensitiveSink: true, OutputIndependent: true},
		{Name: "qos", Kind: ParamSelect, LabelKey: "easydrag.param.mqtt_qos", Default: "0",
			Options: []Option{{Value: "0", Label: "0"}, {Value: "1", Label: "1"}, {Value: "2", Label: "2"}}},
		{Name: "retain", Kind: ParamBool, LabelKey: "easydrag.param.mqtt_retain", Default: false},
	}
	def.OutputFields = []FieldSpec{{Name: "published", Type: "bool", Primary: true}, {Name: "topic", Type: "text"}}
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		issues := literalIssueIfSet(n, "topic", mqttTopic)
		issues = append(issues, literalIssueIfSet(n, "payload", mqttPayload)...)
		issues = append(issues, literalIssue(n, "qos", readerCheck(mqttQoS))...)
		return append(issues, literalIssue(n, "retain", readerCheck(mqttRetain))...)
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		topic, err := mqttTopic(in.Params["topic"])
		if err != nil {
			return ExecResult{}, err
		}
		payload, err := mqttPayload(in.Params["payload"])
		if err != nil {
			return ExecResult{}, err
		}
		qos, err := mqttQoS(in.Params["qos"])
		if err != nil {
			return ExecResult{}, err
		}
		retain, err := mqttRetain(in.Params["retain"])
		if err != nil {
			return ExecResult{}, err
		}
		// An empty retained message clears the topic. A payload that is written out as
		// blank (or left out) says so on purpose, the MQTT idiom; a payload template that
		// found nothing (a missing field) must not do it by accident. A filter does not
		// make it safe: {{x.y | trim}} turns a missing field into "". So a template whose
		// result is nothing or empty text is refused; text that mixes a template with
		// other words renders something and passes.
		if retain && payload == "" && in.Node != nil && isTemplateText(in.Node.Params["payload"]) {
			return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "the payload template resolved to nothing; a retained empty message would clear the topic")
		}
		out, err := callTool(ctx, in, "mqtt_publish", map[string]any{"topic": topic, "payload": payload, "qos": qos, "retain": retain})
		if err != nil {
			return ExecResult{}, err
		}
		if err := requireSuccess(out, "MQTT tool"); err != nil {
			return ExecResult{}, err
		}
		return ExecResult{Output: map[string]any{"published": true, "topic": topic}}, nil
	}
	return def
}

// plannerDef is the common part of the planner nodes. They write into the user's own
// planner, which is not an outward effect (the planner decides later whether a reminder
// goes out), so they list none. Nothing picks a calendar or an account (the tool has
// only the one planner) and the description is plain content, so it is no sink. The
// title is a sensitive sink: planner.BuildPromptContextText lists the titles of open
// todos and upcoming appointments in the main agent's system prompt, so an untrusted
// title is a persistent prompt-injection channel; that is also why a title is one
// bounded line. The nodes offer no wake_agent or agent_instruction, and the description
// is not in the prompt snapshot. The title is part of the output (the node returns it
// with the id), so only the description is OutputIndependent.
//
// Times: the planner stores a time string as it is and compares it as TEXT, in SQL,
// with time.Now().UTC().Format(RFC3339) (planner.GetDueNotifications,
// AutoExpireAppointments). A time sent in another zone ("...+02:00") sorts by its local
// digits, so a reminder fires late by the offset (early in a negative zone) and an
// appointment is marked overdue at the wrong time. The nodes therefore send every time
// to the tool in UTC (the form with "Z"); the node's own output keeps the zone the
// date was written in. A planner add is not idempotent: Retry after an answer that got
// lost creates the entry twice.
func plannerDef(typ, icon, tool string, env CatalogEnv) *NodeDef {
	def := actionDef(typ, "planner", icon, tool, env)
	def.PrimaryInput = "title"
	return def
}

func appointmentAddDef(env CatalogEnv) *NodeDef {
	def := plannerDef(TypeAppointmentAdd, "calendar-plus", "manage_appointments", env)
	def.Params = []ParamSpec{
		{Name: "title", Kind: ParamText, LabelKey: "easydrag.param.title", Required: true, Templatable: true, SensitiveSink: true},
		{Name: "date_time", Kind: ParamDateTime, LabelKey: "easydrag.param.date_time", Required: true, Templatable: true},
		{Name: "description", Kind: ParamTextarea, LabelKey: "easydrag.param.description", Templatable: true, OutputIndependent: true},
		{Name: "remind_minutes", Kind: ParamNumber, LabelKey: "easydrag.param.remind_minutes", Default: 0.0},
	}
	def.OutputFields = []FieldSpec{{Name: "id", Type: "text", Primary: true}, {Name: "title", Type: "text"}, {Name: "date_time", Type: "text"}}
	def.Validate = func(n *Node, vc ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		issues := literalIssueIfSet(n, "title", plannerTitle)
		issues = append(issues, literalIssueIfSet(n, "date_time", timeCheck(vc.Location, "date_time"))...)
		issues = append(issues, literalIssueIfSet(n, "description", plannerDescription)...)
		return append(issues, literalIssueIfSet(n, "remind_minutes", readerCheck(remindMinutes))...)
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		title, err := plannerTitle(in.Params["title"])
		if err != nil {
			return ExecResult{}, err
		}
		at, ok, err := plannerTime(in.Params["date_time"], in.Services.Loc(), "date_time")
		if err != nil {
			return ExecResult{}, err
		}
		if !ok {
			return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "enter a date and time")
		}
		description, err := plannerDescription(in.Params["description"])
		if err != nil {
			return ExecResult{}, err
		}
		remind, err := remindMinutes(in.Params["remind_minutes"])
		if err != nil {
			return ExecResult{}, err
		}
		args := map[string]any{"operation": "add", "title": title, "date_time": at.UTC().Format(time.RFC3339)}
		setArg(args, "description", description)
		if remind > 0 {
			notify := at.Add(-time.Duration(remind * float64(time.Minute)))
			// The time is sent in UTC, so the year that counts is the UTC one.
			if notify.UTC().Year() < 0 {
				return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "the reminder would fall before the year 0")
			}
			args["notification_at"] = notify.UTC().Format(time.RFC3339)
		}
		out, err := callTool(ctx, in, "manage_appointments", args)
		if err != nil {
			return ExecResult{}, err
		}
		if err := requireSuccess(out, "planner tool"); err != nil {
			return ExecResult{}, err
		}
		return ExecResult{Output: map[string]any{"id": outString(out, "id"), "title": title, "date_time": at.Format(time.RFC3339)}}, nil
	}
	return def
}

func todoAddDef(env CatalogEnv) *NodeDef {
	def := plannerDef(TypeTodoAdd, "checkbox", "manage_todos", env)
	def.Params = []ParamSpec{
		{Name: "title", Kind: ParamText, LabelKey: "easydrag.param.title", Required: true, Templatable: true, SensitiveSink: true},
		{Name: "description", Kind: ParamTextarea, LabelKey: "easydrag.param.description", Templatable: true, OutputIndependent: true},
		{Name: "priority", Kind: ParamSelect, LabelKey: "easydrag.param.priority", Default: "medium",
			Options: []Option{option("low", "priority_low"), option("medium", "priority_medium"), option("high", "priority_high")}},
		{Name: "due_date", Kind: ParamDateTime, LabelKey: "easydrag.param.due_date", Templatable: true},
	}
	def.OutputFields = []FieldSpec{{Name: "id", Type: "text", Primary: true}, {Name: "title", Type: "text"}}
	def.Validate = func(n *Node, vc ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		issues := literalIssueIfSet(n, "title", plannerTitle)
		issues = append(issues, literalIssueIfSet(n, "description", plannerDescription)...)
		issues = append(issues, choiceIssue(n, "priority", "medium", todoPriorities)...)
		return append(issues, literalIssueIfSet(n, "due_date", timeCheck(vc.Location, "due_date"))...)
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		title, err := plannerTitle(in.Params["title"])
		if err != nil {
			return ExecResult{}, err
		}
		priority, ok := choiceParam(in.Params["priority"], "medium", todoPriorities...)
		if !ok {
			return ExecResult{}, choiceError("priority", todoPriorities)
		}
		description, err := plannerDescription(in.Params["description"])
		if err != nil {
			return ExecResult{}, err
		}
		due, hasDue, err := plannerTime(in.Params["due_date"], in.Services.Loc(), "due_date")
		if err != nil {
			return ExecResult{}, err
		}
		args := map[string]any{"operation": "add", "title": title, "priority": priority}
		setArg(args, "description", description)
		if hasDue {
			// A date without a time is midnight in the run's zone, in UTC like the others
			// (the planner's own date-only form is midnight UTC).
			args["due_date"] = due.UTC().Format(time.RFC3339)
		}
		out, err := callTool(ctx, in, "manage_todos", args)
		if err != nil {
			return ExecResult{}, err
		}
		if err := requireSuccess(out, "planner tool"); err != nil {
			return ExecResult{}, err
		}
		return ExecResult{Output: map[string]any{"id": outString(out, "id"), "title": title}}, nil
	}
	return def
}
