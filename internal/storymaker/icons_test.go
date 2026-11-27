package storymaker

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadCatalogReadsIcons(t *testing.T) {
	mini := loadMini(t)
	if got := len(mini.IconIDs()); got != 6 {
		t.Fatalf("mini icons = %d, want 6", got)
	}
	cpu, ok := mini.Icon("icon.cpu")
	if !ok || len(cpu.Paths) == 0 || len(cpu.Tags) == 0 {
		t.Fatalf("Icon(icon.cpu) = %+v, %v", cpu, ok)
	}
	if _, ok := mini.Icon("icon.nope"); ok {
		t.Fatal("an unknown icon was found")
	}
	if mini.IconsFile() != "icons.json" {
		t.Fatalf("IconsFile() = %q", mini.IconsFile())
	}
	ids := mini.IconIDs()
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("IconIDs not sorted: %v", ids)
		}
	}

	rc := loadReal(t)
	if n := len(rc.IconIDs()); n < 490 || n > 530 {
		t.Fatalf("real icons = %d, want 490..530", n)
	}
	if rc.IconsFile() != "icons/lucide.json" {
		t.Fatalf("real IconsFile() = %q", rc.IconsFile())
	}

	manifest, err := os.ReadFile("testdata/catalog-mini/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	icons, err := os.ReadFile("testdata/catalog-mini/icons.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, from, to, want string }{
		{"duplicate id", `"id":"icon.database"`, `"id":"icon.cpu"`, "icon.cpu"},
		{"unsorted", `"id":"icon.cpu"`, `"id":"icon.zzz"`, "sorted"},
		{"bad id", `"id":"icon.mail"`, `"id":"icon.Mail"`, "icon.Mail"},
		{"bad schema", `storymaker-icons/1`, `storymaker-icons/9`, "schema"},
		{"bad path", `"M12 20v2"`, `"M12 20 <script>"`, "path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := strings.Replace(string(icons), tc.from, tc.to, 1)
			if text == string(icons) {
				t.Fatal("the edit changed nothing")
			}
			fsys := fstest.MapFS{"catalog.json": {Data: manifest}, "icons.json": {Data: []byte(text)}}
			_, err := LoadCatalog(fsys, "catalog.json")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadCatalog error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// newIconStudio returns a Studio over the real catalog in the phase, for the icon search tests.
func newIconStudio(t *testing.T, mode, phase string) *Studio {
	t.Helper()
	ps, _, _ := newTestStore(t)
	proj, _, _, err := ps.Create(context.Background(), "Icons", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	studio, err := NewStudio(StudioDeps{
		Projects: ps,
		Catalog:  loadReal(t),
		Limits:   DefaultLimits(),
		Compile: func(context.Context, string) (CompileSummary, error) {
			return CompileSummary{Available: false, Reason: "no browser in tests"}, nil
		},
		ReadOnly: func() bool { return false },
	}, StudioJob{JobID: "job-1", ProjectID: proj.ID, Kind: AgentKindCreate, Language: "en", TargetSeconds: 60, Mode: mode})
	if err != nil {
		t.Fatal(err)
	}
	studio.SetPhase(phase, "")
	return studio
}

func iconAnswer(t *testing.T, s *Studio, op string, args map[string]any) map[string]any {
	t.Helper()
	res := s.Call(context.Background(), StudioCall{Tool: ToolCatalog, Operation: op, Args: args})
	var answer map[string]any
	if err := json.Unmarshal(res.JSON, &answer); err != nil {
		t.Fatalf("answer %s: %v", res.JSON, err)
	}
	return answer
}

func TestSearchFindsIcons(t *testing.T) {
	cat := loadReal(t)
	if NormalizeKind("icon") != "icon" || NormalizeKind("Icons") != "icon" {
		t.Fatal("NormalizeKind must accept icon and icons")
	}
	ids := func(es []Entry) []string {
		out := make([]string, len(es))
		for i, e := range es {
			out[i] = e.ID
		}
		return out
	}
	if got := ids(cat.Search("processor", "icon", 6)); !slices.Contains(got, "icon.cpu") {
		t.Fatalf("Search(processor, icon) = %v", got)
	}
	both := ids(cat.Search("lock", "", 12))
	prop, icon := slices.Index(both, "prop.lock"), slices.Index(both, "icon.lock")
	if prop < 0 || icon < 0 || prop > icon {
		t.Fatalf("Search(lock) = %v, want prop.lock before icon.lock", both)
	}
	if slices.Contains(ids(cat.Search("processor", "prop", 6)), "icon.cpu") {
		t.Fatal("a prop search must not return icons")
	}

	studio := newIconStudio(t, ModeCatalog, PhaseFill)
	found := iconAnswer(t, studio, "search", map[string]any{"query": "processor", "kind": "icon"})
	results, _ := found["results"].([]any)
	var cpu map[string]any
	for _, r := range results {
		if m := r.(map[string]any); m["id"] == "icon.cpu" {
			cpu = m
		}
	}
	if cpu == nil || cpu["kind"] != "icon" {
		t.Fatalf("search answer = %v", found)
	}
	example, _ := cpu["usage_example"].(map[string]any)
	if example["do"] != "show" || example["icon"] != "icon.cpu" || example["style"] != "card" || example["text"] != "Cpu" || example["at"] != "board" || example["entry"] != "draw" {
		t.Fatalf("usage_example = %v", cpu["usage_example"])
	}
	described := iconAnswer(t, studio, "describe", map[string]any{"id": "icon.cpu"})
	entry, _ := described["entry"].(map[string]any)
	if entry["id"] != "icon.cpu" || entry["kind"] != "icon" {
		t.Fatalf("describe answer = %v", described)
	}
	hit := studio.catalogHitOf(cat.Search("server-cog", "icon", 1)[0])
	if !strings.Contains(string(hit.UsageExample), `"text":"Server cog"`) {
		t.Fatalf("label of a dashed name: %s", hit.UsageExample)
	}
}
