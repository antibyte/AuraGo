package flows

import "testing"

// file.write keeps its default "overwrite" (owner decision 2 of the S0c spec). Two things pin it:
// the parameter's declared default, and the behaviour of a node that sets no if_exists: it writes
// over an existing file, with one write call and no probe for the name.
func TestFileWriteDefaultsToOverwrite(t *testing.T) {
	def := lookupDef(t, docRegistry(t, nil), TypeFileWrite)
	var spec *ParamSpec
	for i := range def.Params {
		if def.Params[i].Name == "if_exists" {
			spec = &def.Params[i]
		}
	}
	if spec == nil || spec.Default != "overwrite" {
		t.Fatalf("if_exists spec = %+v, want default %q", spec, "overwrite")
	}

	tools := existsFake(func(string) bool { return true }) // the file already exists
	res, err := execDef(def, map[string]any{"path": "a.txt", "content": "x"}, &Services{Tools: tools})
	if err != nil {
		t.Fatalf("a node without if_exists must overwrite an existing file, got %v", err)
	}
	calls := tools.allCalls()
	writes, probes := opsOf(calls, "write_file"), opsOf(calls, "stat")
	if len(calls) != 1 || len(writes) != 1 || writes[0] != "a.txt" || len(probes) != 0 {
		t.Fatalf("calls = %d, writes = %q, probes = %q; want one write of a.txt and no probe", len(calls), writes, probes)
	}
	if got := res.Output["file"].(map[string]any)["path"]; got != "a.txt" {
		t.Fatalf("output path = %v, want a.txt (no renamed copy)", got)
	}
}

// Only file.write's if_exists carries card labels; they resolve like the option labels and are
// sent as card_label. Without a translation the card falls back to nothing (the client then uses
// the option label).
func TestFileWriteOptionsCarryCardLabels(t *testing.T) {
	def := lookupDef(t, docRegistry(t, nil), TypeFileWrite)
	tr := func(key string) string { return "T:" + key }
	for _, p := range def.Params {
		if p.Name != "if_exists" {
			continue
		}
		info := describeParam(p, tr)
		want := map[string]string{
			"overwrite": "T:easydrag.option.if_exists_overwrite.card",
			"unique":    "T:easydrag.option.if_exists_unique.card",
			"fail":      "T:easydrag.option.if_exists_fail.card",
		}
		if len(info.Options) != len(want) {
			t.Fatalf("options = %+v", info.Options)
		}
		for _, o := range info.Options {
			if o.CardLabel != want[o.Value] || o.Label == "" {
				t.Errorf("option %s: card label %q, label %q; want card label %q", o.Value, o.CardLabel, o.Label, want[o.Value])
			}
		}
	}
	// Every other option in the catalog keeps no card label.
	reg := catalogRegistry(t, fullEnv())
	for _, d := range reg.All() {
		if d.Type == TypeFileWrite {
			continue
		}
		for _, p := range d.Params {
			for _, o := range p.Options {
				if o.CardLabelKey != "" {
					t.Errorf("%s.%s option %s has a card label key", d.Type, p.Name, o.Value)
				}
			}
		}
	}
}
