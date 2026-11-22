package chibisensei

import "math"

type PlanSplit struct {
	Train int `json:"train"`
	Val   int `json:"val"`
	Eval  int `json:"eval"`
}

type Plan struct {
	Preset                string    `json:"preset"`
	ProfileID             string    `json:"profile_id"`
	Method                string    `json:"method"`
	Hyper                 Hyper     `json:"hyper"`
	Split                 PlanSplit `json:"split"`
	EstimateMinutes       int       `json:"estimate_minutes"`
	SessionTimeoutMinutes int       `json:"session_timeout_minutes"`
	DataRev               int64     `json:"data_rev"`
	Trainable             bool      `json:"trainable"`
	ReasonKey             string    `json:"reason_key"`
}

type PlanInput struct {
	Project              Project
	Profile              *Profile
	Rows                 []SplitRow
	Split                SplitResult
	Preset               string // "" = the project's preset
	RemoteTimeoutMinutes int
	NeedBaseGGUF         bool
}

// ComputePlan derives the training plan. Training is not available in P1, so Trainable is false. Overrides that
// do not fit the profile are a *ValidationError.
func ComputePlan(in PlanInput) (Plan, error) {
	p := in.Profile
	if is := ValidateOverrides(in.Project.Overrides, p); len(is) > 0 {
		return Plan{}, &ValidationError{Issues: is}
	}
	preset := in.Preset
	if preset == "" {
		preset = in.Project.Preset
	}
	if preset == "" {
		preset = "balanced"
	}
	method := in.Project.Overrides.Method
	if method == "" {
		method = p.Training.MethodDefault
	}
	if p.Training.LR[method] <= 0 && in.Project.Overrides.LearningRate <= 0 {
		return Plan{}, &ValidationError{Issues: []Issue{badOverride("method", "The profile does not support this method.")}}
	}
	var trainEst []int
	for _, r := range in.Rows {
		if in.Split.Assign[r.ID] == "train" {
			trainEst = append(trainEst, r.EstTokens)
		}
	}
	p95 := Percentile(trainEst, 0.95)
	hyper, evalSamples := PresetHyper(p, preset, method, len(trainEst), p95, in.Project.Overrides)
	var trainTok float64
	for _, e := range trainEst {
		trainTok += float64(min(e, hyper.MaxSeqLen))
	}
	maxNew := p.Eval.MaxNewTokensAssistant
	if in.Project.Recipe == RecipeSpecialist {
		maxNew = p.Eval.MaxNewTokensSpecialist
	}
	samples := min(evalSamples, in.Split.Counts.Eval)
	trainMin := trainTok * float64(hyper.Epochs) / p.Training.T4TokensPerSecond / 60
	if preset == "quick" && trainMin > 30 {
		trainMin = 30
	}
	evalMin := float64(samples) * 2 * float64(maxNew) / p.Training.T4GenTokensPerSecond / 60
	est := 8 + 3 + trainMin + evalMin + 6
	if in.NeedBaseGGUF {
		est += 3
	}
	estimate := int(math.Ceil(est - 1e-9))
	session := int(math.Ceil(1.5*float64(estimate) + 15))
	if session < 30 {
		session = 30
	}
	if in.RemoteTimeoutMinutes > 0 && session > in.RemoteTimeoutMinutes {
		session = in.RemoteTimeoutMinutes
	}
	return Plan{Preset: preset, ProfileID: p.ID, Method: method, Hyper: hyper,
		Split:           PlanSplit{Train: in.Split.Counts.Train, Val: in.Split.Counts.Val, Eval: in.Split.Counts.Eval},
		EstimateMinutes: estimate, SessionTimeoutMinutes: session, DataRev: in.Project.DataRev,
		Trainable: false, ReasonKey: "chibi_sensei.plan.training_later"}, nil
}
