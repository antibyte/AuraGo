package chibisensei

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"aurago/internal/chibisensei/trainer"
)

// Profile is one model profile of the embedded catalog.
type Profile struct {
	ID          string           `json:"id"`
	DisplayName string           `json:"display_name"`
	Schema      int              `json:"schema"`
	Status      string           `json:"status"` // experimental|verified
	Verified    *ProfileVerified `json:"verified"`
	Unverified  []string         `json:"unverified"`
	License     ProfileLicense   `json:"license"`
	Base        ProfileBase      `json:"base"`
	Arch        ProfileArch      `json:"arch"`
	Chat        ProfileChat      `json:"chat"`
	Training    ProfileTraining  `json:"training"`
	Sampling    ProfileSampling  `json:"sampling"`
	Eval        ProfileEval      `json:"eval"`
	GGUF        ProfileGGUF      `json:"gguf"`
}

type ProfileVerified struct {
	Date          string `json:"date"`
	AuraGoVersion string `json:"aurago_version"`
	KaggleRef     string `json:"kaggle_ref"`
	LlamaCpp      string `json:"llama_cpp"`
}

type ProfileLicense struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	URL             string `json:"url"`
	Version         string `json:"version"`
	RequiresConsent bool   `json:"requires_consent"`
	NoteKey         string `json:"note_key"`
}

type ProfileBase struct {
	Source             string   `json:"source"`
	Repo               string   `json:"repo"`
	Revision           string   `json:"revision"`
	AutoClass          string   `json:"auto_class"`
	AutoClassFallbacks []string `json:"auto_class_fallbacks,omitempty"` // tried in order after AutoClass
	Gated              bool     `json:"gated"`
}

type ProfileArch struct {
	LlamaCppArch string   `json:"llama_cpp_arch"`
	ContextMax   int      `json:"context_max"`
	ParamsB      float64  `json:"params_b"`
	TextOnly     bool     `json:"text_only"`
	Languages    []string `json:"languages"`
}

type ProfileChat struct {
	SystemRole string   `json:"system_role"`
	Stop       []string `json:"stop"`
}

type ProfileLoRA struct {
	R             int      `json:"r"`
	Alpha         int      `json:"alpha"`
	Dropout       float64  `json:"dropout"`
	TargetModules []string `json:"target_modules"`
}

type ProfileTraining struct {
	MethodDefault        string             `json:"method_default"`
	Methods              []string           `json:"methods"`
	DType                string             `json:"dtype"`
	LoRA                 ProfileLoRA        `json:"lora"`
	LR                   map[string]float64 `json:"lr"`
	MaxSeqCap            int                `json:"max_seq_cap"`
	BatchTokens          int                `json:"batch_tokens"`
	GradCheckpointing    bool               `json:"grad_checkpointing"`
	T4TokensPerSecond    float64            `json:"t4_tokens_per_second"`
	T4GenTokensPerSecond float64            `json:"t4_gen_tokens_per_second"`
	MinGPUCapability     string             `json:"min_gpu_capability"`
	MinVRAMGB            float64            `json:"min_vram_gb"`
	ExtraPip             []string           `json:"extra_pip"`
}

type ProfileSampling struct {
	Temperature     float64 `json:"temperature"`
	TopP            float64 `json:"top_p"`
	TopK            int     `json:"top_k"`
	RepeatPenalty   float64 `json:"repeat_penalty"`
	PresencePenalty float64 `json:"presence_penalty"`
}

type ProfileEval struct {
	MaxNewTokensAssistant  int `json:"max_new_tokens_assistant"`
	MaxNewTokensSpecialist int `json:"max_new_tokens_specialist"`
}

type ProfileGGUF struct {
	OutType      string `json:"out_type"`
	ExpectedArch string `json:"expected_arch"`
	EstimatedMB  int    `json:"estimated_mb"`
}

// ProfileSummary is the /meta shape.
type ProfileSummary struct {
	ID              string   `json:"id"`
	DisplayName     string   `json:"display_name"`
	Status          string   `json:"status"`
	ParamsB         float64  `json:"params_b"`
	ContextMax      int      `json:"context_max"`
	Languages       []string `json:"languages"`
	TextOnly        bool     `json:"text_only"`
	SystemRole      string   `json:"system_role"`
	MethodDefault   string   `json:"method_default"`
	GGUFEstimatedMB int      `json:"gguf_estimated_mb"`
	Methods         []string `json:"methods"`     // training methods the profile allows (A32)
	MaxSeqCap       int      `json:"max_seq_cap"` // largest max_seq_len an override may pick (A32)
	License         struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		URL             string `json:"url"`
		Version         string `json:"version"`
		RequiresConsent bool   `json:"requires_consent"`
		Accepted        bool   `json:"accepted"` // filled by the server from the consents table; Summary leaves it false
	} `json:"license"`
}

//go:embed profiles/*.json
var profileFS embed.FS

// Catalog is the validated set of embedded profiles.
type Catalog struct {
	byID  map[string]*Profile
	order []*Profile
	sha   map[string]string
}

// LoadCatalog decodes every embedded file with DisallowUnknownFields, requires id == file stem, runs
// ValidateProfile and sorts by Arch.ParamsB. SHA = hex sha256 of json.Marshal(profile).
func LoadCatalog() (*Catalog, error) {
	entries, err := profileFS.ReadDir("profiles")
	if err != nil {
		return nil, err
	}
	c := &Catalog{byID: map[string]*Profile{}, sha: map[string]string{}}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		raw, err := profileFS.ReadFile("profiles/" + name)
		if err != nil {
			return nil, err
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		var p Profile
		if err := dec.Decode(&p); err != nil {
			return nil, fmt.Errorf("profile %s: %w", name, err)
		}
		if stem := strings.TrimSuffix(name, ".json"); p.ID != stem {
			return nil, fmt.Errorf("profile %s: id %q does not match the file name", name, p.ID)
		}
		if err := ValidateProfile(&p); err != nil {
			return nil, fmt.Errorf("profile %s: %w", name, err)
		}
		canon, err := json.Marshal(&p)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(canon)
		pp := p
		c.byID[p.ID] = &pp
		c.sha[p.ID] = hex.EncodeToString(sum[:])
		c.order = append(c.order, &pp)
	}
	sort.SliceStable(c.order, func(i, j int) bool {
		if c.order[i].Arch.ParamsB != c.order[j].Arch.ParamsB {
			return c.order[i].Arch.ParamsB < c.order[j].Arch.ParamsB
		}
		return c.order[i].ID < c.order[j].ID
	})
	return c, nil
}

// Get returns a profile by ID.
func (c *Catalog) Get(id string) (*Profile, bool) {
	p, ok := c.byID[id]
	return p, ok
}

// List returns the profiles ordered by parameter count.
func (c *Catalog) List() []*Profile { return append([]*Profile(nil), c.order...) }

// SHA returns the canonical hash of a profile ("" when unknown).
func (c *Catalog) SHA(id string) string { return c.sha[id] }

// Summary is the compact /meta form.
func (p *Profile) Summary() ProfileSummary {
	s := ProfileSummary{ID: p.ID, DisplayName: p.DisplayName, Status: p.Status, ParamsB: p.Arch.ParamsB,
		ContextMax: p.Arch.ContextMax, Languages: append([]string{}, p.Arch.Languages...), TextOnly: p.Arch.TextOnly,
		SystemRole: p.Chat.SystemRole, MethodDefault: p.Training.MethodDefault, GGUFEstimatedMB: p.GGUF.EstimatedMB,
		Methods: append([]string{}, p.Training.Methods...), MaxSeqCap: p.Training.MaxSeqCap}
	s.License.ID, s.License.Name, s.License.URL = p.License.ID, p.License.Name, p.License.URL
	s.License.Version = p.License.Version
	s.License.RequiresConsent = p.License.RequiresConsent
	return s
}

func profileErr(path, msg string) error { return fmt.Errorf("%s: %s", path, msg) }

// ValidateProfile checks a profile; the first failure wins and the error names the JSON path.
func ValidateProfile(p *Profile) error {
	if p.Schema != 1 {
		return profileErr("schema", "must be 1")
	}
	if !profileIDPattern.MatchString(p.ID) {
		return profileErr("id", "invalid profile id")
	}
	switch p.Status {
	case "experimental":
	case "verified":
		if len(p.Unverified) > 0 {
			return profileErr("unverified", "must be empty for a verified profile")
		}
		if p.Verified == nil {
			return profileErr("verified", "required for a verified profile")
		}
	default:
		return profileErr("status", "must be experimental or verified")
	}
	if p.Base.Source != "hf" {
		return profileErr("base.source", `must be "hf"`)
	}
	if p.Base.Repo == "" {
		return profileErr("base.repo", "required")
	}
	if p.Base.Revision == "" {
		return profileErr("base.revision", "required")
	}
	if p.Base.Gated {
		return profileErr("base.gated", "gated models are not supported")
	}
	t := p.Training
	if t.DType != "float32" {
		return profileErr("training.dtype", `must be "float32"`)
	}
	if len(t.Methods) == 0 {
		return profileErr("training.methods", "required")
	}
	inMethods := false
	for _, m := range t.Methods {
		if m != "lora" && m != "full" {
			return profileErr("training.methods", "only lora and full are allowed")
		}
		if m == t.MethodDefault {
			inMethods = true
		}
	}
	if !inMethods {
		return profileErr("training.method_default", "must be one of the methods")
	}
	for _, m := range t.Methods {
		if t.LR[m] <= 0 {
			return profileErr("training.lr."+m, "needs a value above 0")
		}
	}
	if t.MaxSeqCap < 256 || t.MaxSeqCap > 4096 {
		return profileErr("training.max_seq_cap", "must be 256 to 4096")
	}
	if t.BatchTokens < 256 {
		return profileErr("training.batch_tokens", "must be at least 256")
	}
	if t.T4TokensPerSecond <= 0 {
		return profileErr("training.t4_tokens_per_second", "must be above 0")
	}
	if t.T4GenTokensPerSecond <= 0 {
		return profileErr("training.t4_gen_tokens_per_second", "must be above 0")
	}
	if t.LoRA.R < 1 {
		return profileErr("training.lora.r", "must be at least 1")
	}
	if t.LoRA.Alpha < 1 {
		return profileErr("training.lora.alpha", "must be at least 1")
	}
	if p.Chat.SystemRole != "native" && p.Chat.SystemRole != "merge_into_first_user" {
		return profileErr("chat.system_role", "must be native or merge_into_first_user")
	}
	if p.GGUF.EstimatedMB <= 0 {
		return profileErr("gguf.estimated_mb", "must be above 0")
	}
	if len(p.Arch.Languages) == 0 {
		return profileErr("arch.languages", "required")
	}
	return validateProfileNames(p)
}

var (
	revisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	pyClassPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,79}$`)
	modulePattern   = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
)

const maxAutoClassFallbacks = 4

// validateProfileNames checks the values the trainer hands to Python: the pinned revision, the class names it
// resolves with getattr(transformers, name), the LoRA module names, the extra pip pins and the GGUF type.
func validateProfileNames(p *Profile) error {
	if !revisionPattern.MatchString(p.Base.Revision) {
		return profileErr("base.revision", "must be a pinned 40-character commit sha")
	}
	if !pyClassPattern.MatchString(p.Base.AutoClass) {
		return profileErr("base.auto_class", "must be a transformers class name")
	}
	if len(p.Base.AutoClassFallbacks) > maxAutoClassFallbacks {
		return profileErr("base.auto_class_fallbacks", "at most 4 entries")
	}
	seen := map[string]bool{p.Base.AutoClass: true}
	for _, c := range p.Base.AutoClassFallbacks {
		if !pyClassPattern.MatchString(c) || seen[c] {
			return profileErr("base.auto_class_fallbacks", "entries must be distinct class names other than auto_class")
		}
		seen[c] = true
	}
	mods := p.Training.LoRA.TargetModules
	if len(mods) == 0 || len(mods) > 32 {
		return profileErr("training.lora.target_modules", "needs 1 to 32 module names")
	}
	seenMod := map[string]bool{}
	for _, m := range mods {
		if !modulePattern.MatchString(m) || seenMod[m] {
			return profileErr("training.lora.target_modules", "entries must be distinct module names")
		}
		seenMod[m] = true
	}
	for _, pin := range p.Training.ExtraPip {
		if !trainer.ValidPin(pin) {
			return profileErr("training.extra_pip", `entries must be exact "name==version" pins`)
		}
	}
	if p.GGUF.OutType != "q8_0" {
		return profileErr("gguf.out_type", `must be "q8_0"`)
	}
	return nil
}
