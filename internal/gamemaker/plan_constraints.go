package gamemaker

import (
	"fmt"
	"slices"
	"strings"
)

// planSelectionConstraints are captured from the user's StartJob request and
// kept only while that job is active. Plans may add custom choices, but may not
// drop any explicit user selection.
type planSelectionConstraints struct {
	assetSelections []AssetSelection
	modelAssetIDs   []string
	presentation    *Presentation
}

func newPlanSelectionConstraints(assets []AssetSelection, models []string, presentation *Presentation) planSelectionConstraints {
	constraints := planSelectionConstraints{
		assetSelections: append([]AssetSelection(nil), assets...),
		modelAssetIDs:   append([]string(nil), models...),
	}
	if presentation != nil {
		copy := *presentation
		copy.Effects = slices.Clone(presentation.Effects)
		copy.Sounds = slices.Clone(presentation.Sounds)
		constraints.presentation = &copy
	}
	return constraints
}

type planSelectionIssue struct {
	field string
	value string
}

// validatePlanSelections runs at the common acceptance boundary after schema,
// asset catalog and view checks. It intentionally checks only explicit user
// selections, leaving other custom plan fields and assets unrestricted.
func (s *Service) validatePlanSelections(jobID string, plan GamePlan, compact bool) error {
	s.mu.RLock()
	constraints := newPlanSelectionConstraints(
		s.planConstraints[jobID].assetSelections,
		s.planConstraints[jobID].modelAssetIDs,
		s.planConstraints[jobID].presentation,
	)
	remaining := max(0, 3-s.planAttempts[jobID])
	s.mu.RUnlock()
	if len(constraints.assetSelections) == 0 && len(constraints.modelAssetIDs) == 0 && constraints.presentation == nil {
		return nil
	}

	var issues []planSelectionIssue
	add := func(field, value string) {
		if !slices.Contains(issues, planSelectionIssue{field: field, value: value}) {
			issues = append(issues, planSelectionIssue{field: field, value: value})
		}
	}
	for _, selected := range constraints.assetSelections {
		if !slices.ContainsFunc(plan.Assets, func(asset PlanAsset) bool {
			return asset.PackID == selected.PackID && asset.AssetID == selected.AssetID
		}) {
			add("assets", selected.PackID+"/"+selected.AssetID)
		}
	}
	for _, id := range constraints.modelAssetIDs {
		if !slices.ContainsFunc(plan.Assets, func(asset PlanAsset) bool {
			return asset.PackID == ModelPackID && asset.AssetID == id
		}) {
			add("assets", id)
		}
	}
	if selection := constraints.presentation; selection != nil {
		wantedFX, wantedAudio, err := resolvePresentation(selection)
		if err != nil {
			return err
		}
		actualFX, actualAudio, err := resolvePresentation(plan.Presentation)
		if err != nil {
			return err // checkPlan already validates this; retain the guard if it changes.
		}
		for _, pair := range [][2][]PresentationAsset{{wantedFX, actualFX}, {wantedAudio, actualAudio}} {
			for _, wanted := range pair[0] {
				if !slices.ContainsFunc(pair[1], func(actual PresentationAsset) bool { return actual.ID == wanted.ID }) {
					add("presentation", wanted.ID)
				}
			}
		}
		for _, binding := range selection.Sounds {
			if plan.Presentation == nil || !slices.Contains(plan.Presentation.Sounds, binding) {
				add("presentation", binding.Event+":"+binding.Sound)
			}
		}
	}
	if len(issues) == 0 {
		return nil
	}

	values := make([]string, 0, len(issues))
	assetValues := make([]string, 0, len(issues))
	presentationValues := make([]string, 0, len(issues))
	for _, issue := range issues {
		values = append(values, issue.value)
		switch issue.field {
		case "assets":
			assetValues = append(assetValues, issue.value)
		case "presentation":
			presentationValues = append(presentationValues, issue.value)
		}
	}
	if !compact {
		return fmt.Errorf("plan: include user-selected asset and presentation IDs: %s", strings.Join(values, ", "))
	}
	structured := make([]DesignIssue, 0, 2)
	if len(assetValues) > 0 {
		structured = append(structured, DesignIssue{
			Path: "design.assets", Code: "selection",
			Message: "include every user-selected asset/model ID in the complete assets array; other custom roles may remain",
			Example: assetValues,
		})
	}
	if len(presentationValues) > 0 {
		structured = append(structured, DesignIssue{
			Path: "design.presentation", Code: "selection",
			Message: "retain the user-selected presentation IDs and exact sound bindings",
			Example: presentationValues,
		})
	}
	return &DesignValidationError{Issues: structured, RemainingAttempts: remaining, RetainedBase: plan.Template}
}
