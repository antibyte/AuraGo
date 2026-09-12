package gamemaker

import (
	"fmt"
	"slices"
	"strings"
)

// GameplayEvidence is a bounded observation, never a game-authored pass verdict.
// The browser records engine objects alongside the public helper's event trail.
type GameplayEvidence struct {
	Presentation *PresentationEvidence `json:"presentation,omitempty"`
	Outcome      string                `json:"outcome,omitempty"`
	Trace        []ObservedGameEvent   `json:"trace,omitempty"`
	Roles        []ObservedAssetRole   `json:"roles,omitempty"`
	Events       map[string]int        `json:"events,omitempty"`
	Faults       []string              `json:"faults,omitempty"`
	Nodes        []ObservedSceneNode   `json:"nodes,omitempty"`
}

// Sound counts are playback requests, not proof of audible output.
type PresentationEvidence struct {
	Events  map[string]int `json:"events,omitempty"`
	Effects map[string]int `json:"effects,omitempty"`
	Sounds  map[string]int `json:"sounds,omitempty"`
}

type ObservedGameEvent struct {
	Type string  `json:"type"`
	ID   string  `json:"id"`
	At   float64 `json:"at"`
}

type ObservedAssetRole struct {
	Role    string `json:"role"`
	AssetID string `json:"asset_id"`
	Count   int    `json:"count"`
}

type ObservedSceneNode struct {
	ID     string   `json:"id"`
	Active bool     `json:"active"`
	Health *float64 `json:"health,omitempty"`
	X      float64  `json:"x"`
	Y      float64  `json:"y"`
	Z      float64  `json:"z"`
}

func validateGameplayEvidence(e *GameplayEvidence) error {
	if e == nil {
		return nil
	}
	if !slices.Contains([]string{"", "playing", "won", "lost"}, e.Outcome) || len(e.Roles) > 64 || len(e.Events) > 24 || len(e.Faults) > 16 || len(e.Nodes) > 128 || len(e.Trace) > 32 {
		return fmt.Errorf("invalid or oversized gameplay evidence")
	}
	if e.Presentation != nil {
		for _, group := range []map[string]int{e.Presentation.Events, e.Presentation.Effects, e.Presentation.Sounds} {
			if len(group) > 64 {
				return fmt.Errorf("oversized presentation evidence")
			}
			for id, count := range group {
				if len(id) == 0 || len(id) > 64 || count < 0 || count > 1000000 {
					return fmt.Errorf("invalid presentation evidence")
				}
			}
		}
	}
	for _, role := range e.Roles {
		if len(role.Role) == 0 || len(role.Role) > 64 || len(role.AssetID) > 128 || role.Count < 0 || role.Count > 100000 {
			return fmt.Errorf("invalid asset role observation")
		}
	}
	for name, count := range e.Events {
		if len(name) == 0 || len(name) > 64 || count < 0 || count > 1000000 {
			return fmt.Errorf("invalid event observation")
		}
	}
	for _, fault := range e.Faults {
		if len(fault) > 160 {
			return fmt.Errorf("oversized gameplay fault")
		}
	}
	for _, event := range e.Trace {
		if len(event.Type) == 0 || len(event.Type) > 64 || len(event.ID) > 96 || !finite(event.At) || event.At < 0 {
			return fmt.Errorf("invalid gameplay event trace")
		}
	}
	seen := map[string]bool{}
	for _, node := range e.Nodes {
		if len(node.ID) == 0 || len(node.ID) > 96 || seen[node.ID] || !finite(node.X) || !finite(node.Y) || !finite(node.Z) || node.Health != nil && !finite(*node.Health) {
			return fmt.Errorf("invalid scene node observation")
		}
		seen[node.ID] = true
	}
	return nil
}

// Compare only evidence that was actually observed. Missing natural outcomes
// remain unverified; an ESC test is not a natural loss or a complete playthrough.
func compareGameplayEvidence(plan *GamePlan, observations []GameObservation) (string, []CheckResult) {
	status := "unverified"
	checks := []CheckResult{}
	seenChecks := map[string]bool{}
	add := func(id, state, expected, observed string) {
		if seenChecks[id] {
			return
		}
		seenChecks[id] = true
		checks = append(checks, CheckResult{ID: id, Status: state, Expected: expected, Observed: observed})
		if state == "failed" {
			status = "failed"
		}
	}
	roles := map[string]string{}
	if plan != nil {
		for _, role := range plan.Assets {
			id := role.AssetID
			if role.AssemblyID != "" {
				id = role.AssemblyID
			}
			roles[role.Role] = id
		}
	}
	observedRoles := map[string]bool{}
	events := map[string]int{}
	var audited, won, lost bool
	for _, o := range observations {
		e := o.EvidenceAfter
		if e == nil {
			continue
		}
		audited = true
		for _, role := range e.Roles {
			if role.Count == 0 {
				continue
			}
			observedRoles[role.Role] = true
			if expected, ok := roles[role.Role]; ok && expected != "" && expected != role.AssetID {
				add("role_"+role.Role, "failed", expected, role.AssetID)
			}
		}
		for _, fault := range e.Faults {
			add("scene_runtime_"+fmt.Sprint(len(checks)), "failed", "Consistent scene and asset bindings", fault)
		}
		for name, count := range e.Events {
			events[name] = max(events[name], count)
		}
		if e.Outcome == "won" {
			if remaining, ok := o.After["goal_remaining"]; ok && remaining > 0 {
				add("outcome_goal", "failed", "All required goals completed before winning", fmt.Sprintf("won with %g goals remaining", remaining))
			}
			livesEnabled := o.Before["lives"] > 0 || e.Events["lose"] > 0 || plan != nil && plan.Template == "blocks"
			if lives, ok := o.After["lives"]; ok && livesEnabled && lives <= 0 {
				add("outcome_lives", "failed", "A loss cannot be displayed as a victory", fmt.Sprintf("won with %g lives", lives))
			}
		}
		forced := false
		if plan != nil {
			for _, scenario := range plan.Scenarios {
				if scenario.ID == o.ID {
					forced = slices.ContainsFunc(scenario.Steps, func(step GameTestStep) bool { return step.Action == "key" && step.Key == "ESC" })
				}
			}
		}
		if !forced && o.ID != "required_end" && o.ID != "required_restart" {
			won = won || e.Outcome == "won" && e.Events["win"] > 0
			lost = lost || e.Outcome == "lost" && e.Events["lose"] > 0
		}
		if plan != nil && plan.Presentation != nil && e.Presentation != nil {
			if e.Events["pickup"] > 0 && slices.Contains(plan.Presentation.Effects, "pickup-glow") && e.Presentation.Effects["pickup-glow"] == 0 {
				add("effect_pickup", "failed", "Deliver the selected pickup-glow on collection", "Collection observed without the required effect")
			}
		}
		if o.EvidenceBefore != nil && e.Events["hit"] > o.EvidenceBefore.Events["hit"] {
			hits := e.Events["hit"] - o.EvidenceBefore.Events["hit"]
			changedIDs := map[string]bool{}
			for _, before := range o.EvidenceBefore.Nodes {
				for _, after := range e.Nodes {
					if before.ID == after.ID && (before.Active && !after.Active || before.Health != nil && after.Health != nil && *after.Health < *before.Health) {
						changedIDs[before.ID] = true
					}
				}
			}
			traced := 0
			remainingHits := hits
			for i := len(e.Trace) - 1; i >= 0 && remainingHits > 0; i-- {
				event := e.Trace[i]
				if event.Type == "hit" {
					remainingHits--
					if changedIDs[event.ID] {
						traced++
					}
				}
			}
			if traced < hits {
				add("hit_evidence", "unverified", "A hit accompanied by a change to the same target", "Complete target-bound hit evidence was not captured")
			}
			if e.Presentation != nil && o.EvidenceBefore.Presentation != nil {
				feedback := e.Presentation.Events["hit"] - o.EvidenceBefore.Presentation.Events["hit"]
				if feedback > hits {
					add("duplicate_hit_feedback", "failed", "At most one hit event delivery per actual contact", fmt.Sprintf("%d deliveries for %d hits", feedback, hits))
				}
			}
		}
	}
	if audited {
		for role, asset := range roles {
			if asset != "" && !observedRoles[role] {
				add("role_"+role, "unverified", "Observe planned asset role", "Role was not observed in this run")
			}
		}
		if plan != nil && plan.Presentation != nil {
			for _, binding := range plan.Presentation.Sounds {
				if binding.Event != "ambient" && events[binding.Event] == 0 {
					add("event_"+binding.Event, "unverified", "Observe the planned sound event", "Event not reached in the executed scenarios")
				}
			}
		}
	}
	expectedOutcomes := []string{"won", "lost"}
	if plan != nil {
		if raw, ok := plan.Mechanics["outcomes"]; ok {
			expectedOutcomes = nil
			switch values := raw.(type) {
			case []string:
				expectedOutcomes = values
			case []any:
				for _, v := range values {
					if name, ok := v.(string); ok {
						expectedOutcomes = append(expectedOutcomes, name)
					}
				}
			}
		}
	}
	missing := []string{}
	for _, outcome := range expectedOutcomes {
		if outcome == "won" && !won {
			missing = append(missing, "natural win")
		}
		if outcome == "lost" && !lost {
			missing = append(missing, "natural loss")
		}
	}
	if len(missing) > 0 {
		add("natural_outcomes", "unverified", "Observe declared end conditions through gameplay", "Not observed: "+strings.Join(missing, ", "))
	}
	if len(expectedOutcomes) == 0 {
		add("custom_rules", "unverified", "Observe the custom mechanics", "This game declares no built-in end conditions")
	}
	if status != "failed" && audited && len(checks) == 0 {
		status = "passed"
	}
	return status, checks
}
