package flows

import (
	"encoding/json"
	"net/url"
	"reflect"
	"slices"
	"strings"

	"aurago/internal/security"
)

// DiffDetail describes one bounded, credential-redacted publication change.
type DiffDetail struct {
	Kind    string `json:"kind"`
	NodeID  string `json:"node_id,omitempty"`
	NodeKey string `json:"node_key,omitempty"`
	Label   string `json:"label,omitempty"`
	Field   string `json:"field,omitempty"`
	Before  any    `json:"before,omitempty"`
	After   any    `json:"after,omitempty"`
}

const maxDiffDetails = 500

func diffText(value string, limit int) string {
	if strings.Contains(strings.ToLower(value), "bearer ") {
		return redactedText
	}
	return truncateRunes(security.RedactSensitiveInfo(security.ScrubSecrets(value)), limit)
}

func diffValue(key string, value any) any {
	value = diffValueDepth(key, value, 0)
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) > 1024 {
		return "[value omitted]"
	}
	return value
}

func diffValueDepth(key string, value any, depth int) any {
	if depth > 8 {
		return "[value omitted]"
	}
	k := strings.ToLower(key)
	if k == "url" || k == "uri" || k == "endpoint" || strings.HasSuffix(k, "_url") {
		if text, ok := value.(string); ok && !HasTemplate(text) {
			if u, err := url.Parse(text); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" {
				return diffText(u.Scheme+"://"+u.Host, 160)
			}
		}
		if value != nil {
			return redactedText
		}
		return nil
	}
	if credentialHeaderName(k) || strings.Contains(k, "credential") || strings.HasSuffix(k, "_key") || k == "headers" {
		if value != nil {
			return redactedText
		}
		return nil
	}
	switch v := value.(type) {
	case string:
		return diffText(v, 160)
	case map[string]any:
		if name, ok := v["name"].(string); ok && (credentialHeaderName(name) || strings.HasSuffix(strings.ToLower(name), "_key")) {
			return redactedText
		}
		out := make(map[string]any, len(v))
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		omitted := len(keys) - 20
		if omitted > 0 {
			keys = keys[:20]
		}
		for _, key := range keys {
			out[diffText(key, 80)] = diffValueDepth(key, v[key], depth+1)
		}
		if omitted > 0 {
			out["[additional entries omitted]"] = omitted
		}
		return out
	case []any:
		omitted := len(v) > 20
		if len(v) > 20 {
			v = v[:20]
		}
		out := make([]any, len(v))
		for i := range v {
			out[i] = diffValueDepth(key, v[i], depth+1)
		}
		if omitted {
			out = append(out, "[additional entries omitted]")
		}
		return out
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return "[value omitted]"
		}
		var decoded any
		if json.Unmarshal(data, &decoded) != nil {
			return "[value omitted]"
		}
		switch decoded.(type) {
		case nil, bool, float64:
			return decoded
		default:
			return diffValueDepth(key, decoded, depth+1)
		}
	}
}

func diffDetails(live, draft *Flow) ([]DiffDetail, bool) {
	if live == nil {
		live = &Flow{}
	}
	if draft == nil {
		draft = &Flow{}
	}
	details := []DiffDetail{}
	truncated := false
	add := func(d DiffDetail) {
		if len(details) < maxDiffDetails {
			details = append(details, d)
		} else {
			truncated = true
		}
	}
	oldNodes := map[string]Node{}
	newNodes := map[string]Node{}
	for _, n := range live.Nodes {
		oldNodes[n.ID] = n
	}
	for _, n := range draft.Nodes {
		newNodes[n.ID] = n
	}
	for _, n := range draft.Nodes {
		old, ok := oldNodes[n.ID]
		base := DiffDetail{NodeID: diffText(n.ID, 64), NodeKey: diffText(n.Key, 80), Label: diffText(n.Label, 160)}
		if !ok {
			base.Kind = "node_added"
			add(base)
			continue
		}
		fields := []struct {
			name          string
			before, after any
		}{
			{"key", old.Key, n.Key}, {"label", old.Label, n.Label}, {"type", old.Type, n.Type},
			{"type_version", old.TypeVersion, n.TypeVersion}, {"settings", old.Settings, n.Settings}, {"parent", old.Parent, n.Parent},
		}
		for _, f := range fields {
			if !reflect.DeepEqual(f.before, f.after) {
				d := base
				d.Kind, d.Field = "node_changed", f.name
				d.Before, d.After = diffValue(f.name, f.before), diffValue(f.name, f.after)
				add(d)
			}
		}
		keys := map[string]bool{}
		for k := range old.Params {
			keys[k] = true
		}
		for k := range n.Params {
			keys[k] = true
		}
		names := make([]string, 0, len(keys))
		for k := range keys {
			names = append(names, k)
		}
		slices.Sort(names)
		for _, k := range names {
			a, aPresent := old.Params[k]
			b, bPresent := n.Params[k]
			if aPresent != bPresent || !reflect.DeepEqual(a, b) {
				d := base
				d.Kind, d.Field = "node_changed", "params."+diffText(k, 80)
				d.Before, d.After = diffValue(k, a), diffValue(k, b)
				if !aPresent {
					d.Before = "[not set]"
				}
				if !bPresent {
					d.After = "[not set]"
				}
				if aPresent && a == nil {
					d.Before = "[null]"
				}
				if bPresent && b == nil {
					d.After = "[null]"
				}
				add(d)
			}
		}
	}
	for _, n := range live.Nodes {
		if _, ok := newNodes[n.ID]; !ok {
			add(DiffDetail{Kind: "node_removed", NodeID: diffText(n.ID, 64), NodeKey: diffText(n.Key, 80), Label: diffText(n.Label, 160)})
		}
	}
	edgeKey := func(e Edge) string { data, _ := json.Marshal([]PortRef{e.Source, e.Target}); return string(data) }
	oldEdges, newEdges := map[string]bool{}, map[string]bool{}
	for _, e := range live.Edges {
		oldEdges[edgeKey(e)] = true
	}
	for _, e := range draft.Edges {
		newEdges[edgeKey(e)] = true
	}
	for _, e := range draft.Edges {
		if !oldEdges[edgeKey(e)] {
			add(DiffDetail{Kind: "edge_added", After: diffValue("edge", []PortRef{e.Source, e.Target})})
		}
	}
	for _, e := range live.Edges {
		if !newEdges[edgeKey(e)] {
			add(DiffDetail{Kind: "edge_removed", Before: diffValue("edge", []PortRef{e.Source, e.Target})})
		}
	}
	for _, f := range []struct {
		name          string
		before, after any
	}{{"name", live.Name, draft.Name}, {"description", live.Description, draft.Description}, {"kind", live.Kind, draft.Kind}, {"settings", live.Settings, draft.Settings}} {
		if !reflect.DeepEqual(f.before, f.after) {
			add(DiffDetail{Kind: "flow_changed", Field: f.name, Before: diffValue(f.name, f.before), After: diffValue(f.name, f.after)})
		}
	}
	return details, truncated
}
