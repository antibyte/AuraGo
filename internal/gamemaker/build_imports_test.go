package gamemaker

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func buildImportFixture(t *testing.T, dimension, source string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "project")
	for name, body := range map[string]string{
		"game.json":   `{"name":"imports","dimension":` + strconv.Quote(dimension) + `,"entry":"src/main.ts"}`,
		"index.html":  scaffoldHTML(dimension, "imports"),
		"src/main.ts": source,
	} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestBuildDirectoryRejectsOutsideImports(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		for _, kind := range []string{"json", "text", "indirect", "absolute"} {
			t.Run(dimension+"/"+kind, func(t *testing.T) {
				dir := buildImportFixture(t, dimension, "")
				outside := filepath.Join(filepath.Dir(dir), "outside.json")
				if err := os.WriteFile(outside, []byte(`{"value":"outside-private-marker"}`), 0o600); err != nil {
					t.Fatal(err)
				}
				source := `import value from "../../outside.json"; console.log(value);`
				switch kind {
				case "text":
					outside = filepath.Join(filepath.Dir(dir), "outside.yaml")
					if err := os.WriteFile(outside, []byte("value: outside-private-marker\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					source = `import value from "../../outside.yaml" with { type: "text" }; console.log(value);`
				case "indirect":
					if err := os.WriteFile(filepath.Join(dir, "src", "child.ts"), []byte(source), 0o640); err != nil {
						t.Fatal(err)
					}
					source = `import "./child";`
				case "absolute":
					source = "import value from " + strconv.Quote(filepath.ToSlash(outside)) + "; console.log(value);"
				}
				if err := os.WriteFile(filepath.Join(dir, "src", "main.ts"), []byte(source), 0o640); err != nil {
					t.Fatal(err)
				}
				result := buildDirectory(context.Background(), dir, 100, 64<<20)
				if result.OK || len(result.Diagnostics) == 0 || !strings.Contains(result.Diagnostics[0].Message, "imports must stay inside the game project") {
					t.Fatalf("expected the import boundary to refuse this build: %+v", result)
				}
				if bundle, err := os.ReadFile(filepath.Join(dir, "dist", "game.js")); err == nil {
					t.Fatalf("failed build wrote a bundle: %s", bundle)
				}
			})
		}
	}
}

func TestBuildDirectoryKeepsProjectImportsAndVendorExternals(t *testing.T) {
	dir := buildImportFixture(t, "2d", `import value from "./nested/inside"; import "../vendor/phaser.min.js"; console.log(value);`)
	if err := os.MkdirAll(filepath.Join(dir, "src", "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "nested", "inside.ts"), []byte(`export { default } from "../../local.json";`), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "local.json"), []byte(`{"value":"inside-project-marker"}`), 0o640); err != nil {
		t.Fatal(err)
	}
	result := buildDirectory(context.Background(), dir, 100, 64<<20)
	if !result.OK {
		t.Fatalf("valid project imports failed: %+v", result.Diagnostics)
	}
	bundle, err := os.ReadFile(filepath.Join(dir, "dist", "game.js"))
	if err != nil || !strings.Contains(string(bundle), "inside-project-marker") || !strings.Contains(string(bundle), "../vendor/phaser.min.js") {
		t.Fatalf("project imports changed: %v", err)
	}
}

func TestBuildDirectoryRejectsSymlinkImports(t *testing.T) {
	dir := buildImportFixture(t, "3d", `import value from "./linked.json"; console.log(value);`)
	outside := filepath.Join(filepath.Dir(dir), "outside.json")
	if err := os.WriteFile(outside, []byte(`{"value":"outside-private-marker"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "src", "linked.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if result := buildDirectory(context.Background(), dir, 100, 64<<20); result.OK || len(result.Diagnostics) == 0 || !strings.Contains(result.Diagnostics[0].Message, "imports must stay inside the game project") {
		t.Fatalf("expected the import boundary to refuse the symlink: %+v", result)
	}
}

func TestBuildDirectoryIgnoresAncestorTypeScriptConfig(t *testing.T) {
	dir := buildImportFixture(t, "2d", `console.log("local source");`)
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "tsconfig.json"), []byte("invalid ancestor configuration"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := buildDirectory(context.Background(), dir, 100, 64<<20); !result.OK {
		t.Fatalf("ancestor configuration affected the project: %+v", result.Diagnostics)
	}
}
