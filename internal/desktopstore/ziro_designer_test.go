package desktopstore

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestZiroDesignerCatalogUsesIsolatedStaticImage(t *testing.T) {
	var found bool
	for _, entry := range DefaultCatalog() {
		if entry.ID != "ziro-designer" {
			continue
		}
		found = true
		if entry.Category != "creative" || entry.Metadata["store_ui"] != "ziro-designer" || entry.PrimaryPort.ContainerPort != 8080 {
			t.Fatalf("unexpected Ziro catalog contract: %+v", entry)
		}
		if entry.Image != "ghcr.io/antibyte/aurago-ziro-designer:ziro-7b45970-1" || entry.LogoURL != "/img/desktop/store/ziro-designer.svg" {
			t.Fatal("Ziro must use pinned first-party image and logo")
		}
		if len(entry.HostBinds)+len(entry.WorkspaceBinds)+len(entry.Volumes)+len(entry.Companions)+len(entry.GeneratedSecrets) != 0 {
			t.Fatal("Ziro must not mount Shared, sockets or provider secrets")
		}
	}
	if !found {
		t.Fatal("Ziro Designer is missing")
	}
}

func TestZiroDesignerLifecyclePreservesSharedProjects(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	desktop := &fakeDesktopAdapter{}
	svc := newTestService(t, docker, desktop, &fakeLaunchpadAdapter{}, fixedPorts(19389))
	project := filepath.Join(svc.cfg.WorkspaceDir, "Shared", "Ziro Designer", "Board", "Board.kicad_pro")
	want := []byte("{\"native\":true}")
	if err := os.MkdirAll(filepath.Dir(project), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(project, want, 0o600); err != nil {
		t.Fatal(err)
	}
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "ziro-designer", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	if desktop.installed.Metadata["store_ui"] != "ziro-designer" {
		t.Fatal("desktop manifest lost the dedicated Ziro wrapper")
	}
	for _, operation := range []string{OperationStop, OperationStart, OperationRestart, OperationUpdate, OperationUninstall} {
		op, err := svc.StartAppOperation(ctx, "ziro-designer", operation, OperationRequest{DeleteData: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.RunOperation(ctx, op.ID); err != nil {
			t.Fatalf("%s: %v", operation, err)
		}
		got, err := os.ReadFile(project)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s modified the Shared project: %v", operation, err)
		}
	}
	if len(docker.created) != 2 || len(docker.pulled) != 2 {
		t.Fatal("install and update must create the catalog image")
	}
	for _, spec := range docker.created {
		if len(spec.HostBinds)+len(spec.Volumes) != 0 {
			t.Fatal("a Ziro container gained filesystem access")
		}
	}
	if len(docker.removedVolumes) != 0 {
		t.Fatal("Ziro uninstall must never delete project data")
	}
}
