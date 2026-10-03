package gamemaker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/fileutil"
)

func TestFilesystemReceiptRecoversEveryPublicationBoundary(t *testing.T) {
	for _, phase := range []string{"receipt", "backup", "swap", "commit"} {
		t.Run(phase, func(t *testing.T) {
			s := newTestService(t)
			project := createTestProject(t, s, "2d")
			initial := filepath.Join(s.stagingDir, "initial")
			if err := os.MkdirAll(initial, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(initial, "marker"), []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.publish(context.Background(), initial, project, Job{}, "test", "initial"); err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(s.stagingDir, "candidate")
			if err := os.MkdirAll(stage, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stage, "marker"), []byte("new"), 0600); err != nil {
				t.Fatal(err)
			}
			r := filesystemReceipt{Version: 1, ID: "testpublish", Kind: "publish", ProjectID: project.ID, ProjectKey: project.ProjectKey, Stage: "candidate", PreviousRevision: 1, Revision: 2, HadTarget: true}
			path, err := s.writeFilesystemReceipt(r)
			if err != nil {
				t.Fatal(err)
			}
			target, backup, _, err := s.receiptPaths(r)
			if err != nil {
				t.Fatal(err)
			}
			if phase != "receipt" {
				if err := fileutil.Rename(target, backup); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "swap" || phase == "commit" {
				if err := fileutil.Rename(stage, target); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "commit" {
				if _, err := s.db.Exec("UPDATE gm_projects SET current_revision=2 WHERE id=?", project.ID); err != nil {
					t.Fatal(err)
				}
			}
			// This is the same recovery routine executed before NewService exposes data.
			if err := s.recoverFilesystemReceipts(context.Background()); err != nil {
				t.Fatal(err)
			}
			want := "old"
			if phase == "commit" {
				want = "new"
			}
			got, err := os.ReadFile(filepath.Join(target, "marker"))
			if err != nil || string(got) != want {
				t.Fatalf("recovered %q, %v; want %q", got, err, want)
			}
			if phase != "commit" {
				got, err := os.ReadFile(filepath.Join(stage, "marker"))
				if err != nil || string(got) != "new" {
					t.Fatalf("lost candidate: %q %v", got, err)
				}
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("receipt remains: %v", err)
			}
			if err := s.recoverFilesystemReceipts(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSnapshotRejectsCorruptExistingBlob(t *testing.T) {
	s := newTestService(t)
	stage := filepath.Join(s.stagingDir, "blob-test")
	if err := os.MkdirAll(stage, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "file"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	files, _, err := s.snapshotFiles(stage)
	if err != nil {
		t.Fatal(err)
	}
	hash := files[0].Hash
	if err := os.WriteFile(filepath.Join(s.blobDir, hash[:2], hash), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.snapshotFiles(stage); err == nil {
		t.Fatal("corrupt blob was accepted")
	}
}
