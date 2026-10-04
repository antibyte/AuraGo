package flows

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Trigger node types.
const (
	TypeTriggerManual   = "trigger.manual"
	TypeTriggerSchedule = "trigger.schedule"
	TypeTriggerDateTime = "trigger.datetime"
	TypeTriggerWebhook  = "trigger.webhook"
	TypeTriggerEmail    = "trigger.email"
	TypeTriggerMQTT     = "trigger.mqtt"
	TypeTriggerHAState  = "trigger.ha_state"
	TypeTriggerDevice   = "trigger.device"
	TypeTriggerFritzBox = "trigger.fritzbox_call"
	TypeTriggerPlanner  = "trigger.planner"
	TypeTriggerStartup  = "trigger.startup"
	TypeTriggerBudget   = "trigger.budget"
	TypeTriggerMission  = "trigger.mission_completed"
)

const (
	// maxTriggerRawBytes caps the raw text NormalizeTriggerData parses. It matches
	// maxToolOutputBytes: json.Unmarshal cannot be cancelled and needs a multiple of
	// its input in memory, so text over the cap is never parsed.
	maxTriggerRawBytes = 8 << 20

	// triggerRawKeepBytes is how much of over-long raw text NormalizeTriggerData keeps
	// as "raw". It is the size of a stored output preview.
	triggerRawKeepBytes = storedPreviewBytes

	// maxMinIntervalSeconds bounds min_interval_seconds. Mission Control keeps it as
	// an int, and a value beyond a month only mutes the trigger for good.
	maxMinIntervalSeconds = 30 * 24 * 3600

	// maxCronBytes bounds a cron expression. Real ones have a few dozen characters;
	// the expression is stored in a Mission Control schedule.
	maxCronBytes = 200
)

// BindingKind says how a trigger node is armed.
type BindingKind string

const (
	BindingManual  BindingKind = "manual"
	BindingCron    BindingKind = "cron"
	BindingTimer   BindingKind = "timer"
	BindingMission BindingKind = "mission"
)

// TriggerBinding tells Mission Control or the timer service how to fire a trigger node.
// For BindingMission, MissionTrigger is a Mission Control trigger type and Config uses
// the JSON field names of Mission Control's TriggerConfig.
type TriggerBinding struct {
	NodeID         string         `json:"node_id"`
	Kind           BindingKind    `json:"kind"`
	MissionTrigger string         `json:"mission_trigger,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
	Schedule       string         `json:"schedule,omitempty"`
	FireAt         time.Time      `json:"fire_at,omitzero"`
	Repeat         string         `json:"repeat,omitempty"`
}

// errParamMissing marks binding errors that the required-parameter check already reports.
var errParamMissing = errors.New("a required value is missing")

type triggerType struct {
	def    *NodeDef
	bind   func(n *Node, loc *time.Location, now time.Time) (TriggerBinding, error)
	sample func(n *Node) map[string]any
}

var (
	triggerTableOnce sync.Once
	triggerTable     map[string]triggerType
)

func triggerTypes() map[string]triggerType {
	triggerTableOnce.Do(func() {
		triggerTable = map[string]triggerType{}
		for _, tt := range buildTriggerTypes() {
			triggerTable[tt.def.Type] = tt
		}
	})
	return triggerTable
}

// RegisterTriggerNodes registers the trigger node types.
func RegisterTriggerNodes(reg *Registry) error {
	for _, tt := range buildTriggerTypes() {
		if err := reg.Register(tt.def); err != nil {
			return err
		}
	}
	return nil
}

// newTriggerType builds a trigger definition. The Validate hook runs bind on the raw
// node, so bind sees params of any type (nil, numbers, lists, huge text) and must
// neither panic nor echo them unbounded: error text goes through quoteForError.
func newTriggerType(typ, icon string, untrusted bool, params []ParamSpec,
	bind func(n *Node, loc *time.Location, now time.Time) (TriggerBinding, error),
	sample func(n *Node) map[string]any) triggerType {
	key := strings.ReplaceAll(typ, ".", "_")
	def := &NodeDef{
		Type: typ, Version: 1, Category: "trigger", Icon: icon, Color: "trigger", Trigger: true,
		UntrustedOutput: untrusted, Params: params,
		LabelKey:       "easydrag.node." + key + ".label",
		DescriptionKey: "easydrag.node." + key + ".description",
		SummaryKey:     "easydrag.node." + key + ".summary",
		OutputFields: []FieldSpec{
			{Name: "data", Type: "object", Primary: true}, {Name: "fired_at", Type: "text"},
			{Name: "type", Type: "text"}, {Name: "node", Type: "text"},
		},
	}
	def.Validate = func(n *Node, vc ValidateContext) []Issue {
		now := vc.Now
		if now.IsZero() {
			now = time.Now()
		}
		loc := vc.Location
		if loc == nil {
			loc = time.Local
		}
		if _, err := bind(n, loc, now); err != nil && !errors.Is(err, errParamMissing) {
			return []Issue{{Code: IssueParamInvalid, Severity: SeverityError, NodeID: n.ID, Message: err.Error()}}
		}
		return nil
	}
	return triggerType{def: def, bind: bind, sample: sample}
}

func staticSample(sample map[string]any) func(*Node) map[string]any {
	return func(*Node) map[string]any { return cloneJSONMap(sample) }
}

func cloneJSONMap(m map[string]any) map[string]any {
	data, err := json.Marshal(m)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if json.Unmarshal(data, &out) != nil || out == nil {
		return map[string]any{}
	}
	return out
}

type missionConfigFunc func(p map[string]any) (missionTrigger string, cfg map[string]any, err error)

func missionTriggerType(typ, icon string, untrusted bool, params []ParamSpec, config missionConfigFunc, sample map[string]any) triggerType {
	bind := func(n *Node, _ *time.Location, _ time.Time) (TriggerBinding, error) {
		mt, cfg, err := config(n.Params)
		if err != nil {
			return TriggerBinding{}, err
		}
		if cfg == nil {
			cfg = map[string]any{}
		}
		v, set, err := minIntervalSeconds(n.Params)
		if err != nil {
			return TriggerBinding{}, err
		}
		if set {
			cfg["min_interval_seconds"] = v
		}
		return TriggerBinding{NodeID: n.ID, Kind: BindingMission, MissionTrigger: mt, Config: cfg}, nil
	}
	return newTriggerType(typ, icon, untrusted, params, bind, staticSample(sample))
}

// minIntervalSeconds reads the min_interval_seconds parameter as whole seconds. set is
// false when the value is missing, not a number or not positive. A value over
// maxMinIntervalSeconds is an error (Mission Control stores the field as an int, and
// converting a huge float to int is implementation-defined).
func minIntervalSeconds(p map[string]any) (secs float64, set bool, err error) {
	v, ok := toNumber(p["min_interval_seconds"])
	if !ok || v <= 0 {
		return 0, false, nil
	}
	if v > maxMinIntervalSeconds {
		return 0, false, fmt.Errorf("min_interval_seconds can be at most %d", maxMinIntervalSeconds)
	}
	return math.Floor(v), true, nil
}

func textParam(p map[string]any, key string) string { return strings.TrimSpace(Stringify(p[key])) }

func choose(p map[string]any, key, def string, allowed ...string) (string, error) {
	v := textParam(p, key)
	if v == "" {
		return def, nil
	}
	for _, a := range allowed {
		if v == a {
			return v, nil
		}
	}
	return "", fmt.Errorf("%s has an unknown value %s", key, quoteForError(v))
}

const sampleTime = "2026-10-03T07:00:00Z"

func buildTriggerTypes() []triggerType {
	minInterval := ParamSpec{Name: "min_interval_seconds", Kind: ParamNumber, LabelKey: "easydrag.param.min_interval_seconds",
		HelpKey: "easydrag.help.min_interval_seconds", Default: 0.0}
	return []triggerType{
		newTriggerType(TypeTriggerManual, "hand-click", false,
			[]ParamSpec{{Name: "data", Kind: ParamJSON, LabelKey: "easydrag.param.manual_data", HelpKey: "easydrag.help.manual_data"}},
			func(n *Node, _ *time.Location, _ time.Time) (TriggerBinding, error) {
				if raw, ok := n.Params["data"]; ok && raw != nil {
					if _, isMap := raw.(map[string]any); !isMap {
						return TriggerBinding{}, errors.New("the sample data must be a JSON object")
					}
				}
				return TriggerBinding{NodeID: n.ID, Kind: BindingManual}, nil
			},
			func(n *Node) map[string]any {
				if m, ok := n.Params["data"].(map[string]any); ok {
					return cloneJSONMap(m)
				}
				return map[string]any{}
			}),
		newTriggerType(TypeTriggerSchedule, "clock", false, scheduleParams(),
			func(n *Node, _ *time.Location, _ time.Time) (TriggerBinding, error) {
				expr, err := ScheduleToCron(n.Params)
				if err != nil {
					return TriggerBinding{}, err
				}
				return TriggerBinding{NodeID: n.ID, Kind: BindingCron, Schedule: expr}, nil
			}, staticSample(map[string]any{})),
		newTriggerType(TypeTriggerDateTime, "calendar-time", false, []ParamSpec{
			{Name: "at", Kind: ParamDateTime, LabelKey: "easydrag.param.datetime_at", Required: true},
			{Name: "repeat", Kind: ParamSegmented, LabelKey: "easydrag.param.datetime_repeat", Default: "none",
				Options: []Option{option("none", "repeat_none"), option(RepeatYearly, "repeat_yearly")}},
		}, bindDateTime, staticSample(map[string]any{"scheduled_for": "2026-10-05T07:00:00Z"})),
		missionTriggerType(TypeTriggerWebhook, "webhook", true, []ParamSpec{
			{Name: "webhook", Kind: ParamSelect, LabelKey: "easydrag.param.webhook", Required: true, OptionsSource: "webhooks"},
			minInterval,
		}, func(p map[string]any) (string, map[string]any, error) {
			id := textParam(p, "webhook")
			if id == "" {
				return "", nil, fmt.Errorf("%w: webhook", errParamMissing)
			}
			return "webhook", map[string]any{"webhook_id": id}, nil
		}, map[string]any{"raw": `{"text":"Hallo"}`, "payload": map[string]any{"text": "Hallo"}}),
		missionTriggerType(TypeTriggerEmail, "mail", true, []ParamSpec{
			{Name: "folder", Kind: ParamText, LabelKey: "easydrag.param.email_folder", Default: "INBOX"},
			{Name: "subject_contains", Kind: ParamText, LabelKey: "easydrag.param.email_subject_contains"},
			{Name: "from_contains", Kind: ParamText, LabelKey: "easydrag.param.email_from_contains"},
			minInterval,
		}, func(p map[string]any) (string, map[string]any, error) {
			return "email_received", map[string]any{
				"email_folder":           textParam(p, "folder"),
				"email_subject_contains": textParam(p, "subject_contains"),
				"email_from_contains":    textParam(p, "from_contains"),
			}, nil
		}, map[string]any{"subject": "Rechnung Oktober", "from": "shop@example.com", "body": "Hallo, anbei die Rechnung."}),
		missionTriggerType(TypeTriggerMQTT, "broadcast", true, []ParamSpec{
			{Name: "topic", Kind: ParamText, LabelKey: "easydrag.param.mqtt_topic", Required: true},
			{Name: "payload_contains", Kind: ParamText, LabelKey: "easydrag.param.mqtt_payload_contains"},
			minInterval,
		}, func(p map[string]any) (string, map[string]any, error) {
			topic := textParam(p, "topic")
			if topic == "" {
				return "", nil, fmt.Errorf("%w: topic", errParamMissing)
			}
			cfg := map[string]any{"mqtt_topic": topic, "mqtt_payload_contains": textParam(p, "payload_contains")}
			v, set, err := minIntervalSeconds(p)
			if err != nil {
				return "", nil, err
			}
			if set {
				cfg["mqtt_min_interval_seconds"] = v
			}
			return "mqtt_message", cfg, nil
		}, map[string]any{"topic": "home/door/state", "payload": `{"open":true}`, "json": map[string]any{"open": true}}),
		missionTriggerType(TypeTriggerHAState, "home-signal", false, []ParamSpec{
			{Name: "entity", Kind: ParamSelect, LabelKey: "easydrag.param.ha_entity", Required: true, OptionsSource: "ha_entities"},
			{Name: "state_equals", Kind: ParamText, LabelKey: "easydrag.param.ha_state_equals"},
			minInterval,
		}, func(p map[string]any) (string, map[string]any, error) {
			entity := textParam(p, "entity")
			if entity == "" {
				return "", nil, fmt.Errorf("%w: entity", errParamMissing)
			}
			return "home_assistant_state", map[string]any{"ha_entity_id": entity, "ha_state_equals": textParam(p, "state_equals")}, nil
		}, map[string]any{"entity_id": "binary_sensor.door", "new_state": "on", "old_state": "off", "time": sampleTime}),
		missionTriggerType(TypeTriggerDevice, "device-desktop", false, []ParamSpec{
			{Name: "event", Kind: ParamSegmented, LabelKey: "easydrag.param.device_event", Default: "connected",
				Options: []Option{option("connected", "device_connected"), option("disconnected", "device_disconnected")}},
			{Name: "device_id", Kind: ParamText, LabelKey: "easydrag.param.device_id"},
			minInterval,
		}, func(p map[string]any) (string, map[string]any, error) {
			event, err := choose(p, "event", "connected", "connected", "disconnected")
			if err != nil {
				return "", nil, err
			}
			return "device_" + event, map[string]any{"device_id": textParam(p, "device_id")}, nil
		}, map[string]any{"event": "device_connected", "device_id": "dev-1", "device_name": "Laptop", "time": sampleTime}),
		// The call summary carries the caller's number and display name, which the caller
		// (or the network) chooses, so the output is untrusted.
		missionTriggerType(TypeTriggerFritzBox, "phone-incoming", true, []ParamSpec{
			{Name: "call_type", Kind: ParamSelect, LabelKey: "easydrag.param.call_type", Default: "any",
				Options: []Option{option("any", "call_any"), option("call", "call_call"), option("tam_message", "call_tam")}},
			minInterval,
		}, func(p map[string]any) (string, map[string]any, error) {
			ct, err := choose(p, "call_type", "any", "any", "call", "tam_message")
			if err != nil {
				return "", nil, err
			}
			if ct == "any" {
				ct = ""
			}
			return "fritzbox_call", map[string]any{"call_type": ct}, nil
		}, map[string]any{"call_type": "call", "summary": "Anruf von 030 1234567", "time": sampleTime}),
		missionTriggerType(TypeTriggerPlanner, "calendar-event", false, []ParamSpec{
			{Name: "event", Kind: ParamSegmented, LabelKey: "easydrag.param.planner_event", Default: "appointment_due",
				Options: []Option{option("appointment_due", "planner_appointment_due"), option("todo_overdue", "planner_todo_overdue")}},
			{Name: "title_contains", Kind: ParamText, LabelKey: "easydrag.param.planner_title_contains"},
			minInterval,
		}, func(p map[string]any) (string, map[string]any, error) {
			event, err := choose(p, "event", "appointment_due", "appointment_due", "todo_overdue")
			if err != nil {
				return "", nil, err
			}
			return "planner_" + event, map[string]any{"planner_title_contains": textParam(p, "title_contains")}, nil
		}, map[string]any{"appointment_id": "apt-1", "title": "Zahnarzt", "date_time": "2026-10-05T09:00:00Z", "time": sampleTime}),
		missionTriggerType(TypeTriggerStartup, "power", false, nil,
			func(map[string]any) (string, map[string]any, error) { return "system_startup", map[string]any{}, nil },
			map[string]any{"event": "system_startup", "time": sampleTime}),
		missionTriggerType(TypeTriggerBudget, "coin", false, []ParamSpec{
			{Name: "event", Kind: ParamSegmented, LabelKey: "easydrag.param.budget_event", Default: "warning",
				Options: []Option{option("warning", "budget_warning"), option("exceeded", "budget_exceeded")}},
		}, func(p map[string]any) (string, map[string]any, error) {
			event, err := choose(p, "event", "warning", "warning", "exceeded")
			if err != nil {
				return "", nil, err
			}
			return "budget_" + event, map[string]any{}, nil
		}, map[string]any{"event": "budget_warning", "spent_usd": 4.2, "limit_usd": 5.0, "percentage": 84.0, "time": sampleTime}),
		// The output of the source mission is text an agent produced, possibly from web
		// pages or mail, so it is as untrusted as the data the agent read.
		missionTriggerType(TypeTriggerMission, "checks", true, []ParamSpec{
			{Name: "source", Kind: ParamSelect, LabelKey: "easydrag.param.source_mission", Required: true, OptionsSource: "missions"},
			{Name: "require_success", Kind: ParamBool, LabelKey: "easydrag.param.require_success", Default: true},
			minInterval,
		}, func(p map[string]any) (string, map[string]any, error) {
			source := textParam(p, "source")
			if source == "" {
				return "", nil, fmt.Errorf("%w: source", errParamMissing)
			}
			requireSuccess := true
			if v, ok := p["require_success"]; ok {
				requireSuccess = truthy(v)
			}
			return "mission_completed", map[string]any{"source_mission_id": source, "require_success": requireSuccess}, nil
		}, map[string]any{"source_mission": "mission_123", "result": "success", "output": "Fertig.", "outputs": map[string]any{}}),
	}
}

// bindDateTime arms a Date/Time trigger. A yearly date uses nextYearly, like the timer
// service when it re-arms, so a Feb 29 date fires only in leap years instead of drifting
// to Mar 1.
func bindDateTime(n *Node, loc *time.Location, now time.Time) (TriggerBinding, error) {
	raw := textParam(n.Params, "at")
	if raw == "" {
		return TriggerBinding{}, fmt.Errorf("%w: at", errParamMissing)
	}
	at, ok := toTime(raw, loc)
	if !ok {
		return TriggerBinding{}, fmt.Errorf("%s is not a date and time", quoteForError(raw))
	}
	if textParam(n.Params, "repeat") == RepeatYearly {
		return TriggerBinding{NodeID: n.ID, Kind: BindingTimer, FireAt: nextYearly(at, now), Repeat: RepeatYearly}, nil
	}
	if !at.After(now) {
		return TriggerBinding{}, fmt.Errorf("the time %s is in the past", at.Format("2006-01-02 15:04"))
	}
	return TriggerBinding{NodeID: n.ID, Kind: BindingTimer, FireAt: at}, nil
}

var weekdayNumbers = []struct {
	key string
	num int
}{{"mon", 1}, {"tue", 2}, {"wed", 3}, {"thu", 4}, {"fri", 5}, {"sat", 6}, {"sun", 0}}

func scheduleParams() []ParamSpec {
	when := func(modes ...string) *Visibility { return &Visibility{Param: "mode", Equals: modes} }
	days := make([]Option, 0, len(weekdayNumbers))
	for _, d := range weekdayNumbers {
		days = append(days, option(d.key, "weekday_"+d.key))
	}
	return []ParamSpec{
		{Name: "mode", Kind: ParamSelect, LabelKey: "easydrag.param.schedule_mode", Default: "daily", Options: []Option{
			option("interval_minutes", "schedule_interval_minutes"), option("interval_hours", "schedule_interval_hours"),
			option("daily", "schedule_daily"), option("weekdays", "schedule_weekdays"), option("weekly", "schedule_weekly"),
			option("monthly", "schedule_monthly"), option("cron", "schedule_cron")}},
		{Name: "minutes", Kind: ParamNumber, LabelKey: "easydrag.param.schedule_minutes", Required: true, Default: 15.0, VisibleIf: when("interval_minutes")},
		{Name: "hours", Kind: ParamNumber, LabelKey: "easydrag.param.schedule_hours", Required: true, Default: 1.0, VisibleIf: when("interval_hours")},
		{Name: "time", Kind: ParamText, LabelKey: "easydrag.param.schedule_time", Required: true, Default: "07:00", VisibleIf: when("daily", "weekdays", "weekly", "monthly")},
		{Name: "weekdays", Kind: ParamMultiSelect, LabelKey: "easydrag.param.schedule_weekdays", Required: true, Options: days, VisibleIf: when("weekly")},
		{Name: "day", Kind: ParamNumber, LabelKey: "easydrag.param.schedule_day", Required: true, Default: 1.0, VisibleIf: when("monthly")},
		{Name: "cron", Kind: ParamCron, LabelKey: "easydrag.param.schedule_cron", Required: true, VisibleIf: when("cron")},
	}
}

// ScheduleToCron converts schedule parameters into a cron expression. Missing values fall
// back to the parameter defaults (daily at 07:00, every 15 minutes, every hour, day 1).
// The parameters can be of any type; every echo of them in an error is cut.
func ScheduleToCron(p map[string]any) (string, error) {
	mode := textParam(p, "mode")
	if mode == "" {
		mode = "daily"
	}
	number := func(key string, def, lo, hi int) (int, error) {
		v, present := p[key]
		if !present || isEmptyValue(v) {
			return def, nil
		}
		// Compare as floats: converting a huge float to int is implementation-defined.
		f, ok := toNumber(v)
		if !ok || f != math.Trunc(f) || f < float64(lo) || f > float64(hi) {
			return 0, fmt.Errorf("%s must be a whole number from %d to %d", key, lo, hi)
		}
		return int(f), nil
	}
	switch mode {
	case "interval_minutes":
		n, err := number("minutes", 15, 1, 59)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("*/%d * * * *", n), nil
	case "interval_hours":
		n, err := number("hours", 1, 1, 23)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("0 */%d * * *", n), nil
	case "daily", "weekdays", "weekly", "monthly":
		clock := textParam(p, "time")
		if clock == "" {
			clock = "07:00"
		}
		m := clockPattern.FindStringSubmatch(clock)
		if m == nil {
			return "", fmt.Errorf("time must use the format HH:MM, got %s", quoteForError(clock))
		}
		hour, _ := strconv.Atoi(m[1])
		minute, _ := strconv.Atoi(m[2])
		switch mode {
		case "daily":
			return fmt.Sprintf("%d %d * * *", minute, hour), nil
		case "weekdays":
			return fmt.Sprintf("%d %d * * 1-5", minute, hour), nil
		case "weekly":
			days, err := weekdayList(p["weekdays"])
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d %d * * %s", minute, hour, days), nil
		default:
			day, err := number("day", 1, 1, 31)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d %d %d * *", minute, hour, day), nil
		}
	case "cron":
		text := textParam(p, "cron")
		if len(text) > maxCronBytes {
			return "", fmt.Errorf("a cron expression can be at most %d characters", maxCronBytes)
		}
		expr := strings.Join(strings.Fields(text), " ")
		if expr == "" {
			return "", fmt.Errorf("%w: cron", errParamMissing)
		}
		if n := len(strings.Fields(expr)); n != 5 && n != 6 {
			return "", fmt.Errorf("a cron expression needs 5 or 6 fields, got %d", n)
		}
		return expr, nil
	}
	return "", fmt.Errorf("unknown schedule mode %s", quoteForError(mode))
}

// weekdayList turns the weekdays parameter (a list of mon..sun) into a cron day list
// in Monday-to-Sunday order. An empty value counts as missing; anything else that is
// not a list of known names is an error that names the first offender.
func weekdayList(v any) (string, error) {
	if isEmptyValue(v) {
		return "", fmt.Errorf("%w: weekdays", errParamMissing)
	}
	list, ok := v.([]any)
	if !ok {
		return "", errors.New("weekdays must be a list of weekday names")
	}
	selected := map[string]bool{}
	for _, item := range list {
		name := strings.ToLower(Stringify(item))
		known := false
		for _, d := range weekdayNumbers {
			if d.key == name {
				known = true
				break
			}
		}
		if !known {
			return "", fmt.Errorf("unknown weekday %s", quoteForError(name))
		}
		selected[name] = true
	}
	var parts []string
	for _, d := range weekdayNumbers {
		if selected[d.key] {
			parts = append(parts, strconv.Itoa(d.num))
		}
	}
	return strings.Join(parts, ","), nil
}

// BindTriggers returns the bindings of all enabled trigger nodes in document order.
func BindTriggers(f *Flow, reg *Registry, loc *time.Location, now time.Time) ([]TriggerBinding, error) {
	if f == nil {
		return nil, nil
	}
	if reg == nil {
		return nil, errors.New("no node registry to bind triggers with")
	}
	if loc == nil {
		loc = time.Local
	}
	table := triggerTypes()
	var out []TriggerBinding
	for i := range f.Nodes {
		n := &f.Nodes[i]
		if n.Settings.Disabled {
			continue
		}
		def, ok := reg.Lookup(n.Type)
		if !ok || !def.Trigger {
			continue
		}
		tt, ok := table[n.Type]
		if !ok {
			continue
		}
		b, err := tt.bind(n, loc, now)
		if err != nil {
			return nil, fmt.Errorf("trigger %s: %w", echoKey(n.Key), err)
		}
		out = append(out, b)
	}
	return out, nil
}

// TriggerSample returns example trigger data for test runs and the editor's data tree.
func TriggerSample(n *Node) map[string]any {
	if n == nil {
		return map[string]any{}
	}
	if tt, ok := triggerTypes()[n.Type]; ok {
		return tt.sample(n)
	}
	return map[string]any{}
}

// NormalizeTriggerData turns raw Mission Control trigger data into a trigger's "data" object.
// Webhooks keep the raw text and a parsed "payload"; MQTT payloads that are JSON get a "json" field.
//
// raw is attacker-controlled text (a webhook body, a mail, an MQTT payload). The caller
// bounds request bodies, as the webhook handler does with its payload limit; this
// function is the second line of defence, because json.Unmarshal cannot be cancelled and
// needs a multiple of its input in memory. Text over maxTriggerRawBytes (8 MiB, the cap
// callTool applies to tool output) is not parsed: the result then holds the first
// triggerRawKeepBytes (64 KiB, cut at a rune boundary) as "raw" and "truncated": true,
// plus "payload": nil for a webhook. The flag is absent otherwise. Text is made valid
// UTF-8 before it is kept or parsed.
//
// The engine still replaces trigger data whose encoding exceeds MaxOutputBytes with {},
// and for a webhook "raw" and "payload" together are about twice the body size, so
// bodies above roughly 2.5 MiB are lost to the engine even though they are parsed here.
func NormalizeTriggerData(kind, raw string) map[string]any {
	if len(raw) > maxTriggerRawBytes {
		return truncatedTriggerData(kind, raw)
	}
	raw = validUTF8(strings.TrimSpace(raw))
	if raw == "" {
		return map[string]any{}
	}
	if kind == "webhook" {
		out := map[string]any{"raw": raw}
		var v any
		if json.Unmarshal([]byte(raw), &v) == nil {
			out["payload"] = v
		} else {
			out["payload"] = raw
		}
		return out
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m == nil {
		return map[string]any{"raw": raw}
	}
	if kind == "mqtt" || kind == "mqtt_message" {
		if payload, ok := m["payload"].(string); ok {
			var v any
			if json.Unmarshal([]byte(payload), &v) == nil {
				m["json"] = v
			}
		}
	}
	return m
}

// truncatedTriggerData is NormalizeTriggerData for text over maxTriggerRawBytes. It
// reads only the first triggerRawKeepBytes of raw.
func truncatedTriggerData(kind, raw string) map[string]any {
	cut := triggerRawKeepBytes
	for cut > 0 && !utf8.RuneStart(raw[cut]) {
		cut--
	}
	out := map[string]any{"raw": validUTF8(strings.TrimSpace(raw[:cut])), "truncated": true}
	if kind == "webhook" {
		out["payload"] = nil
	}
	return out
}
