//go:build linux

package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestWorkspaceWritesStayRooted(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	outsideFile := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(outsideFile, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{outsideFile, filepath.Join(outside, "new.txt")} {
		link := filepath.Join(dir, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		for _, appendMode := range []bool{false, true} {
			if err := writeWorkspaceData(root, "link", []byte("overwrite"), appendMode); err == nil {
				t.Fatal("followed final symlink outside root")
			}
		}
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
	}
	if data, err := os.ReadFile(outsideFile); err != nil || string(data) != "original" {
		t.Fatalf("outside file changed: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(outside, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("created outside file: %v", err)
	}
	if err := writeWorkspaceData(root, "nested/file.txt", []byte("first"), false); err != nil {
		t.Fatal(err)
	}
	if err := writeWorkspaceData(root, "nested/file.txt", []byte(" second"), true); err != nil {
		t.Fatal(err)
	}
	if data, err := root.ReadFile("nested/file.txt"); err != nil || string(data) != "first second" {
		t.Fatalf("normal append: %q %v", data, err)
	}
	if err := os.Symlink("nested/file.txt", filepath.Join(dir, "inside")); err != nil {
		t.Fatal(err)
	}
	if err := writeWorkspaceData(root, "inside", []byte("inside"), false); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceWriteParentSwapCannotEscape(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Mkdir(filepath.Join(dir, "inner"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("inner", filepath.Join(dir, "flip")); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	var swaps sync.WaitGroup
	swaps.Add(1)
	go func() {
		defer swaps.Done()
		for {
			for _, target := range []string{outside, "inner"} {
				select {
				case <-stop:
					return
				default:
				}
				next := filepath.Join(dir, "next")
				if os.Symlink(target, next) == nil {
					os.Rename(next, filepath.Join(dir, "flip"))
				}
			}
		}
	}()
	for i := 0; i < 100; i++ {
		_ = writeWorkspaceData(root, "flip/escape.txt", []byte("fixture"), i%2 == 0)
	}
	close(stop)
	swaps.Wait()
	if _, err := os.Stat(filepath.Join(outside, "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("write escaped during parent swap: %v", err)
	}
}
