package chibisensei

import (
	"fmt"
	"regexp"
	"time"

	"aurago/internal/chibisensei/trainer"
)

const (
	JobSchemaVersion = 1
	LogPrefix        = "@@chibi"
	OutputPrefix     = "chibi-"
	jobLRScheduler   = "cosine"
	jobWarmupRatio   = 0.03
	jobWeightDecay   = 0.0
	jobSeed          = 42
	jobMetricEvery   = 10
)

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// JobSpec is job.json, schema v1; the field order is the spec's.
type JobSpec struct {
	V             int        `json:"v"`
	RunID         string     `json:"run_id"`
	Created       string     `json:"created"`
	AuraGoVersion string     `json:"aurago_version"`
	Profile       *Profile   `json:"profile"`
	ProfileSHA    string     `json:"profile_sha"`
	Recipe        string     `json:"recipe"`
	TaskKind      *string    `json:"task_kind"` // null unless specialist
	Labels        []string   `json:"labels"`
	Method        string     `json:"method"`
	Data          JobData    `json:"data"`
	Hyper         JobHyper   `json:"hyper"`
	Budget        JobBudget  `json:"budget"`
	Outputs       JobOutputs `json:"outputs"`
	Deps          JobDeps    `json:"deps"`
	Log           JobLog     `json:"log"`
}

type JobData struct {
	Marker string `json:"marker"` // exact dataset file name of the marker
	Format string `json:"format"` // messages | text
}

type JobHyper struct {
	Epochs                int      `json:"epochs"`
	LearningRate          float64  `json:"learning_rate"`
	LRScheduler           string   `json:"lr_scheduler"`
	WarmupRatio           float64  `json:"warmup_ratio"`
	WeightDecay           float64  `json:"weight_decay"`
	PerDeviceBatch        int      `json:"per_device_batch"`
	GradAccum             int      `json:"grad_accum"`
	MaxSeqLen             int      `json:"max_seq_len"`
	Seed                  int      `json:"seed"`
	GradientCheckpointing bool     `json:"gradient_checkpointing"`
	LossOn                string   `json:"loss_on"` // assistant | all
	LoRA                  *JobLoRA `json:"lora,omitempty"`
}

type JobLoRA struct {
	R             int      `json:"r"`
	Alpha         int      `json:"alpha"`
	Dropout       float64  `json:"dropout"`
	TargetModules []string `json:"target_modules"`
}

type JobBudget struct {
	SessionTimeoutS int `json:"session_timeout_s"`
	MaxTrainS       int `json:"max_train_s"`
	EvalMaxSamples  int `json:"eval_max_samples"`
	MaxNewTokens    int `json:"max_new_tokens"`
}

type JobOutputs struct {
	EmitBaseGGUF bool   `json:"emit_base_gguf"`
	GGUFOutType  string `json:"gguf_outtype"`
	Prefix       string `json:"prefix"`
}

type JobDeps struct {
	Pip          []string    `json:"pip"`
	Transformers string      `json:"transformers"`
	LlamaCpp     JobLlamaCpp `json:"llama_cpp"`
}

type JobLlamaCpp struct {
	Tag    string `json:"tag"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type JobLog struct {
	Prefix           string `json:"prefix"`
	MetricEverySteps int    `json:"metric_every_steps"`
}

type JobInput struct {
	RunID         string
	Created       time.Time
	AuraGoVersion string
	Project       Project
	Profile       *Profile
	ProfileSHA    string
	Plan          Plan
	Marker        string // file name of the dataset marker
	Format        string // messages | text
	Lock          trainer.Lock
}

func jobErr(format string, a ...any) error { return fmt.Errorf("job spec: "+format, a...) }

// BuildJobSpec builds job.json from a computed plan. Hyper-parameters come from the plan; scheduler, warmup,
// weight decay and seed are the fixed values of the spec; deps are the lock plus the profile's extra pins.
func BuildJobSpec(in JobInput) (JobSpec, error) {
	p, pl := in.Profile, in.Plan
	switch {
	case p == nil:
		return JobSpec{}, jobErr("profile required")
	case !ValidID(in.RunID):
		return JobSpec{}, jobErr("invalid run id")
	case in.Created.IsZero():
		return JobSpec{}, jobErr("created time required")
	case in.AuraGoVersion == "" || len(in.AuraGoVersion) > 64:
		return JobSpec{}, jobErr("aurago version must have 1 to 64 characters")
	case !sha256Hex.MatchString(in.ProfileSHA):
		return JobSpec{}, jobErr("profile sha must be 64 hex characters")
	case !pl.Trainable:
		return JobSpec{}, jobErr("plan is not trainable")
	case pl.ProfileID != p.ID:
		return JobSpec{}, jobErr("plan is for profile %q, not %q", pl.ProfileID, p.ID)
	case !containsString(p.Training.Methods, pl.Method):
		return JobSpec{}, jobErr("profile does not support method %q", pl.Method)
	case (pl.Method == "lora") != (pl.Hyper.LoRA != nil):
		return JobSpec{}, jobErr("LoRA hyper-parameters must be set exactly for method lora")
	case pl.Hyper.Epochs < 1 || pl.Hyper.PerDeviceBatch < 1 || pl.Hyper.GradAccum < 1 ||
		pl.Hyper.MaxSeqLen < 256 || pl.Hyper.LearningRate <= 0:
		return JobSpec{}, jobErr("plan hyper-parameters out of range")
	case pl.SessionTimeoutMinutes < 1 || pl.MaxTrainSeconds < minMaxTrainSeconds || pl.MaxNewTokens < 1 || pl.EvalSamples < 0:
		return JobSpec{}, jobErr("plan budget out of range")
	case in.Marker != MarkerFileName(in.RunID):
		return JobSpec{}, jobErr("marker must be %s", MarkerFileName(in.RunID))
	}
	recipe := in.Project.Recipe
	format := FormatFor(recipe)
	if format == "" {
		return JobSpec{}, jobErr("unknown recipe %q", recipe)
	}
	if in.Format != format {
		return JobSpec{}, jobErr("format %q does not fit recipe %s", in.Format, recipe)
	}
	var taskKind *string
	if recipe == RecipeSpecialist {
		if !validTaskKind(in.Project.TaskKind) {
			return JobSpec{}, jobErr("a specialist needs a task kind")
		}
		tk := in.Project.TaskKind
		taskKind = &tk
	} else if in.Project.TaskKind != "" {
		return JobSpec{}, jobErr("only a specialist has a task kind")
	}
	deps := in.Lock
	deps.Pip = append(append([]string{}, in.Lock.Pip...), p.Training.ExtraPip...)
	if err := deps.Validate(); err != nil {
		return JobSpec{}, jobErr("dependencies: %w", err)
	}
	labels := append([]string{}, in.Project.Labels...)
	lossOn := "assistant"
	if recipe == RecipeFree {
		lossOn = "all"
	}
	prof := *p
	spec := JobSpec{
		V: JobSchemaVersion, RunID: in.RunID, Created: in.Created.UTC().Format(time.RFC3339),
		AuraGoVersion: in.AuraGoVersion, Profile: &prof, ProfileSHA: in.ProfileSHA,
		Recipe: recipe, TaskKind: taskKind, Labels: labels, Method: pl.Method,
		Data: JobData{Marker: in.Marker, Format: format},
		Hyper: JobHyper{Epochs: pl.Hyper.Epochs, LearningRate: pl.Hyper.LearningRate, LRScheduler: jobLRScheduler,
			WarmupRatio: jobWarmupRatio, WeightDecay: jobWeightDecay, PerDeviceBatch: pl.Hyper.PerDeviceBatch,
			GradAccum: pl.Hyper.GradAccum, MaxSeqLen: pl.Hyper.MaxSeqLen, Seed: jobSeed,
			GradientCheckpointing: p.Training.GradCheckpointing, LossOn: lossOn},
		Budget: JobBudget{SessionTimeoutS: pl.SessionTimeoutMinutes * 60, MaxTrainS: pl.MaxTrainSeconds,
			EvalMaxSamples: pl.EvalSamples, MaxNewTokens: pl.MaxNewTokens},
		Outputs: JobOutputs{EmitBaseGGUF: pl.NeedBaseGGUF, GGUFOutType: p.GGUF.OutType, Prefix: OutputPrefix},
		Deps: JobDeps{Pip: deps.Pip, Transformers: deps.Transformers,
			LlamaCpp: JobLlamaCpp{Tag: deps.LlamaCppTag, URL: deps.LlamaCppURL, SHA256: deps.LlamaCppSHA256}},
		Log: JobLog{Prefix: LogPrefix, MetricEverySteps: jobMetricEvery},
	}
	if l := pl.Hyper.LoRA; l != nil {
		spec.Hyper.LoRA = &JobLoRA{R: l.R, Alpha: l.Alpha, Dropout: l.Dropout,
			TargetModules: append([]string{}, p.Training.LoRA.TargetModules...)}
	}
	return spec, nil
}

// JSON is the deterministic job.json: struct order, no HTML escaping, one-space indent, one trailing newline.
func (j JobSpec) JSON() ([]byte, error) {
	b, err := marshalStable(j)
	if err != nil {
		return nil, jobErr("encode: %w", err)
	}
	return b, nil
}
