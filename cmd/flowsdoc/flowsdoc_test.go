package main

import (
	"maps"
	"os"
	"path/filepath"
	"testing"
)

const regenerate = "run `go run ./cmd/flowsdoc` from the repository root and commit the result"

// The manuals' reference blocks are generated from the node catalog. This test renders
// every block in memory and compares it with the file, so a changed label, description or
// effect (or a deleted marker) cannot reach main without the regenerated documents.
func TestFlowsDocBlocksMatchTheCatalog(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	src, err := loadSource(root)
	if err != nil {
		t.Fatal(err)
	}
	read := func(tg target) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(tg.path)))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	t.Run("every generated block is current", func(t *testing.T) {
		for _, tg := range targets {
			doc := read(tg)
			next, err := src.apply(tg, doc)
			if err != nil {
				t.Errorf("%v; restore the marker pair, then %s", err, regenerate)
				continue
			}
			if next != doc {
				t.Errorf("%s: the generated blocks differ from the catalog; %s", tg.path, regenerate)
			}
		}
	})

	t.Run("a document without a marker pair fails", func(t *testing.T) {
		for name, doc := range map[string]string{
			"no markers":             "# Chapter\n\nText.\n",
			"a begin without an end": "<!-- BEGIN GENERATED: flows-triggers -->\n<!-- BEGIN GENERATED: flows-steps -->\n<!-- END GENERATED: flows-steps -->\n",
		} {
			if _, err := src.apply(targets[0], doc); err == nil {
				t.Errorf("%s: apply accepted the document", name)
			}
		}
	})

	t.Run("a changed catalog label is detected", func(t *testing.T) {
		tg := targets[0]
		current := read(tg)
		locales := maps.Clone(src.locales)
		en := maps.Clone(src.locales["en"])
		en["easydrag.node.trigger_manual.label"] += " (renamed)"
		locales["en"] = en
		next, err := (&source{reg: src.reg, locales: locales}).apply(tg, current)
		if err != nil {
			t.Fatal(err)
		}
		if next == current {
			t.Fatal("renaming a label did not change the generated block, so drift would go unnoticed")
		}
	})
}
