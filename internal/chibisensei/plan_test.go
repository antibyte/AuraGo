package chibisensei

import "testing"

func TestPresetsAndPlan(t *testing.T) {
	c := mustCatalog(t)
	qwen, _ := c.Get("qwen35-0p8b")
	gemma, _ := c.Get("gemma3-270m")

	rows := make([]SplitRow, 371)
	assign := map[int64]string{}
	for i := range rows {
		rows[i] = SplitRow{ID: int64(i + 1), Split: "auto", EstTokens: 180}
		assign[rows[i].ID] = "train"
	}
	split := SplitResult{Counts: SplitCounts{Train: 371, Eval: 20}, Assign: assign}
	proj := Project{ProjectSummary: ProjectSummary{ID: "p", Recipe: RecipeAssistant, DataRev: 5}}
	plan := func(preset string) Plan {
		proj.Preset = preset
		pl, err := ComputePlan(PlanInput{Project: proj, Profile: qwen, Rows: rows, Split: split, RemoteTimeoutMinutes: 60, NeedBaseGGUF: true})
		if err != nil {
			t.Fatal(err)
		}
		return pl
	}
	b := plan("balanced")
	h := b.Hyper
	if h.MaxSeqLen != 256 || h.Epochs != 2 || h.PerDeviceBatch != 8 || h.GradAccum != 2 || h.LearningRate != 0.0002 ||
		h.LoRA == nil || *h.LoRA != (LoRAHyper{16, 16, 0.05}) || b.EstimateMinutes != 27 || b.SessionTimeoutMinutes != 56 {
		t.Fatalf("balanced = %+v", b)
	}
	if !b.Trainable || b.ReasonKey != "" || b.DataRev != 5 || b.EvalSamples != 20 || b.MaxNewTokens != 256 ||
		b.MaxTrainSeconds != 1048 || !b.NeedBaseGGUF || b.ArtifactsEstimatedMB != 1850 {
		t.Fatalf("plan flags = %+v", b)
	}
	q := plan("quick")
	if q.Hyper.Epochs != 1 || *q.Hyper.LoRA != (LoRAHyper{8, 16, 0.05}) || q.EstimateMinutes != 27 || q.MaxTrainSeconds != 1048 {
		t.Fatalf("quick = %+v", q)
	}
	th := plan("thorough")
	if th.Hyper.Epochs != 3 || *th.Hyper.LoRA != (LoRAHyper{32, 32, 0.05}) || th.EstimateMinutes != 28 || th.MaxTrainSeconds != 1108 {
		t.Fatalf("thorough = %+v", th)
	}
	short := func() Plan {
		pl, err := ComputePlan(PlanInput{Project: proj, Profile: qwen, Rows: rows, Split: split, RemoteTimeoutMinutes: 30, NeedBaseGGUF: true})
		if err != nil {
			t.Fatal(err)
		}
		return pl
	}()
	if short.SessionTimeoutMinutes != 30 || short.MaxTrainSeconds != 300 {
		t.Fatalf("30 minute limit = %+v", short)
	}
	proj.Preset = "balanced"
	noBase, err := ComputePlan(PlanInput{Project: proj, Profile: qwen, Rows: rows, Split: split, RemoteTimeoutMinutes: 60})
	if err != nil {
		t.Fatal(err)
	}
	if noBase.EstimateMinutes != 24 || noBase.SessionTimeoutMinutes != 51 || noBase.MaxTrainSeconds != 1018 ||
		noBase.ArtifactsEstimatedMB != 950 || noBase.NeedBaseGGUF {
		t.Fatalf("no base gguf = %+v", noBase)
	}
	proj.Preset = "quick"
	bigRows := make([]SplitRow, 3000)
	bigAssign := map[int64]string{}
	for i := range bigRows {
		bigRows[i] = SplitRow{ID: int64(i + 1), Split: "auto", EstTokens: 2000}
		bigAssign[bigRows[i].ID] = "train"
	}
	capped, err := ComputePlan(PlanInput{Project: proj, Profile: qwen, Rows: bigRows,
		Split: SplitResult{Counts: SplitCounts{Train: 3000, Eval: 20}, Assign: bigAssign}, RemoteTimeoutMinutes: 720, NeedBaseGGUF: true})
	if err != nil {
		t.Fatal(err)
	}
	if capped.EstimateMinutes != 56 || capped.SessionTimeoutMinutes != 99 || capped.MaxTrainSeconds != 1800 {
		t.Fatalf("quick cap = %+v", capped)
	}

	for _, tc := range [][3]int{{232, 2048, 256}, {233, 2048, 512}, {1900, 2048, 2048}, {0, 2048, 256}} {
		if got := MaxSeqLen(tc[0], tc[1]); got != tc[2] {
			t.Errorf("MaxSeqLen(%d,%d) = %d, want %d", tc[0], tc[1], got, tc[2])
		}
	}
	for n, want := range map[int]int{299: 3, 300: 2, 3000: 1} {
		if got := BaseEpochs(n); got != want {
			t.Errorf("BaseEpochs(%d) = %d, want %d", n, got, want)
		}
	}
	if h, _ := PresetHyper(qwen, "balanced", "lora", 50, 180, Overrides{}); h.PerDeviceBatch != 5 || h.GradAccum != 1 {
		t.Errorf("nTrain 50: %+v", h)
	}
	if h, _ := PresetHyper(qwen, "balanced", "lora", 5, 180, Overrides{}); h.PerDeviceBatch != 1 || h.GradAccum != 1 {
		t.Errorf("nTrain 5: %+v", h)
	}
	if h, _ := PresetHyper(gemma, "balanced", "full", 371, 180, Overrides{}); h.LoRA != nil || h.LearningRate != 0.00005 {
		t.Errorf("gemma full: %+v", h)
	}
	if is := ValidateOverrides(Overrides{Method: "full"}, qwen); len(is) != 1 || is[0].Field != "overrides.method" {
		t.Errorf("method override: %+v", is)
	}
	if is := ValidateOverrides(Overrides{MaxSeqLen: 300}, qwen); len(is) != 1 || is[0].Field != "overrides.max_seq_len" {
		t.Errorf("max_seq_len override: %+v", is)
	}
}
