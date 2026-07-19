package server

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPublishFileNoReplacePreservesExistingTarget(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, ".upload.part")
	targetPath := filepath.Join(dir, "manual.txt")
	if err := os.WriteFile(tempPath, []byte("new content"), 0o640); err != nil {
		t.Fatalf("write temporary file: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("existing content"), 0o640); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	err := publishFileNoReplace(tempPath, targetPath)
	if !errors.Is(err, errAtomicPublishTargetExists) {
		t.Fatalf("publishFileNoReplace error = %v, want errAtomicPublishTargetExists", err)
	}
	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(raw) != "existing content" {
		t.Fatalf("target content = %q, want existing content", raw)
	}
}

func TestPublishFileNoReplaceCompetesAtomicallyWithExclusiveWebUpload(t *testing.T) {
	for attempt := 0; attempt < 25; attempt++ {
		dir := t.TempDir()
		tempPath := filepath.Join(dir, ".agodesk.part")
		targetPath := filepath.Join(dir, "shared.txt")
		if err := os.WriteFile(tempPath, []byte("agodesk"), 0o640); err != nil {
			t.Fatalf("write temporary file: %v", err)
		}

		start := make(chan struct{})
		var wait sync.WaitGroup
		wait.Add(2)
		results := make(chan string, 2)
		go func() {
			defer wait.Done()
			<-start
			if err := publishFileNoReplace(tempPath, targetPath); err == nil {
				results <- "agodesk"
			}
		}()
		go func() {
			defer wait.Done()
			<-start
			file, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
			if err != nil {
				return
			}
			if _, err := file.WriteString("web"); err == nil && file.Close() == nil {
				results <- "web"
				return
			}
			_ = file.Close()
		}()
		close(start)
		wait.Wait()
		close(results)

		var winners []string
		for winner := range results {
			winners = append(winners, winner)
		}
		if len(winners) != 1 {
			t.Fatalf("attempt %d winners = %v, want exactly one", attempt, winners)
		}
		raw, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("attempt %d read target: %v", attempt, err)
		}
		if string(raw) != winners[0] {
			t.Fatalf("attempt %d target content = %q, winner = %q", attempt, raw, winners[0])
		}
	}
}

func TestPublishFileNoReplaceRejectsDifferentDirectories(t *testing.T) {
	tempPath := filepath.Join(t.TempDir(), ".upload.part")
	if err := os.WriteFile(tempPath, []byte("content"), 0o640); err != nil {
		t.Fatalf("write temporary file: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "manual.txt")
	if err := publishFileNoReplace(tempPath, targetPath); err == nil {
		t.Fatal("publishFileNoReplace accepted files in different directories")
	}
	if _, err := os.Stat(targetPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected target state: %v", err)
	}
}
