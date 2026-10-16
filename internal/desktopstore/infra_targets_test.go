package desktopstore

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadInfraTargetsUsesExistingRowsWithoutStoreInitialization(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE desktop_store_apps (
		app_id TEXT, container_id TEXT, status TEXT, env_json TEXT
	);
	INSERT INTO desktop_store_apps VALUES
		('reader', 'container-reader', 'running', '["secret"]'),
		('paused', 'container-paused', 'stopped', '["secret"]'),
		('installing', 'container-installing', 'installing', '["secret"]'),
		('missing-id', '', 'running', '["secret"]');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := ReadInfraTargets(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []InfraTarget{{AppID: "paused", ContainerID: "container-paused"}, {AppID: "reader", ContainerID: "container-reader"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("targets = %#v, want %#v", got, want)
	}

	missingPath := filepath.Join(t.TempDir(), "missing.db")
	got, err = ReadInfraTargets(context.Background(), missingPath)
	if err != nil || len(got) != 0 {
		t.Fatalf("missing store targets = %#v, %v", got, err)
	}
	if _, err := os.Stat(missingPath); !os.IsNotExist(err) {
		t.Fatalf("reading a missing target database created it: stat error=%v", err)
	}
}
