package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

// flowMissionTriggerTypes are the Mission Control trigger types a flows.BindingMission may
// use: the ones MissionManagerV2 registers for a flow mission (ensureFlowTriggersLocked:
// webhook, email, MQTT) and the ones it starts flows for through notifyFlowsLocked (the
// Notify* events and mission_completed). MissionManagerV2.SyncFlowMission checks only the
// node ids, so a spec of any other type would be stored and never fire. Egg, nest and
// planner_operational_issue events start no flows; schedule, datetime and manual are
// separate binding kinds. TestC15CatalogTriggerConfigsDecode keeps the catalog and this
// set in step.
var flowMissionTriggerTypes = map[tools.TriggerType]bool{
	tools.TriggerWebhook:               true,
	tools.TriggerEmailReceived:         true,
	tools.TriggerMQTTMessage:           true,
	tools.TriggerSystemStartup:         true,
	tools.TriggerDeviceConnected:       true,
	tools.TriggerDeviceDisconnected:    true,
	tools.TriggerFritzBoxCall:          true,
	tools.TriggerBudgetWarning:         true,
	tools.TriggerBudgetExceeded:        true,
	tools.TriggerHomeAssistantState:    true,
	tools.TriggerPlannerAppointmentDue: true,
	tools.TriggerPlannerTodoOverdue:    true,
	tools.TriggerMissionCompleted:      true,
}

// flowTriggerSpecs converts published trigger bindings into Mission Control trigger specs.
// A mission binding must name a trigger type Mission Control starts flows for
// (flowMissionTriggerTypes), and its Config must decode into tools.TriggerConfig without
// unknown keys (flowTriggerConfig).
func flowTriggerSpecs(bindings []flows.TriggerBinding) ([]tools.FlowTriggerSpec, error) {
	specs := make([]tools.FlowTriggerSpec, 0, len(bindings))
	for _, b := range bindings {
		node := flowBoundRunes(b.NodeID, flowNameEchoRunes)
		spec := tools.FlowTriggerSpec{NodeID: b.NodeID}
		switch b.Kind {
		case flows.BindingMission:
			trigger := tools.TriggerType(b.MissionTrigger)
			if !flowMissionTriggerTypes[trigger] {
				return nil, fmt.Errorf("trigger %s waits for the event %s, which Mission Control cannot start flows for",
					node, flowQuoteName(b.MissionTrigger))
			}
			cfg, err := flowTriggerConfig(b.Config)
			if err != nil {
				return nil, fmt.Errorf("trigger %s: %w", node, err)
			}
			spec.TriggerType, spec.TriggerConfig = trigger, cfg
		case flows.BindingCron:
			spec.TriggerType, spec.Schedule = tools.FlowTriggerSchedule, b.Schedule
		case flows.BindingTimer:
			spec.TriggerType = tools.FlowTriggerDateTime
		case flows.BindingManual:
			spec.TriggerType = tools.FlowTriggerManual
		default:
			return nil, fmt.Errorf("trigger %s has an unknown binding %s", node, flowQuoteName(string(b.Kind)))
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// flowTriggerConfig decodes the Config of a mission binding into Mission Control's
// TriggerConfig. Every key must be the exact JSON name of a TriggerConfig field: a key
// Mission Control does not know would otherwise be dropped silently, and with it a filter
// the flow's author set. The first unknown key in sorted order is reported, quoted and
// bounded; encoding/json's own check (DisallowUnknownFields) stays on as well, and a value
// of the wrong type names its setting.
func flowTriggerConfig(raw map[string]any) (*tools.TriggerConfig, error) {
	cfg := &tools.TriggerConfig{}
	if len(raw) == 0 {
		return cfg, nil
	}
	known := flowTriggerConfigFields()
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !known[k] {
			return nil, fmt.Errorf("unknown setting %s", flowQuoteName(k))
		}
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("the settings cannot be encoded: %s", flowBoundRunes(err.Error(), flowErrorRunes))
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(cfg); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return nil, fmt.Errorf("the setting %s has the wrong type", flowQuoteName(typeErr.Field))
		}
		return nil, fmt.Errorf("invalid settings: %s", flowBoundRunes(err.Error(), flowErrorRunes))
	}
	return cfg, nil
}

// flowTriggerConfigFields returns the JSON names of the fields of tools.TriggerConfig.
var flowTriggerConfigFields = sync.OnceValue(func() map[string]bool {
	t := reflect.TypeFor[tools.TriggerConfig]()
	out := make(map[string]bool, t.NumField())
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			out[name] = true
		}
	}
	return out
})
