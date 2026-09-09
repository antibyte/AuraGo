package desktopstore

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
	"aurago/internal/security"
)

func TestStoreCatalogIconsAreAcceptedByDesktop(t *testing.T) {
	for _, entry := range DefaultCatalog() {
		t.Run(entry.ID, func(t *testing.T) {
			if _, err := desktop.NormalizeDesktopIconName(entry.Icon, "desktop app"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGodsEyeInstallRegistersWithRealDesktop(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("42", 32), filepath.Join(root, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	desktopService, err := desktop.NewService(desktop.Config{
		Enabled: true, AllowGeneratedApps: true,
		WorkspaceDir: filepath.Join(root, "workspace"), DBPath: filepath.Join(root, "desktop.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	desktopService.SetIntegritySecretStore(vault)
	if err := desktopService.Init(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = desktopService.Close() })
	s := newTestServiceWithSecrets(t, &fakeDockerAdapter{}, desktopService, &fakeLaunchpadAdapter{}, fixedPorts(14173), vault)
	op, err := s.StartInstall(ctx, InstallRequest{AppID: GodsEyeAppID, BindMode: BindModeLocal, AllowedOrigins: []string{"http://localhost:8088"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("install using real desktop and vault: %v", err)
	}
	app, exists, err := s.GetInstalled(ctx, GodsEyeAppID)
	if err != nil || !exists || app.Status != AppStatusRunning {
		t.Fatalf("installed record missing: exists=%v status=%s err=%v", exists, app.Status, err)
	}
	bootstrap, err := desktopService.Bootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, manifest := range bootstrap.InstalledApps {
		if manifest.ID == DesktopAppID(GodsEyeAppID) && manifest.Icon == GodsEyeAppID {
			return
		}
	}
	t.Fatal("God's Eye View missing from real desktop bootstrap")
}
