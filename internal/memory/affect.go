package memory

import (
	"math"
	"strings"
	"time"
)

const (
	AffectHalfLife          = 4 * time.Hour
	AffectRestValence       = 0.0
	AffectRestArousal       = 0.35
	AffectDefaultWeight     = 0.30
	AffectRepeatWindow      = 15 * time.Minute
	AffectRepeatWeightScale = 0.40
	AffectLLMValenceDelta   = 0.15
	AffectLLMArousalDelta   = 0.15
)

const (
	AffectCauseConversation           = "conversation"
	AffectCausePositiveFeedback       = "positive_feedback"
	AffectCauseNegativeFeedback       = "negative_feedback"
	AffectCauseUserReturn             = "user_return_after_absence"
	AffectCausePlanCreated            = "plan_created"
	AffectCausePlanAdvanced           = "plan_advanced"
	AffectCausePlanBlocked            = "plan_blocked"
	AffectCausePlanUnblocked          = "plan_unblocked"
	AffectCausePlanCompleted          = "plan_completed"
	AffectCauseToolErrorStreak        = "tool_error_streak"
	AffectCauseToolSuccessStreak      = "tool_success_streak"
	AffectCauseOpsIssueOpened         = "ops_issue_opened"
	AffectCauseOpsIssueResolved       = "ops_issue_resolved"
	AffectCauseAutonomousRunFailed    = "autonomous_run_failed"
	AffectCauseAutonomousRunSucceeded = "autonomous_run_succeeded"
	AffectCauseQuietHours             = "quiet_hours"
)

// AffectState is the Go-owned momentary affect. Traits stay slow; this decays in hours.
type AffectState struct {
	Valence   float64
	Arousal   float64
	Mood      Mood
	CauseCode string
	UpdatedAt time.Time
}

// Active reports whether affect has been driven by a world or conversation event.
func (a AffectState) Active() bool {
	if a.UpdatedAt.IsZero() {
		return false
	}
	if strings.TrimSpace(a.CauseCode) != "" {
		return true
	}
	return math.Abs(a.Valence-AffectRestValence) > 0.05 || math.Abs(a.Arousal-AffectRestArousal) > 0.05
}

// AffectEvent is a typed world or conversation stimulus.
type AffectEvent struct {
	CauseCode string
	Valence   float64
	Arousal   float64
	Weight    float64
	Source    string
	Detail    string
	At        time.Time
}

// RestAffect returns the calm baseline used for decay and first-run state.
func RestAffect(now time.Time) AffectState {
	if now.IsZero() {
		now = time.Now()
	}
	return AffectState{
		Valence:   AffectRestValence,
		Arousal:   AffectRestArousal,
		Mood:      DeriveMoodFromAffect(AffectRestValence, AffectRestArousal),
		UpdatedAt: now,
	}
}

// DeriveMoodFromAffect maps valence/arousal onto the existing mood enum.
func DeriveMoodFromAffect(valence, arousal float64) Mood {
	valence = clampFinite(valence, -1, 1, 0)
	arousal = clampFinite(arousal, 0, 1, AffectRestArousal)
	switch {
	case valence <= -0.35 && arousal >= 0.65:
		return MoodFrustrated
	case valence <= -0.25 && arousal >= 0.50:
		return MoodCautious
	case valence <= -0.20:
		return MoodConcerned
	case valence >= 0.35 && arousal <= 0.40:
		return MoodRelaxed
	case valence >= 0.30 && arousal >= 0.55:
		return MoodPlayful
	case arousal >= 0.50:
		return MoodFocused
	default:
		return MoodCurious
	}
}

// DecayAffect pulls a state toward rest using the configured half-life.
func DecayAffect(state AffectState, now time.Time) AffectState {
	if now.IsZero() {
		now = time.Now()
	}
	if state.UpdatedAt.IsZero() {
		return RestAffect(now)
	}
	elapsed := now.Sub(state.UpdatedAt)
	if elapsed <= 0 {
		state.Mood = DeriveMoodFromAffect(state.Valence, state.Arousal)
		return state
	}
	remain := math.Pow(0.5, elapsed.Seconds()/AffectHalfLife.Seconds())
	state.Valence = AffectRestValence + (state.Valence-AffectRestValence)*remain
	state.Arousal = AffectRestArousal + (state.Arousal-AffectRestArousal)*remain
	state.Mood = DeriveMoodFromAffect(state.Valence, state.Arousal)
	return state
}

// IntegrateAffect decays the current state, then blends in one event with inertia.
func IntegrateAffect(current AffectState, event AffectEvent, now time.Time) AffectState {
	if now.IsZero() {
		now = time.Now()
		if !event.At.IsZero() {
			now = event.At
		}
	}
	current = DecayAffect(current, now)
	weight := event.Weight
	if weight <= 0 {
		weight = AffectDefaultWeight
	}
	if !current.UpdatedAt.IsZero() && current.CauseCode != "" &&
		current.CauseCode == strings.TrimSpace(event.CauseCode) &&
		now.Sub(current.UpdatedAt) < AffectRepeatWindow {
		weight *= AffectRepeatWeightScale
	}
	weight = clampFinite(weight, 0.05, 0.60, AffectDefaultWeight)
	current.Valence = clampFinite(current.Valence*(1-weight)+event.Valence*weight, -1, 1, 0)
	current.Arousal = clampFinite(current.Arousal*(1-weight)+event.Arousal*weight, 0, 1, AffectRestArousal)
	if code := strings.TrimSpace(event.CauseCode); code != "" {
		current.CauseCode = code
	}
	current.UpdatedAt = now
	current.Mood = DeriveMoodFromAffect(current.Valence, current.Arousal)
	return current
}

// BindEmotionStateToAffect lets an LLM narration stay near the Go-owned affect.
func BindEmotionStateToAffect(state *EmotionState, affect AffectState) {
	if state == nil || !affect.Active() {
		return
	}
	state.Valence = clampFinite(state.Valence, affect.Valence-AffectLLMValenceDelta, affect.Valence+AffectLLMValenceDelta, affect.Valence)
	state.Arousal = clampFinite(state.Arousal, affect.Arousal-AffectLLMArousalDelta, affect.Arousal+AffectLLMArousalDelta, affect.Arousal)
	if affect.Mood != "" {
		state.PrimaryMood = affect.Mood
	}
}

// OverlayAffectOnEmotion copies the trusted affect numbers onto a runtime emotion snapshot.
func OverlayAffectOnEmotion(state *EmotionState, affect AffectState) {
	if state == nil || !affect.Active() {
		return
	}
	state.Valence = affect.Valence
	state.Arousal = affect.Arousal
	if affect.Mood != "" {
		state.PrimaryMood = affect.Mood
	}
	if strings.TrimSpace(affect.CauseCode) != "" {
		state.Cause = affect.CauseCode
	}
}

// AffectEventForTrigger maps existing conversation/tool trigger types onto affect pulls.
func AffectEventForTrigger(trigger EmotionTriggerType, detail, source string) (AffectEvent, bool) {
	cause := strings.TrimSpace(string(trigger))
	if cause == "" {
		return AffectEvent{}, false
	}
	event, ok := affectEventByCause(cause)
	if !ok {
		return AffectEvent{}, false
	}
	event.Detail = strings.TrimSpace(detail)
	event.Source = strings.TrimSpace(source)
	return event, true
}

func affectEventByCause(cause string) (AffectEvent, bool) {
	switch cause {
	case AffectCauseConversation:
		return AffectEvent{CauseCode: cause, Valence: 0.00, Arousal: 0.40, Weight: 0.15}, true
	case AffectCausePositiveFeedback:
		return AffectEvent{CauseCode: cause, Valence: 0.55, Arousal: 0.45, Weight: 0.30}, true
	case AffectCauseNegativeFeedback:
		return AffectEvent{CauseCode: cause, Valence: -0.45, Arousal: 0.55, Weight: 0.35}, true
	case AffectCauseUserReturn:
		return AffectEvent{CauseCode: cause, Valence: 0.25, Arousal: 0.40, Weight: 0.30}, true
	case AffectCausePlanCreated:
		return AffectEvent{CauseCode: cause, Valence: 0.15, Arousal: 0.50, Weight: 0.20}, true
	case AffectCausePlanAdvanced:
		return AffectEvent{CauseCode: cause, Valence: 0.20, Arousal: 0.45, Weight: 0.20}, true
	case AffectCausePlanBlocked:
		return AffectEvent{CauseCode: cause, Valence: -0.35, Arousal: 0.55, Weight: 0.30}, true
	case AffectCausePlanUnblocked:
		return AffectEvent{CauseCode: cause, Valence: 0.25, Arousal: 0.45, Weight: 0.25}, true
	case AffectCausePlanCompleted:
		return AffectEvent{CauseCode: cause, Valence: 0.45, Arousal: 0.35, Weight: 0.30}, true
	case AffectCauseToolErrorStreak:
		return AffectEvent{CauseCode: cause, Valence: -0.40, Arousal: 0.70, Weight: 0.35}, true
	case AffectCauseToolSuccessStreak:
		return AffectEvent{CauseCode: cause, Valence: 0.35, Arousal: 0.50, Weight: 0.25}, true
	case AffectCauseOpsIssueOpened:
		return AffectEvent{CauseCode: cause, Valence: -0.50, Arousal: 0.65, Weight: 0.40}, true
	case AffectCauseOpsIssueResolved:
		return AffectEvent{CauseCode: cause, Valence: 0.35, Arousal: 0.35, Weight: 0.30}, true
	case AffectCauseAutonomousRunFailed:
		return AffectEvent{CauseCode: cause, Valence: -0.30, Arousal: 0.55, Weight: 0.25}, true
	case AffectCauseAutonomousRunSucceeded:
		return AffectEvent{CauseCode: cause, Valence: 0.20, Arousal: 0.40, Weight: 0.15}, true
	case AffectCauseQuietHours:
		return AffectEvent{CauseCode: cause, Valence: -0.05, Arousal: 0.20, Weight: 0.20}, true
	default:
		return AffectEvent{}, false
	}
}
