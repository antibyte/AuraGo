package invasion

import (
	"strings"
	"testing"
)

func TestGetNestByNameRejectsAmbiguousNames(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	first, err := CreateNest(db, NestRecord{Name: "BÜRO", AccessType: "ssh", Host: "192.0.2.1"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := GetNestByName(db, "büro"); err != nil || got.ID != first {
		t.Fatalf("unique name: %+v, %v", got, err)
	}
	if _, err := CreateNest(db, NestRecord{Name: "büro", AccessType: "ssh", Host: "192.0.2.2"}); err != nil {
		t.Fatal(err)
	}
	if got, err := GetNestByName(db, "Büro"); err == nil || !strings.Contains(err.Error(), "nest_id") || got.ID != "" {
		t.Fatalf("ambiguous name selected a nest: %+v, %v", got, err)
	}
	if got, err := GetNest(db, first); err != nil || got.ID != first {
		t.Fatalf("explicit ID no longer works: %+v, %v", got, err)
	}
	if _, err := GetNestByName(db, "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing name: %v", err)
	}
}
