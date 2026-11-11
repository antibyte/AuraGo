package upkeep

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaintenanceDatabasePathsOnlyParsesConfiguredStores(t *testing.T) {
	root := t.TempDir()
	const yaml = "sqlite:\n  planner_path: ./custom/planner.sqlite\n  launchpad_path: ./custom/launchpad.data\n  optimization_path: ./custom/optimization.db\n  virtual_computers_path: ./custom/computers.db\n  game_maker_path: ./custom/games.db\n"
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := RunCLI([]string{"--root", root, "--database-paths"}, &out, &stderr); code != 0 {
		t.Fatalf("code=%d: %s", code, &stderr)
	}
	for _, name := range []string{"planner.sqlite", "launchpad.data", "optimization.db", "computers.db", "games.db"} {
		if !strings.Contains(out.String(), filepath.Join(root, "custom", name)+"\x00") {
			t.Fatalf("missing configured %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "custom")); !os.IsNotExist(err) {
		t.Fatal("path listing opened/created database directory")
	}
	if _, err := os.Stat(filepath.Join(root, StateDir)); !os.IsNotExist(err) {
		t.Fatal("passive mode mutated update state")
	}
	if err := os.MkdirAll(filepath.Join(root, "custom", "planner.sqlite"), 0700); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := RunCLI([]string{"--root", root, "--database-paths"}, &out, &stderr); code == 0 || out.Len() != 0 {
		t.Fatal("non-file database produced a successful or partial list")
	}
	if err := os.Remove(filepath.Join(root, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := RunCLI([]string{"--root", root, "--database-paths"}, &out, &stderr); code == 0 || out.Len() != 0 {
		t.Fatal("unreadable config produced a successful or partial list")
	}
}
