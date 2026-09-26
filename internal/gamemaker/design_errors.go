package gamemaker

import (
	"fmt"
	"slices"
	"strings"
)

type DesignIssue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Example any    `json:"example,omitempty"`
}

type DesignValidationError struct {
	Issues            []DesignIssue `json:"errors"`
	RemainingAttempts int           `json:"remaining_attempts"`
	RetainedBase      string        `json:"retained_base,omitempty"`
}

func (e *DesignValidationError) Error() string {
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, issue.Path+": "+issue.Message)
	}
	return strings.Join(parts, "; ")
}

// Validate independent choices together; canonical plan validation still runs
// afterward, including all cross-field, catalog and gameplay evidence checks.
func (s *Service) designIssues(project Project, d GameDesign) []DesignIssue {
	var issues []DesignIssue
	add := func(path, code, message string, example any) {
		if len(issues) < 8 {
			runes := []rune(message)
			issues = append(issues, DesignIssue{path, code, string(runes[:min(1000, len(runes))]), example})
		}
	}
	if !slices.Contains(templateNames(), d.Base) || is3DTemplate(d.Base) != (project.Dimension == "3d") {
		add("design.base", "dimension", "choose a base matching project dimension ("+project.Dimension+")", ExampleGameDesign(project).Base)
	}
	if strings.TrimSpace(d.Objective) == "" || len(d.Objective) > 2000 {
		add("design.objective", "length", "provide 1–2000 characters", "Reach the forest exit")
	}
	if len(d.Features) < 1 || len(d.Features) > 12 || slices.ContainsFunc(d.Features, func(f string) bool { return strings.TrimSpace(f) == "" }) {
		add("design.features", "items", "list 1–12 nonempty concrete features", []string{"Enemies react to visible targets", "Cover blocks shots"})
	}
	if d.Settings != nil {
		if !guided3D(d.Base) {
			add("design.settings", "unsupported", "goal/speed/duration apply only to fps/exploration/transport/flight/space. Keep base \""+d.Base+"\"; omit settings or send settings:null in the correction. Other fields are retained", map[string]any{"settings": nil})
		} else if err := d.Settings.validate(); err != nil {
			add("design.settings", "range", err.Error(), GameSettings{Goal: 5, Speed: 5, Duration: 120})
		}
	}
	seen := map[string]bool{}
	for i, a := range d.Assets {
		path := fmt.Sprintf("design.assets[%d]", i)
		if strings.TrimSpace(a.Role) == "" || seen[a.Role] {
			add(path+".role", "unique", "use a unique nonempty role; resubmit the complete corrected assets array", "cover_wall")
		}
		seen[a.Role] = true
		if a.PackID != "" {
			if _, err := s.describeAsset(a.PackID, a.AssetID, a.AssemblyID); err != nil {
				add(path, "catalog", err.Error(), "Use exact pack_id and asset_id from one search_assets match")
			}
		}
	}
	if d.Scene != nil {
		if d.Scene.Dimension != project.Dimension {
			add("design.scene.dimension", "dimension", "must match project dimension", project.Dimension)
		} else if err := validateScene(*d.Scene, AssetCatalog{}); err != nil {
			add("design.scene", "invalid", err.Error(), nil)
		}
	}
	if d.Mechanics != nil {
		if err := validateGameMechanics(d.Mechanics); err != nil {
			add("design.mechanics", "invalid", err.Error(), nil)
		}
	}
	// Presentation aliases are normalized by the canonical plan path first.
	return issues
}
