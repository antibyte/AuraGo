// Command flowsdoc rewrites the generated blocks of the EasyDrag manuals and of the
// manage_missions tool manual from the node catalog.
//
// Run it from anywhere inside the repository after a label, a description or an effect of
// a building block changed:
//
//	go run ./cmd/flowsdoc
//
// It rewrites only the text between "<!-- BEGIN GENERATED: name -->" and
// "<!-- END GENERATED: name -->". Its output has no dates, a fixed order and LF line ends.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "flowsdoc:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	src, err := loadSource(root)
	if err != nil {
		return err
	}
	for _, tg := range targets {
		path := filepath.Join(root, filepath.FromSlash(tg.path))
		old, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		next, err := src.apply(tg, string(old))
		if err != nil {
			return err
		}
		if next == string(old) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(next), info.Mode().Perm()); err != nil {
			return err
		}
		fmt.Println("updated", tg.path)
	}
	return nil
}

// repoRoot is the first directory, from the working directory upwards, that holds go.mod.
// go run starts in the module root, go test in the package directory.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found above the working directory")
		}
		dir = parent
	}
}
