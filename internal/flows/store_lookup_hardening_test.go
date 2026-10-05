package flows

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// mustCreateFlow stores a sample flow under a mission id and fails the test on error.
func mustCreateFlow(t *testing.T, s *Store, id, missionID string) {
	t.Helper()
	if _, err := s.CreateFlow(context.Background(), sampleFlow(id), missionID, storeNow); err != nil {
		t.Fatalf("CreateFlow(%s, %q): %v", id, missionID, err)
	}
}

func TestGetFlowByMissionReturnsTheStoredRecord(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	mustCreateFlow(t, s, "flow_aaaaaaaaeb", "mission_20")
	mustCreateFlow(t, s, "flow_aaaaaaaaec", "mission_21")
	if _, err := s.Publish(ctx, "flow_aaaaaaaaeb", 1, storeNow); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetFlowByMission(ctx, "mission_20")
	if err != nil {
		t.Fatalf("GetFlowByMission: %v", err)
	}
	want, err := s.GetFlow(ctx, "flow_aaaaaaaaeb")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetFlowByMission = %+v, want the record GetFlow returns: %+v", got, want)
	}
	if got.Live == nil || got.MissionID != "mission_20" {
		t.Fatalf("the published flow must come back with its live revision and mission: %+v", got)
	}
	other, err := s.GetFlowByMission(ctx, "mission_21")
	if err != nil || other.ID != "flow_aaaaaaaaec" || other.Live != nil {
		t.Fatalf("GetFlowByMission(mission_21) = %+v, %v", other, err)
	}
}

func TestGetFlowByMissionFollowsRelinkAndDelete(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	mustCreateFlow(t, s, "flow_aaaaaaaaed", "")
	if _, err := s.GetFlowByMission(ctx, "mission_30"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("before linking: %v, want ErrNotFound", err)
	}
	if err := s.SetMissionID(ctx, "flow_aaaaaaaaed", "mission_30"); err != nil {
		t.Fatal(err)
	}
	rec, err := s.GetFlowByMission(ctx, "mission_30")
	if err != nil || rec.ID != "flow_aaaaaaaaed" {
		t.Fatalf("after SetMissionID: %+v, %v", rec, err)
	}
	if err := s.SetMissionID(ctx, "flow_aaaaaaaaed", "mission_31"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetFlowByMission(ctx, "mission_30"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the old mission after a relink: %v, want ErrNotFound", err)
	}
	if rec, err = s.GetFlowByMission(ctx, "mission_31"); err != nil || rec.ID != "flow_aaaaaaaaed" {
		t.Fatalf("the new mission after a relink: %+v, %v", rec, err)
	}
	if err := s.DeleteFlow(ctx, "flow_aaaaaaaaed"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetFlowByMission(ctx, "mission_31"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after deleting the flow: %v, want ErrNotFound", err)
	}
}

// Two flows with the same mission id must never resolve to either of them:
// Mission Control would run the wrong flow.
func TestGetFlowByMissionAmbiguousFailsClosed(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	mustCreateFlow(t, s, "flow_aaaaaaaaee", "mission_9")
	mustCreateFlow(t, s, "flow_aaaaaaaaef", "mission_9")
	mustCreateFlow(t, s, "flow_aaaaaaaaeg", "mission_10")

	rec, err := s.GetFlowByMission(ctx, "mission_9")
	if !errors.Is(err, ErrMissionAmbiguous) || rec != nil {
		t.Fatalf("GetFlowByMission(two flows) = %+v, %v, want nil and ErrMissionAmbiguous", rec, err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("an ambiguous mission must not look like a missing one: %v", err)
	}
	if !strings.Contains(err.Error(), "mission_9") {
		t.Fatalf("the error should name the mission: %v", err)
	}

	// A mission with exactly one flow is unaffected.
	if rec, err = s.GetFlowByMission(ctx, "mission_10"); err != nil || rec.ID != "flow_aaaaaaaaeg" {
		t.Fatalf("GetFlowByMission(one flow) = %+v, %v", rec, err)
	}

	// Three flows are ambiguous too, and removing the extras resolves it again.
	mustCreateFlow(t, s, "flow_aaaaaaaaeh", "mission_9")
	if _, err = s.GetFlowByMission(ctx, "mission_9"); !errors.Is(err, ErrMissionAmbiguous) {
		t.Fatalf("GetFlowByMission(three flows) = %v, want ErrMissionAmbiguous", err)
	}
	if err = s.DeleteFlow(ctx, "flow_aaaaaaaaeh"); err != nil {
		t.Fatal(err)
	}
	if err = s.SetMissionID(ctx, "flow_aaaaaaaaef", "mission_11"); err != nil {
		t.Fatal(err)
	}
	if rec, err = s.GetFlowByMission(ctx, "mission_9"); err != nil || rec.ID != "flow_aaaaaaaaee" {
		t.Fatalf("after the other flows left the mission: %+v, %v", rec, err)
	}
}

// SetMissionID does not guard against a second owner, so the ambiguity can also
// arise through it. The lookup must fail closed there as well.
func TestGetFlowByMissionAmbiguousThroughSetMissionID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	mustCreateFlow(t, s, "flow_aaaaaaaaei", "")
	mustCreateFlow(t, s, "flow_aaaaaaaaej", "")
	for _, id := range []string{"flow_aaaaaaaaei", "flow_aaaaaaaaej"} {
		if err := s.SetMissionID(ctx, id, "mission_12"); err != nil {
			t.Fatal(err)
		}
	}
	if rec, err := s.GetFlowByMission(ctx, "mission_12"); !errors.Is(err, ErrMissionAmbiguous) || rec != nil {
		t.Fatalf("GetFlowByMission = %+v, %v, want nil and ErrMissionAmbiguous", rec, err)
	}
}

func TestGetFlowByMissionAmbiguityEchoIsBounded(t *testing.T) {
	s := openTestStore(t)
	huge := "mission_" + strings.Repeat("m", 100000)
	mustCreateFlow(t, s, "flow_aaaaaaaaek", huge)
	mustCreateFlow(t, s, "flow_aaaaaaaael", huge)
	_, err := s.GetFlowByMission(context.Background(), huge)
	if !errors.Is(err, ErrMissionAmbiguous) {
		t.Fatalf("GetFlowByMission(huge id, two flows) = %v, want ErrMissionAmbiguous", err)
	}
	// The sentinel, ": ", the quotes, the id cut to maxErrorEchoRunes and one ellipsis.
	// The id is ASCII, so quoting adds no escapes.
	limit := utf8.RuneCountInString(ErrMissionAmbiguous.Error()) + len(": ") + 2 + maxErrorEchoRunes + 1
	if got := utf8.RuneCountInString(err.Error()); got > limit {
		t.Fatalf("the error echoes the mission id unbounded (%d runes, limit %d): %.200q", got, limit, err)
	}
}

// Flows without a mission share the empty id. That is the "not linked" state,
// not a mission, so the lookup never reports it as ambiguous.
func TestGetFlowByMissionIgnoresUnlinkedFlows(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	mustCreateFlow(t, s, "flow_aaaaaaaaem", "")
	mustCreateFlow(t, s, "flow_aaaaaaaaen", "")
	rec, err := s.GetFlowByMission(ctx, "")
	if !errors.Is(err, ErrNotFound) || rec != nil {
		t.Fatalf("GetFlowByMission(\"\") = %+v, %v, want nil and ErrNotFound", rec, err)
	}
}

func TestGetFlowByMissionFollowsTheContext(t *testing.T) {
	s := openTestStore(t)
	mustCreateFlow(t, s, "flow_aaaaaaaaeo", "mission_40")

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()

	cases := []struct {
		name    string
		ctx     context.Context
		mission string
		want    error
	}{
		{"cancelled, existing mission", cancelled, "mission_40", context.Canceled},
		{"cancelled, missing mission", cancelled, "mission_41", context.Canceled},
		{"expired, existing mission", expired, "mission_40", context.DeadlineExceeded},
		{"expired, missing mission", expired, "mission_41", context.DeadlineExceeded},
	}
	for _, tc := range cases {
		rec, err := s.GetFlowByMission(tc.ctx, tc.mission)
		if !errors.Is(err, tc.want) || rec != nil {
			t.Errorf("%s: GetFlowByMission = %+v, %v, want nil and %v", tc.name, rec, err, tc.want)
		}
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrMissionAmbiguous) {
			t.Errorf("%s: an interrupted lookup must not look like a verdict: %v", tc.name, err)
		}
	}
}

// A damaged document surfaces the way GetFlow reports it, never as ErrNotFound:
// a row that exists but cannot be read must not look like an unknown mission.
func TestGetFlowByMissionSurfacesDamagedRows(t *testing.T) {
	ctx := context.Background()
	setup := func(t *testing.T, column, value string) (*Store, string) {
		t.Helper()
		s := openTestStore(t)
		mustCreateFlow(t, s, "flow_aaaaaaaaep", "mission_50")
		if _, err := s.Publish(ctx, "flow_aaaaaaaaep", 1, storeNow); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE flows SET `+column+` = ? WHERE id = ?`, value, "flow_aaaaaaaaep"); err != nil {
			t.Fatal(err)
		}
		return s, "flow_aaaaaaaaep"
	}

	cases := []struct {
		name, column, value string
		check               func(t *testing.T, err error)
	}{
		{"draft is not JSON", "draft_json", "{not json", func(t *testing.T, err error) {
			var syntax *json.SyntaxError
			if !errors.As(err, &syntax) {
				t.Errorf("want the JSON syntax error wrapped, got %v", err)
			}
		}},
		{"live revision is not JSON", "live_json", "<<<", func(t *testing.T, err error) {
			var syntax *json.SyntaxError
			if !errors.As(err, &syntax) {
				t.Errorf("want the JSON syntax error wrapped, got %v", err)
			}
			// Only the live revision is damaged, so the error must not blame the draft.
			if strings.Contains(err.Error(), "draft") {
				t.Errorf("the error blames the draft, but only the live revision is damaged: %v", err)
			}
		}},
		{"draft has an unsupported schema", "draft_json", `{"schema":99}`, func(t *testing.T, err error) {
			if !errors.Is(err, ErrUnsupportedSchema) {
				t.Errorf("want ErrUnsupportedSchema, got %v", err)
			}
		}},
		{"draft is empty", "draft_json", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, id := setup(t, tc.column, tc.value)
			_, wantErr := s.GetFlow(ctx, id)
			if wantErr == nil {
				t.Fatal("GetFlow must fail on the damaged row, or the test proves nothing")
			}
			rec, err := s.GetFlowByMission(ctx, "mission_50")
			if err == nil || rec != nil {
				t.Fatalf("GetFlowByMission = %+v, %v, want nil and an error", rec, err)
			}
			if errors.Is(err, ErrNotFound) || errors.Is(err, ErrMissionAmbiguous) || errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("a damaged row must not look like a missing or ambiguous mission: %v", err)
			}
			if err.Error() != wantErr.Error() {
				t.Fatalf("GetFlowByMission = %q, GetFlow = %q, want the same error", err, wantErr)
			}
			if tc.check != nil {
				tc.check(t, err)
			}
		})
	}
}

// Ambiguity is decided by the rows, not by their content: a damaged document
// in either of two rows still gives ErrMissionAmbiguous.
func TestGetFlowByMissionAmbiguityBeatsDamage(t *testing.T) {
	ctx := context.Background()
	for _, damaged := range []string{"flow_aaaaaaaaeq", "flow_aaaaaaaaer"} {
		t.Run(damaged, func(t *testing.T) {
			s := openTestStore(t)
			mustCreateFlow(t, s, "flow_aaaaaaaaeq", "mission_51")
			mustCreateFlow(t, s, "flow_aaaaaaaaer", "mission_51")
			if _, err := s.db.ExecContext(ctx, `UPDATE flows SET draft_json = '{not json' WHERE id = ?`, damaged); err != nil {
				t.Fatal(err)
			}
			rec, err := s.GetFlowByMission(ctx, "mission_51")
			if !errors.Is(err, ErrMissionAmbiguous) || rec != nil {
				t.Fatalf("GetFlowByMission = %+v, %v, want nil and ErrMissionAmbiguous", rec, err)
			}
		})
	}
}

// The mission id is a bound parameter and matches exactly: no SQL, no LIKE
// wildcards, no case folding, no prefixes, no trimming.
func TestGetFlowByMissionMatchesExactly(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	mustCreateFlow(t, s, "flow_aaaaaaaaes", "mission_7")
	mustCreateFlow(t, s, "flow_aaaaaaaaet", "Mission_7")
	mustCreateFlow(t, s, "flow_aaaaaaaaeu", "")

	for _, id := range []string{
		"mission_",
		"mission_7 ",
		" mission_7",
		"mission_7\x00",
		"mission_%",
		"mission__",
		"%",
		"_",
		"MISSION_7",
		"mission_7' OR '1'='1",
		"mission_7' --",
		"' OR mission_id = '' --",
		"x'; DELETE FROM flows; --",
		strings.Repeat("m", 100000),
	} {
		if rec, err := s.GetFlowByMission(ctx, id); !errors.Is(err, ErrNotFound) || rec != nil {
			t.Errorf("GetFlowByMission(%.40q) = %+v, %v, want nil and ErrNotFound", id, rec, err)
		}
	}
	rec, err := s.GetFlowByMission(ctx, "mission_7")
	if err != nil || rec.ID != "flow_aaaaaaaaes" {
		t.Fatalf("GetFlowByMission(mission_7) = %+v, %v", rec, err)
	}
	if rec, err = s.GetFlowByMission(ctx, "Mission_7"); err != nil || rec.ID != "flow_aaaaaaaaet" {
		t.Fatalf("GetFlowByMission(Mission_7) = %+v, %v", rec, err)
	}
	if list, err := s.ListFlows(ctx, ""); err != nil || len(list) != 3 {
		t.Fatalf("the injection attempts changed the store: %d flows, %v", len(list), err)
	}
}

// Every path out of the lookup must release its connection. The pool is small,
// so a leak shows up as a deadline error from the 5th call on instead of a hang.
func TestGetFlowByMissionReleasesItsConnection(t *testing.T) {
	s := openTestStore(t)
	mustCreateFlow(t, s, "flow_aaaaaaaaev", "mission_60")
	mustCreateFlow(t, s, "flow_aaaaaaaaew", "mission_61")
	mustCreateFlow(t, s, "flow_aaaaaaaaex", "mission_61")
	mustCreateFlow(t, s, "flow_aaaaaaaaey", "mission_62")
	if _, err := s.db.Exec(`UPDATE flows SET draft_json = '{not json' WHERE id = ?`, "flow_aaaaaaaaey"); err != nil {
		t.Fatal(err)
	}

	const calls = 20
	if limit := s.db.Stats().MaxOpenConnections; limit <= 0 || limit >= calls {
		t.Fatalf("the pool allows %d connections: %d calls could not exhaust it", limit, calls)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	paths := []struct {
		name, mission string
		check         func(err error) bool
	}{
		{"found", "mission_60", func(err error) bool { return err == nil }},
		{"ambiguous", "mission_61", func(err error) bool { return errors.Is(err, ErrMissionAmbiguous) }},
		{"damaged", "mission_62", func(err error) bool {
			return err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrMissionAmbiguous) &&
				!errors.Is(err, context.DeadlineExceeded)
		}},
		{"not found", "mission_63", func(err error) bool { return errors.Is(err, ErrNotFound) }},
	}
	for _, p := range paths {
		for i := 0; i < calls; i++ {
			_, err := s.GetFlowByMission(ctx, p.mission)
			if !p.check(err) {
				t.Fatalf("%s path, call %d: unexpected result %v (a connection leak would show as a deadline error)", p.name, i+1, err)
			}
		}
		if inUse := s.db.Stats().InUse; inUse != 0 {
			t.Fatalf("%s path left %d connections in use after %d calls", p.name, inUse, calls)
		}
	}
}

// The lookup must use the mission index. This runs EXPLAIN QUERY PLAN on the
// real statement (the one with every column and LIMIT 2), not on a stand-in.
func TestGetFlowByMissionQueryUsesTheMissionIndex(t *testing.T) {
	s := openTestStore(t)
	rows, err := s.db.Query(`EXPLAIN QUERY PLAN `+flowByMissionSQL, "mission_1")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var details []string
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatal(err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	plan := strings.Join(details, "; ")
	if !strings.Contains(plan, "idx_flows_mission") {
		t.Fatalf("the mission lookup does not use idx_flows_mission: %s", plan)
	}
	if strings.Contains(plan, "SCAN") {
		t.Fatalf("the mission lookup scans the table: %s", plan)
	}
}
