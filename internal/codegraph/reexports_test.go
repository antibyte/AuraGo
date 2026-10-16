package codegraph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGraphReexportDoesNotResolvePrivateLocalSymbol(t *testing.T) {
	for _, ext := range []string{"js", "ts"} {
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			for name, source := range map[string]string{
				"actual":   "export function Target() {}",
				"provider": "function Target() {}\nexport {Target} from './actual';",
				"main":     "import {Target} from './provider';\nfunction Caller() { Target(); }",
			} {
				if err := os.WriteFile(filepath.Join(dir, name+"."+ext), []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			m := New(t.Context(), nil, nil, nil)
			defer m.Close()
			r, err := m.Query(t.Context(), Project{ID: "exports", Root: dir}, Request{Operation: "callees", Symbol: "Caller", File: "main." + ext})
			if err != nil || !r.Status.Complete || len(r.Edges) != 1 {
				t.Fatal(r, err)
			}
			if r.Edges[0].To != "" || r.Edges[0].Reason == "" {
				t.Fatalf("re-export resolved to a private local symbol: %+v", r.Edges)
			}
		})
	}
}
