package prompts

import (
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/personabooster"
	promptsembed "aurago/prompts"
)

func writePersonaPhaseFile(t *testing.T, dir, name, content string) {
	t.Helper()
	target := filepath.Join(dir, "persona_phases")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, name+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadPersonaPhaseCatalogSourcesAndFallbacks(t *testing.T) {
	embedded, err := fs.ReadFile(promptsembed.FS, "persona_phases/punk.yaml")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("embedded", func(t *testing.T) {
		cat, revision, ok := LoadPersonaPhaseCatalog(t.TempDir(), "punk", slog.Default())
		if !ok || cat.Persona != "punk" || !strings.HasPrefix(revision, "embed:") {
			t.Fatalf("ok=%v revision=%q", ok, revision)
		}
	})
	t.Run("valid disk override wins", func(t *testing.T) {
		dir := t.TempDir()
		override := strings.Replace(string(embedded), "spontaneous_per_hour: 0.10", "spontaneous_per_hour: 0.30", 1)
		if override == string(embedded) {
			t.Fatal("fixture replacement did not apply")
		}
		writePersonaPhaseFile(t, dir, "punk", override)
		cat, revision, ok := LoadPersonaPhaseCatalog(dir, "punk", slog.Default())
		if !ok || cat.SpontaneousPerHour != 0.30 || !strings.HasPrefix(revision, "disk:") {
			t.Fatalf("ok=%v revision=%q", ok, revision)
		}
	})
	t.Run("invalid disk file falls back to embedded", func(t *testing.T) {
		dir := t.TempDir()
		writePersonaPhaseFile(t, dir, "punk", "persona: punk\nphases: []\n")
		cat, revision, ok := LoadPersonaPhaseCatalog(dir, "punk", slog.Default())
		if !ok || len(cat.Phases) == 0 || !strings.HasPrefix(revision, "embed:") {
			t.Fatalf("ok=%v revision=%q", ok, revision)
		}
	})
	t.Run("custom persona uses the generic catalog", func(t *testing.T) {
		cat, _, ok := LoadPersonaPhaseCatalog(t.TempDir(), "my_custom_bot", slog.Default())
		if !ok || cat.Persona != personabooster.GenericPersona {
			t.Fatalf("ok=%v", ok)
		}
	})
	t.Run("neutral and empty have no catalog", func(t *testing.T) {
		for _, persona := range []string{"neutral", ""} {
			if _, _, ok := LoadPersonaPhaseCatalog(t.TempDir(), persona, slog.Default()); ok {
				t.Fatalf("%q must not get a phase catalog", persona)
			}
		}
	})
}

var expectedSpontaneousPerHour = map[string]float64{
	"psycho": 0.15, "punk": 0.10, "evil": 0.10,
	"thinker": 0.08, "mistress": 0.08, "mcp": 0.08,
	"friend": 0.06, "secretary": 0.06, "terminator": 0.06,
	"servant": 0.05, "professional": 0.05, "_generic": 0.05,
}

func TestEmbeddedPersonaPhaseCatalogsFollowWritingRules(t *testing.T) {
	files, err := fs.Glob(promptsembed.FS, "persona_phases/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 2 {
		t.Fatalf("embedded catalogs = %v, want at least punk and _generic", files)
	}
	for _, file := range files {
		id := strings.TrimSuffix(path.Base(file), ".yaml")
		t.Run(id, func(t *testing.T) {
			data, err := fs.ReadFile(promptsembed.FS, file)
			if err != nil {
				t.Fatal(err)
			}
			cat, err := personabooster.ParseCatalog(data)
			if err != nil {
				t.Fatal(err)
			}
			if cat.Persona != id {
				t.Fatalf("persona %q in %s", cat.Persona, file)
			}
			if want, ok := expectedSpontaneousPerHour[id]; !ok || cat.SpontaneousPerHour != want {
				t.Errorf("spontaneous_per_hour = %v, want %v (known=%v)", cat.SpontaneousPerHour, want, ok)
			}
			if n := len(strings.Fields(cat.Amplifier)); n < 40 || n > 80 {
				t.Errorf("amplifier has %d words, want 40-80", n)
			}
			spontaneous := false
			for _, phase := range cat.Phases {
				spontaneous = spontaneous || phase.Triggers[personabooster.TriggerSpontaneous] > 0
				for _, variant := range phase.Variants {
					if n := len(strings.Fields(variant.Text)); n < 40 || n > 90 {
						t.Errorf("%s/%s has %d words, want 40-90", phase.ID, variant.ID, n)
					}
					if strings.Count(variant.Text, `"`) > 2 {
						t.Errorf("%s/%s quotes more than one sample line", phase.ID, variant.ID)
					}
				}
			}
			if !spontaneous {
				t.Error("no phase carries a spontaneous weight")
			}
		})
	}
}
