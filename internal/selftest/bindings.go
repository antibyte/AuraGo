package selftest

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (m *Manager) validateBindings(req *Request, inventory []Capability) error {
	if len(req.Bindings) > 100 {
		return fmt.Errorf("too many bindings")
	}
	if req.Profile != "targeted" {
		if len(req.Bindings) > 0 {
			return fmt.Errorf("bindings require targeted profile")
		}
		return nil
	}
	if len(req.Bindings) == 0 {
		return fmt.Errorf("targeted tests require bindings")
	}
	seen := map[string]bool{}
	req.CaseIDs = nil
	req.Targets = nil
	for _, b := range req.Bindings {
		key := b.CaseID + "/" + b.Tool
		if seen[key] || len(b.Arguments) == 0 || len(b.Expected) == 0 || len(b.Expected) > 8 {
			return fmt.Errorf("invalid binding")
		}
		seen[key] = true
		known := false
		for _, t := range m.catalog {
			if t.ID != b.CaseID || t.Profile == "H" || t.Profile == "L" {
				continue
			}
			for _, cap := range inventory {
				if cap.Name == b.Tool && cap.Enabled && cap.Kind == "native" {
					for _, target := range t.Targets {
						if matches(target, cap) {
							known = true
						}
					}
				}
			}
			if (t.Profile == "X" || t.Profile == "W") && !b.AllowEffects {
				return fmt.Errorf("effect grant required")
			}
		}
		if !known {
			return fmt.Errorf("unavailable binding target")
		}
		for key := range b.Expected {
			if key == "" || strings.ContainsAny(key, "[]<>/\\\n\r") {
				return fmt.Errorf("invalid expected field")
			}
		}
		data, _ := json.Marshal(b)
		if len(data) > 8192 {
			return fmt.Errorf("binding too large")
		}
		req.CaseIDs = append(req.CaseIDs, b.CaseID)
		req.Targets = append(req.Targets, b.Tool)
	}
	return nil
}

func hasBinding(req Request, id, tool string) bool {
	for _, b := range req.Bindings {
		if b.CaseID == id && b.Tool == tool {
			return true
		}
	}
	return false
}
