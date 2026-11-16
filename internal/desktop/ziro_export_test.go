package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type ziroTestDocker struct {
	fakeCodeContainerDocker
	t      *testing.T
	create func(CodeDockerCreateRequest)
	cancel context.CancelFunc
}

func (d *ziroTestDocker) CreateContainer(ctx context.Context, req CodeDockerCreateRequest) (string, error) {
	if req.Image != ZiroExportImage || req.NetworkMode != "none" || len(req.Ports) != 0 || !req.ReadonlyRootfs || len(req.Volumes) != 1 || req.CapDrop[0] != "ALL" || req.Resources.PidsLimit != 128 {
		d.t.Fatal("export isolation missing")
	}
	if d.create != nil {
		d.create(req)
	}
	if d.cancel != nil {
		d.cancel()
	}
	return req.Name, nil
}

func TestZiroExportUsesSnapshotAndCleansIsolatedContainer(t *testing.T) {
	svc := testService(t)
	for _, name := range []string{"board.kicad_sch", "board.kicad_pcb", "board.kicad_pro", "symbols/local.kicad_sym"} {
		if err := svc.WriteFile(t.Context(), "Shared/Ziro/demo/"+name, "saved-content", "user"); err != nil {
			t.Fatal(err)
		}
	}
	jobs := t.TempDir()
	for _, operation := range []string{"netlist", "manufacturing", "probe"} {
		t.Run(operation, func(t *testing.T) {
			docker := &ziroTestDocker{t: t}
			docker.create = func(req CodeDockerCreateRequest) {
				job := strings.TrimSuffix(req.Volumes[0], ":/job")
				data, err := os.ReadFile(filepath.Join(job, "request.json"))
				if err != nil {
					t.Fatal(err)
				}
				var request map[string]any
				if json.Unmarshal(data, &request) != nil || request["operation"] != operation {
					t.Fatal("request mismatch")
				}
				name, result := "result.net", "(export (version D))"
				if operation == "manufacturing" {
					name, result = "result.zip", "zip-fixture"
				}
				if operation == "probe" {
					name, result = "probe.json", `{"version":"10.0.8","protocol":1}`
				} else {
					copied, err := os.ReadFile(filepath.Join(job, "source", "symbols", "local.kicad_sym"))
					if err != nil || string(copied) != "saved-content" {
						t.Fatal("relative library missing")
					}
					if err := os.WriteFile(filepath.Join(job, "source", "board.kicad_sch"), []byte("worker change"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(filepath.Join(job, name), []byte(result), 0600); err != nil {
					t.Fatal(err)
				}
			}
			data, _, err := svc.RunZiroExport(t.Context(), docker, jobs, "Shared/Ziro/demo/board.kicad_pro", operation, 1<<20)
			if err != nil || len(data) == 0 {
				t.Fatalf("export: %v", err)
			}
			if len(docker.actions) != 3 || !strings.HasSuffix(docker.actions[1], " kill") || !strings.HasSuffix(docker.actions[2], " remove") {
				t.Fatal("container not cleaned")
			}
			entries, _ := os.ReadDir(jobs)
			if len(entries) != 0 {
				t.Fatal("snapshot retained")
			}
			original, _ := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, "Shared", "Ziro", "demo", "board.kicad_sch"))
			if string(original) != "saved-content" {
				t.Fatal("source changed")
			}
		})
	}
}

func TestZiroExportRejectsUnsafeSourcesAndCancellation(t *testing.T) {
	svc := testService(t)
	if err := svc.WriteFile(t.Context(), "Shared/demo/board.kicad_sch", "saved", "user"); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"Documents/demo/board.kicad_sch", "Shared/demo/../board.kicad_sch", "Shared/board.kicad_sch", "Shared/demo/missing.kicad_sch"} {
		if _, _, err := svc.RunZiroExport(t.Context(), &ziroTestDocker{t: t}, t.TempDir(), source, "netlist", 100); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
	if _, err := svc.snapshotZiroProject(t.Context(), "Shared/demo/board.kicad_sch", "netlist", filepath.Join(t.TempDir(), "snapshot"), 2); err == nil {
		t.Fatal("size limit bypassed")
	}
	ctx, cancel := context.WithCancel(t.Context())
	docker := &ziroTestDocker{t: t, cancel: cancel}
	if _, _, err := svc.RunZiroExport(ctx, docker, t.TempDir(), "Shared/demo/board.kicad_sch", "netlist", 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if len(docker.actions) != 2 {
		t.Fatal("cancelled container not killed and removed")
	}
	link := filepath.Join(svc.Config().WorkspaceDir, "Shared", "demo", "linked.kicad_sym")
	if err := os.Symlink(filepath.Join(svc.Config().WorkspaceDir, "Shared", "demo", "board.kicad_sch"), link); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, err := svc.snapshotZiroProject(t.Context(), "Shared/demo/board.kicad_sch", "netlist", filepath.Join(t.TempDir(), "snapshot"), 100); err == nil {
		t.Fatal("link accepted")
	}
}
