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
