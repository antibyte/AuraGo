package desktop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestHASwitchboardPersistenceAndValidation(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	board, err := svc.HASwitchboard(ctx)
	if err != nil || board.Version != 1 || len(board.Switches) != 0 {
		t.Fatalf("empty board: %+v %v", board, err)
	}
	valid := `{"version":1,"switches":[{"entity_id":"switch.desk","label":"Schreibtisch"},{"entity_id":"switch.garden"}]}`
	if err := svc.SetSetting(ctx, HASwitchboardSetting, valid, SourceUser); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		`{}`, `null`, `{"version":2,"switches":[]}`, `{"version":1,"switches":null}`,
		`{"version":1,"switches":[],"secret":"x"}`, valid + `{}`,
		`{"version":1,"switches":[{"entity_id":"light.desk"}]}`,
		`{"version":1,"switches":[{"entity_id":"switch.desk/../../"}]}`,
		`{"version":1,"switches":[{"entity_id":"switch.desk,switch.other"}]}`,
		`{"version":1,"switches":[{"entity_id":"switch.desk","service":"toggle"}]}`,
		`{"version":1,"switches":[{"entity_id":"switch.desk"},{"entity_id":"switch.desk"}]}`,
		`{"version":1,"switches":[{"entity_id":"switch.desk","label":"line\nbreak"}]}`,
		`{"version":1,"switches":[{"entity_id":"switch.desk","label":"` + strings.Repeat("a", 81) + `"}]}`,
		strings.Repeat(" ", 16*1024+1),
	} {
		if err := svc.SetSetting(ctx, HASwitchboardSetting, invalid, SourceUser); err == nil {
			t.Fatalf("accepted invalid board: %.160s", invalid)
		}
	}
	// Reopen the real SQLite database to verify durable order/labels after rejected saves.
	cfg := svc.Config()
	_ = svc.Close()
	reopened, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	board, err = reopened.HASwitchboard(ctx)
	if err != nil || len(board.Switches) != 2 || board.Switches[0].Label != "Schreibtisch" || board.Switches[1].EntityID != "switch.garden" {
		t.Fatalf("reopen: %+v %v", board, err)
	}
	tooMany := HASwitchboard{Version: 1, Switches: make([]HASwitchboardEntry, 61)}
	for i := range tooMany.Switches {
		tooMany.Switches[i].EntityID = "switch." + strings.Repeat("a", i+1)
	}
	data, _ := json.Marshal(tooMany)
	if _, err := ParseHASwitchboard(string(data)); err == nil {
		t.Fatal("accepted more than 60 switches")
	}
	app := testFindApp(t, BuiltinApps(), "ha-switchboard")
	if app.Icon != "ha-switchboard" || app.Entry != "builtin://ha-switchboard" || !app.Builtin {
		t.Fatalf("app registration: %+v", app)
	}
}
