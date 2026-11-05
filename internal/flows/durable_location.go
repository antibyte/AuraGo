package flows

import (
	"reflect"
	"strings"
	"time"
)

const durableZoneIndependent = "zone_independent"

// durableZoneIndependentGraph rejects every known use of the ambient zone,
// including dynamic conditions that could become date operations at runtime.
func durableZoneIndependentGraph(f *Flow) bool {
	if f == nil {
		return false
	}
	conditionSafe := func(raw any) bool {
		group, err := DecodeConditionGroup(raw)
		if err != nil {
			return false
		}
		for _, row := range group.Rows {
			if strings.Contains(row.Op, "{{") || strings.Contains(row.Type, "{{") || row.Op == "before" || row.Op == "after" || row.Type == "date" {
				return false
			}
		}
		return true
	}
	for _, node := range f.Nodes {
		refs, problems := CollectTemplateRefs(nodeTemplateParams(&node))
		if len(problems) > 0 {
			return false
		}
		for _, ref := range refs {
			for _, filter := range ref.Expr.Filters {
				if filter.Name == "date" {
					return false
				}
			}
		}
		switch node.Type {
		case TypeIf:
			if !conditionSafe(node.Params["condition"]) {
				return false
			}
		case TypeSwitch:
			cases, ok := node.Params["cases"].([]any)
			if !ok {
				return false
			}
			for _, raw := range cases {
				entry, ok := raw.(map[string]any)
				if !ok || !conditionSafe(entry["condition"]) {
					return false
				}
			}
		}
	}
	return true
}

// durableLocation records an actual named location, or an explicit proof that
// the graph does not use local date semantics. A custom zone cannot masquerade
// as an installed IANA zone merely by borrowing its name.
func durableLocation(f *Flow, loc *time.Location) (string, error) {
	if loc == nil {
		loc = time.Local
	}
	name := loc.String()
	if name != "" && name != "Local" {
		loaded, err := time.LoadLocation(name)
		if err == nil && reflect.DeepEqual(loc, loaded) {
			return "iana:" + name, nil
		}
	}
	if durableZoneIndependentGraph(f) {
		return durableZoneIndependent, nil
	}
	return "", ErrDurableInvalidGraph
}

func restoreDurableLocation(f *Flow, policy string) (*time.Location, error) {
	if policy == durableZoneIndependent && durableZoneIndependentGraph(f) {
		return time.UTC, nil
	}
	if strings.HasPrefix(policy, "iana:") {
		name := strings.TrimPrefix(policy, "iana:")
		if name == "" || name == "Local" {
			return nil, errInvalidCheckpoint
		}
		loc, err := time.LoadLocation(name)
		if err == nil {
			return loc, nil
		}
	}
	return nil, errInvalidCheckpoint
}
