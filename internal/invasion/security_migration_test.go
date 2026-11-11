package invasion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEggSecurityMigrationBacksUpAndDeniesImplicitGrants(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invasion.db")
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	id, err := CreateEgg(db, EggRecord{Name: "old", AllowedTools: `["shell"]`})
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range eggSecurityColumns {
		if _, err := db.Exec("ALTER TABLE " + column.table + " DROP COLUMN " + column.name); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	db, err = InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	egg, err := GetEgg(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if egg.AllowFilesystemWrite || egg.AllowNetworkRequests || egg.AllowUnsandboxedShell {
		t.Fatal("legacy rights were implicitly granted")
	}
	if _, err := os.Stat(path + ".pre-egg-security.bak"); err != nil {
		t.Fatal("migration backup missing:", err)
	}
}

func TestPrivateDockerBootstrapRefusesPlaintext(t *testing.T) {
	for _, method := range []string{"docker_remote", "docker_local", "docker_ssh"} {
		err := requirePrivateDockerBootstrap(NestRecord{DeployMethod: method})
		if (err != nil) != (method == "docker_remote") {
			t.Fatalf("%s: %v", method, err)
		}
	}
	if err := requirePrivateDockerBootstrap(NestRecord{DeployMethod: "docker_remote", DockerTLS: DockerTLSServer}); err != nil {
		t.Fatal(err)
	}
}
