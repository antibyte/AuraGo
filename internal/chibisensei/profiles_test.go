package chibisensei

import (
	"regexp"
	"strings"
	"testing"
)

func mustCatalog(t *testing.T) *Catalog {
	t.Helper()
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCatalogLoadsLaunchProfiles(t *testing.T) {
	c := mustCatalog(t)
	var ids []string
	for _, p := range c.List() {
		ids = append(ids, p.ID)
		if p.Status != "experimental" {
			t.Fatalf("%s status %q", p.ID, p.Status)
		}
	}
	if strings.Join(ids, ",") != "gemma3-270m,qwen35-0p8b,lfm25-1p2b" {
		t.Fatalf("order = %v", ids)
	}
	c2 := mustCatalog(t)
	hex64 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for _, id := range ids {
		if !hex64.MatchString(c.SHA(id)) || c.SHA(id) != c2.SHA(id) {
			t.Fatalf("sha of %s: %q vs %q", id, c.SHA(id), c2.SHA(id))
		}
	}
	g, _ := c.Get("gemma3-270m")
	q, _ := c.Get("qwen35-0p8b")
	if !g.Summary().License.RequiresConsent || q.Summary().License.RequiresConsent {
		t.Fatal("license consent flags wrong")
	}
}

func TestValidateProfileRules(t *testing.T) {
	c := mustCatalog(t)
	base, _ := c.Get("qwen35-0p8b")
	cases := []struct {
		name   string
		mutate func(p *Profile)
		want   string
	}{
		{"verified with unverified", func(p *Profile) { p.Status = "verified" }, "unverified"},
		{"gated", func(p *Profile) { p.Base.Gated = true }, "base.gated"},
		{"dtype", func(p *Profile) { p.Training.DType = "bf16" }, "dtype"},
		{"max_seq_cap", func(p *Profile) { p.Training.MaxSeqCap = 128 }, "max_seq_cap"},
		{"lr missing", func(p *Profile) { p.Training.Methods = []string{"lora", "full"} }, "lr"},
		{"method_default", func(p *Profile) { p.Training.MethodDefault = "qlora" }, "method_default"},
		{"revision not pinned", func(p *Profile) { p.Base.Revision = "main" }, "base.revision"},
		{"auto_class", func(p *Profile) { p.Base.AutoClass = "os.system" }, "base.auto_class"},
		{"fallback repeats auto_class", func(p *Profile) { p.Base.AutoClassFallbacks = []string{"Qwen3_5ForCausalLM"} }, "base.auto_class_fallbacks"},
		{"too many fallbacks", func(p *Profile) { p.Base.AutoClassFallbacks = []string{"A", "B", "C", "D", "E"} }, "base.auto_class_fallbacks"},
		{"target module", func(p *Profile) { p.Training.LoRA.TargetModules = []string{"q_proj; rm"} }, "training.lora.target_modules"},
		{"extra pip unpinned", func(p *Profile) { p.Training.ExtraPip = []string{"flash-linear-attention"} }, "training.extra_pip"},
		{"out type", func(p *Profile) { p.GGUF.OutType = "f16" }, "gguf.out_type"},
	}
	for _, tc := range cases {
		cp := *base
		tc.mutate(&cp)
		err := ValidateProfile(&cp)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to contain %q", tc.name, err, tc.want)
		}
	}
	if err := ValidateProfile(base); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchProfilePins(t *testing.T) {
	c := mustCatalog(t)
	revs := map[string]string{
		"gemma3-270m": "23cf460f6bb16954176b3ddcc8d4f250501458a9",
		"qwen35-0p8b": "2fc06364715b967f1860aea9cf38778875588b17",
		"lfm25-1p2b":  "b9d816ab510f9a28dca37cb182e9da8b0226e13c",
	}
	for id, rev := range revs {
		p, ok := c.Get(id)
		if !ok {
			t.Fatalf("missing profile %s", id)
		}
		if p.Base.Revision != rev {
			t.Errorf("%s revision = %q", id, p.Base.Revision)
		}
		if p.Status != "experimental" || p.Verified != nil {
			t.Errorf("%s must stay experimental and unverified", id)
		}
	}
	q, _ := c.Get("qwen35-0p8b")
	if q.Base.AutoClass != "Qwen3_5ForCausalLM" ||
		strings.Join(q.Base.AutoClassFallbacks, ",") != "AutoModelForCausalLM,AutoModelForMultimodalLM,AutoModelForImageTextToText" {
		t.Errorf("qwen classes = %q %v", q.Base.AutoClass, q.Base.AutoClassFallbacks)
	}
	if strings.Join(q.Training.LoRA.TargetModules, ",") != "q_proj,k_proj,v_proj,o_proj,gate_proj,up_proj,down_proj,in_proj_qkv,in_proj_z,out_proj" {
		t.Errorf("qwen target modules = %v", q.Training.LoRA.TargetModules)
	}
	if len(q.Training.ExtraPip) != 0 {
		t.Errorf("qwen extra_pip = %v", q.Training.ExtraPip)
	}
	hasFallbacks := false
	for _, u := range q.Unverified {
		if u == "base.auto_class_fallbacks" {
			hasFallbacks = true
		}
	}
	if !hasFallbacks {
		t.Error("qwen unverified must list base.auto_class_fallbacks")
	}
	l, _ := c.Get("lfm25-1p2b")
	if strings.Join(l.Training.LoRA.TargetModules, ",") != "q_proj,k_proj,v_proj,out_proj,w1,w2,w3,in_proj" {
		t.Errorf("lfm target modules = %v", l.Training.LoRA.TargetModules)
	}
}
