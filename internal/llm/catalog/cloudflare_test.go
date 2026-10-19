package catalog

import "testing"

func TestCloudflareRESTCatalogIsLocalAndSeparatelyPriced(t *testing.T) {
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	p, ok := s.FindProvider("cloudflare-rest")
	if !ok || p.CatalogOnly {
		t.Fatal("REST provider unavailable")
	}
	for _, m := range s.ModelsForProvider("cloudflare-rest") {
		if m.Source != SourceCloudflare || m.Cost != (Cost{}) || m.API != "openai-completions" {
			t.Fatalf("wrong provenance/wire format/pricing: %+v", m)
		}
	}
	if _, ok := s.FindModel("cloudflare-rest", "google/gemini-2.5-flash"); !ok {
		t.Fatal("Google REST model unavailable")
	}
}
