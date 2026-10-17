package decisions

import (
	"strings"
	"time"
)

// ModelAvailability is a passive, detached view of the last catalog check.
type ModelAvailability struct {
	State     string         `json:"state"`
	Code      string         `json:"code,omitempty"`
	CheckedAt time.Time      `json:"checked_at,omitempty"`
	Model     *DecisionModel `json:"model,omitempty"`
}

func (availability ModelAvailability) Snapshot(now time.Time) ModelAvailability {
	if availability.Model != nil {
		models := cloneDecisionModels([]DecisionModel{*availability.Model})
		availability.Model = &models[0]
	}
	if availability.CheckedAt.IsZero() || now.Sub(availability.CheckedAt) >= openRouterModelCacheTTL {
		availability.State, availability.Code = "unknown", CodeModelUnverified
	}
	return availability
}

// CheckModelCatalog shares eligibility with the selector, runtime and passive consumers.
func CheckModelCatalog(models []DecisionModel, id string, checkedAt time.Time) ModelAvailability {
	result := ModelAvailability{State: "blocked", Code: CodeModelNotAvailable, CheckedAt: checkedAt}
	model, ok := findDecisionModel(models, id)
	if !ok {
		return result
	}
	result.Model = &cloneDecisionModels([]DecisionModel{model})[0]
	if !model.CheckedAt.IsZero() {
		result.CheckedAt = model.CheckedAt
	}
	result.Code = model.eligibilityCode()
	result.Model.Usable, result.Model.UnavailableCode = result.Code == "", result.Code
	if result.Code == "" {
		result.State = "usable"
	}
	return result
}

func (model DecisionModel) eligibilityCode() string {
	if !model.SupportsDecisions {
		return CodeModelNotAvailable
	}
	if model.ContextLength <= 0 {
		return CodeModelContextUnverified
	}
	return ""
}

// ResolvedID freezes an alias to the version observed in this catalog snapshot.
func (model DecisionModel) ResolvedID() string {
	if model.AliasTarget != "" {
		return model.AliasTarget
	}
	if model.CanonicalSlug != "" {
		return model.CanonicalSlug
	}
	return model.ID
}

func validDecisionModelID(id string) bool {
	return id != "" && len(id) <= 512 && !strings.ContainsAny(id, " \t\r\n\x00") && strings.TrimSpace(id) == id
}

// DecisionMediaLimits are AuraGo's bounded shipping policy, not upstream capacity claims.
type DecisionMediaLimits struct {
	MaxImages         int   `json:"max_images"`
	MaxImageBytes     int   `json:"max_image_bytes"`
	MaxRequestBytes   int   `json:"max_request_bytes"`
	MinSide           int   `json:"min_side,omitempty"`
	MaxSide           int   `json:"max_side,omitempty"`
	MinPixels         int64 `json:"min_pixels,omitempty"`
	MaxPixels         int64 `json:"max_pixels,omitempty"`
	MaxProcessedBytes int64 `json:"max_processed_bytes,omitempty"`
	MaxPatches        int64 `json:"max_patches,omitempty"`
}

func decisionMediaLimits(id string) DecisionMediaLimits {
	limits := DecisionMediaLimits{MaxImages: 4, MaxImageBytes: 300 << 10, MaxRequestBytes: 3 << 20}
	id = strings.ToLower(id)
	switch {
	case strings.HasPrefix(id, "liquid/d1"):
		limits.MaxImages, limits.MaxImageBytes, limits.MaxRequestBytes, limits.MaxPatches = 8, 3<<20, 4_500_000, 10_000
	case strings.HasPrefix(id, "typesafe/jev-"):
		limits.MaxImages, limits.MaxImageBytes, limits.MaxRequestBytes = 8, 12<<20, 16<<20
		limits.MinSide, limits.MaxSide, limits.MinPixels, limits.MaxPixels, limits.MaxProcessedBytes = 256, 1568, 78400, 2458624, 32<<20
	case strings.HasPrefix(id, "inception/mercury-decide"):
		limits.MaxImages, limits.MaxImageBytes, limits.MaxRequestBytes = 8, 3<<20, 10_000_000
	case strings.HasPrefix(id, "openai/gpt-6-luna-decisions"):
		limits.MaxImages = 8
	}
	return limits
}
