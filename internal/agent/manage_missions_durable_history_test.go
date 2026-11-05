package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/tools"
)

func TestManageMissionsDurableHistoryUnavailableSourceIsUnknown(t *testing.T) {
	f := mm1c09New(t)
	db, err := tools.InitMissionHistoryDB(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f.mm.SetHistoryDB(db)
	err = tools.ProjectDurableMissionHistory(context.Background(), db, tools.DurableMissionHistory{
		RunID: "run_orphan", MissionID: f.flowID, Status: "waiting", StartedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	out := f.call(t, map[string]interface{}{"operation": "history", "id": f.flowID})
	if !strings.Contains(out, `"status":"unknown"`) || !strings.Contains(out, `"last_known_status":"waiting"`) {
		t.Fatalf("agent history did not identify the unavailable source: %q", out)
	}
}
