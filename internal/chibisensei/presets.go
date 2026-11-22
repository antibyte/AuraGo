package chibisensei

import "math"

var seqBuckets = []int{256, 512, 1024, 2048, 4096}

// MaxSeqLen is the smallest bucket b with b*10 >= 11*p95, capped at maxSeqCap (min(4096, cap) when none fits).
func MaxSeqLen(p95, maxSeqCap int) int {
	pick := seqBuckets[len(seqBuckets)-1]
	for _, b := range seqBuckets {
		if b*10 >= 11*p95 {
			pick = b
			break
		}
	}
	if pick > maxSeqCap {
		pick = maxSeqCap
	}
	return pick
}

// BaseEpochs is the epoch count before the preset adjusts it.
func BaseEpochs(n int) int {
	if n < 300 {
		return 3
	}
	if n < 3000 {
		return 2
	}
	return 1
}

type LoRAHyper struct {
	R       int     `json:"r"`
	Alpha   int     `json:"alpha"`
	Dropout float64 `json:"dropout"`
}

type Hyper struct {
	Epochs         int        `json:"epochs"`
	LearningRate   float64    `json:"learning_rate"`
	PerDeviceBatch int        `json:"per_device_batch"`
	GradAccum      int        `json:"grad_accum"`
	MaxSeqLen      int        `json:"max_seq_len"`
	LoRA           *LoRAHyper `json:"lora"` // nil for full
}

// PresetHyper derives the hyper-parameters of a preset (overrides already validated). It returns the number of
// evaluation samples of the preset as well.
func PresetHyper(p *Profile, preset, method string, nTrain, p95 int, o Overrides) (Hyper, int) {
	if method == "" {
		method = p.Training.MethodDefault
	}
	base := p.Training.LoRA
	var epochs, evalSamples int
	lora := LoRAHyper{R: base.R, Alpha: base.Alpha, Dropout: base.Dropout}
	switch preset {
	case "quick":
		epochs, evalSamples = 1, 30
		lora.R, lora.Alpha = max(1, base.R/2), base.R
	case "thorough":
		epochs, evalSamples = min(BaseEpochs(nTrain)+1, 4), 200
		lora.R, lora.Alpha = 2*base.R, 2*base.Alpha
	default:
		epochs, evalSamples = BaseEpochs(nTrain), 100
	}
	maxSeq := MaxSeqLen(p95, p.Training.MaxSeqCap)
	lr := p.Training.LR[method]
	if o.Epochs > 0 {
		epochs = o.Epochs
	}
	if o.LearningRate > 0 {
		lr = o.LearningRate
	}
	if o.MaxSeqLen > 0 {
		maxSeq = o.MaxSeqLen
	}
	if o.LoRAR > 0 {
		lora.R = o.LoRAR
	}
	if o.LoRAAlpha > 0 {
		lora.Alpha = o.LoRAAlpha
	}
	pdb := min(8, max(1, p.Training.BatchTokens/maxSeq))
	ga := (16 + pdb - 1) / pdb
	limit := float64(nTrain) / 10
	if float64(pdb*ga) > limit {
		ga = max(1, int(math.Floor(limit/float64(pdb))))
	}
	if float64(pdb*ga) > limit {
		pdb = max(1, int(math.Floor(limit)))
		ga = 1
	}
	h := Hyper{Epochs: epochs, LearningRate: lr, PerDeviceBatch: pdb, GradAccum: ga, MaxSeqLen: maxSeq}
	if method == "lora" {
		h.LoRA = &lora
	}
	return h, evalSamples
}

func badOverride(name, msg string) Issue { return issue("overrides."+name, "plan.bad_override", msg) }

// ValidateOverrides checks the user's overrides against a profile.
func ValidateOverrides(o Overrides, p *Profile) []Issue {
	var out []Issue
	if o.Method != "" {
		ok := false
		for _, m := range p.Training.Methods {
			ok = ok || m == o.Method
		}
		if !ok {
			out = append(out, badOverride("method", "The profile does not support this method."))
		}
	}
	if o.Epochs != 0 && (o.Epochs < 1 || o.Epochs > 10) {
		out = append(out, badOverride("epochs", "Epochs must be 1 to 10."))
	}
	if o.LearningRate != 0 && (o.LearningRate < 1e-6 || o.LearningRate > 1e-3) {
		out = append(out, badOverride("learning_rate", "The learning rate must be between 1e-6 and 1e-3."))
	}
	if o.MaxSeqLen != 0 {
		ok := false
		for _, b := range seqBuckets {
			ok = ok || (b == o.MaxSeqLen && b <= p.Training.MaxSeqCap)
		}
		if !ok {
			out = append(out, badOverride("max_seq_len", "The sequence length must be a bucket within the profile cap."))
		}
	}
	if o.LoRAR != 0 && (o.LoRAR < 1 || o.LoRAR > 256) {
		out = append(out, badOverride("lora_r", "The LoRA rank must be 1 to 256."))
	}
	if o.LoRAAlpha != 0 && (o.LoRAAlpha < 1 || o.LoRAAlpha > 512) {
		out = append(out, badOverride("lora_alpha", "The LoRA alpha must be 1 to 512."))
	}
	return out
}
