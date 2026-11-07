package designs

import (
	"strings"
	"testing"
)

func TestCatalogContainsBundledTemplates(t *testing.T) {
	t.Parallel()
	catalog := Catalog()
	if len(catalog) != 25 {
		t.Fatalf("expected 25 bundled design templates, got %d", len(catalog))
	}
	for i, design := range catalog {
		if design.ID == "" || design.Name == "" {
			t.Fatalf("catalog entry %d is incomplete: %+v", i, design)
		}
		if design.Rank != i+1 {
			t.Fatalf("catalog entry %q has rank %d, want %d", design.ID, design.Rank, i+1)
		}
		if design.Likes <= 0 {
			t.Fatalf("catalog entry %q has no like count", design.ID)
		}
	}
	// Most liked first.
	if catalog[0].Likes < catalog[len(catalog)-1].Likes {
		t.Fatalf("catalog is not ordered by likes: %d < %d", catalog[0].Likes, catalog[len(catalog)-1].Likes)
	}
}

func TestCatalogEntryHasTemplateAndPreviews(t *testing.T) {
	t.Parallel()
	for _, design := range Catalog() {
		content, ok := Content(design.ID)
		if !ok || strings.TrimSpace(content) == "" {
			t.Fatalf("template %q has no DESIGN.md content", design.ID)
		}
		if !strings.Contains(strings.ToLower(content), "##") {
			t.Fatalf("template %q does not look like a design system document", design.ID)
		}
		for _, variant := range PreviewVariants {
			data, ok := Preview(design.ID, variant)
			if !ok || len(data) == 0 {
				t.Fatalf("template %q has no %s preview", design.ID, variant)
			}
		}
	}
}

func TestLookupIsCaseInsensitiveAndRejectsUnknownIDs(t *testing.T) {
	t.Parallel()
	design, ok := Lookup("GENESIS")
	if !ok {
		t.Fatal("expected case-insensitive lookup to resolve genesis")
	}
	if design.ID != "genesis" {
		t.Fatalf("lookup returned %q", design.ID)
	}
	for _, id := range []string{"", "  ", "nope", "../genesis", "templates/genesis", "Genesis/../.."} {
		if _, ok := Lookup(id); ok {
			t.Fatalf("lookup must reject %q", id)
		}
	}
}

func TestPreviewRejectsUnknownVariant(t *testing.T) {
	t.Parallel()
	if _, ok := Preview("genesis", "sepia"); ok {
		t.Fatal("unknown preview variant must not resolve")
	}
	if _, ok := Preview("genesis", ""); !ok {
		t.Fatal("empty variant must fall back to the light preview")
	}
}

func TestPromptNamesTheSelectedTemplate(t *testing.T) {
	t.Parallel()
	prompt, err := Prompt("genesis")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	for _, marker := range []string{"# SELECTED DESIGN SYSTEM", "Genesis", "template id: genesis"} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("prompt missing %q:\n%s", marker, prompt)
		}
	}
	if _, err := Prompt("does-not-exist"); err == nil {
		t.Fatal("expected an error for an unknown template")
	}
}
