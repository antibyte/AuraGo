package chibisensei

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildJobSpecFields(t *testing.T) {
	c := mustCatalog(t)
	for _, gc := range goldenCases {
		spec := goldenJob(t, c, gc, goldenLock)
		b, err := spec.JSON()
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(b) || !bytes.HasSuffix(b, []byte("}\n")) || bytes.HasSuffix(b, []byte("\n\n")) {
			t.Errorf("%s: JSON is not one valid document with one trailing newline", gc.profile)
		}
		if !bytes.Contains(b, []byte("<|im_end|>")) && !bytes.Contains(b, []byte("<end_of_turn>")) {
			t.Errorf("%s: end tokens must stay literal (no \\u003c)", gc.profile)
		}
		switch gc.profile {
		case "qwen35-0p8b":
			prof, _ := c.Get(gc.profile)
			if spec.TaskKind != nil || !strings.Contains(string(b), `"task_kind": null`) || !strings.Contains(string(b), `"labels": []`) {
				t.Error("qwen: task_kind must be null and labels []")
			}
			if spec.Method != "lora" || spec.Hyper.LoRA == nil || len(spec.Hyper.LoRA.TargetModules) != len(prof.Training.LoRA.TargetModules) ||
				len(spec.Hyper.LoRA.TargetModules) != 10 {
				t.Errorf("qwen: method/lora wrong: %+v", spec.Hyper.LoRA)
			}
			h := spec.Hyper
			if h.LossOn != "assistant" || h.LRScheduler != "cosine" || h.WarmupRatio != 0.03 || h.WeightDecay != 0 || h.Seed != 42 || !h.GradientCheckpointing {
				t.Errorf("qwen: hyper wrong: %+v", h)
			}
			if spec.Data != (JobData{Marker: "r0123abcd4567-marker.json", Format: "messages"}) {
				t.Errorf("qwen: data %+v", spec.Data)
			}
			pl := goldenInput(t, c, gc, goldenLock).Plan
			if spec.Budget.SessionTimeoutS != pl.SessionTimeoutMinutes*60 || spec.Budget.MaxTrainS != pl.MaxTrainSeconds ||
				spec.Budget.EvalMaxSamples != 20 || spec.Budget.MaxNewTokens != 256 {
				t.Errorf("qwen: budget %+v", spec.Budget)
			}
			if spec.Outputs != (JobOutputs{EmitBaseGGUF: true, GGUFOutType: "q8_0", Prefix: "chibi-"}) {
				t.Errorf("qwen: outputs %+v", spec.Outputs)
			}
			if len(spec.Deps.Pip) != 1 || spec.Deps.Pip[0] != "peft==0.21.3" || spec.Deps.LlamaCpp.Tag != "b9994" {
				t.Errorf("qwen: deps %+v", spec.Deps)
			}
			if spec.Log != (JobLog{Prefix: "@@chibi", MetricEverySteps: 10}) {
				t.Errorf("qwen: log %+v", spec.Log)
			}
		case "gemma3-270m":
			if spec.TaskKind == nil || *spec.TaskKind != "classify" || len(spec.Labels) != 4 || spec.Budget.MaxNewTokens != 64 {
				t.Errorf("gemma: %+v", spec)
			}
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			if _, has := m["hyper"].(map[string]any)["lora"]; has || spec.Method != "full" {
				t.Error("gemma: a full run must have no hyper.lora")
			}
		case "lfm25-1p2b":
			if spec.Data.Format != "text" || spec.Hyper.LossOn != "all" {
				t.Errorf("lfm: %+v %s", spec.Data, spec.Hyper.LossOn)
			}
		}
	}
}

func TestBuildJobSpecRejects(t *testing.T) {
	c := mustCatalog(t)
	cases := []struct {
		name   string
		mutate func(*JobInput)
		want   string
	}{
		{"run id", func(in *JobInput) { in.RunID = "x" }, "invalid run id"},
		{"profile", func(in *JobInput) { in.Profile = nil }, "profile required"},
		{"trainable", func(in *JobInput) { in.Plan.Trainable = false }, "not trainable"},
		{"plan profile", func(in *JobInput) { in.Plan.ProfileID = "gemma3-270m" }, "plan is for profile"},
		{"method", func(in *JobInput) { in.Plan.Method = "full" }, "does not support method"},
		{"lora", func(in *JobInput) { in.Plan.Hyper.LoRA = nil }, "exactly for method lora"},
		{"marker", func(in *JobInput) { in.Marker = "x.json" }, "marker must be"},
		{"format", func(in *JobInput) { in.Format = "text" }, "does not fit recipe"},
		{"task kind", func(in *JobInput) { in.Project.TaskKind = "classify" }, "only a specialist"},
		{"profile sha", func(in *JobInput) { in.ProfileSHA = "abc" }, "profile sha"},
		{"llama url", func(in *JobInput) { in.Lock.LlamaCppURL = "https://example.com/x" }, "dependencies"},
		{"duplicate pin", func(in *JobInput) { in.Lock.Pip = append(in.Lock.Pip, "peft==0.21.3") }, "twice"},
		{"version", func(in *JobInput) { in.AuraGoVersion = "" }, "aurago version"},
		{"budget", func(in *JobInput) { in.Plan.MaxTrainSeconds = 10 }, "budget"},
	}
	for _, tc := range cases {
		in := goldenInput(t, c, goldenCases[1], goldenLock)
		in.Lock.Pip = append([]string{}, in.Lock.Pip...)
		tc.mutate(&in)
		if _, err := BuildJobSpec(in); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v does not contain %q", tc.name, err, tc.want)
		}
	}
}
