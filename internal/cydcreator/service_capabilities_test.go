package cydcreator

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"
)

func TestCapabilitiesMatchTheOverviewShape(t *testing.T) {
	env := newTestService(t, testServiceOptions{limits: ServiceLimits{BuildTimeout: 1, ColdBuildTimeout: 1, KeepBuildsPerProject: 1,
		OTA: OTALimits{Enabled: true, HeartbeatSeconds: 60, PublicURL: "http://aurago.local:8088"}}})
	caps := env.svc.Capabilities(context.Background())
	raw, err := json.Marshal(caps)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(top))
	for k := range top {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"active_job", "boards", "build_queue", "enabled", "fonts", "languages", "libraries", "limits", "ota",
		"readonly", "symbols", "template_version", "themes", "toolchain", "widgets"}
	if !slices.Equal(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	if string(top["active_job"]) != "null" || string(top["build_queue"]) != "0" || caps.TemplateVersion != TemplateVersion {
		t.Fatalf("active_job %s, build_queue %s, template %q", top["active_job"], top["build_queue"], caps.TemplateVersion)
	}
	if string(top["limits"]) != `{"max_screens":16,"max_widgets_per_screen":200,"max_app_files":64,"max_app_file_bytes":262144,"asset_budget_bytes":262144}` {
		t.Fatalf("limits = %s", top["limits"])
	}
	if string(top["ota"]) != `{"enabled":true,"public_url":"http://aurago.local:8088","insecure":true}` {
		t.Fatalf("ota = %s", top["ota"])
	}
	if string(top["toolchain"]) != `{"docker_available":true,"image":"test/cyd-builder:1","present":true,"pulling":false,"progress":0}` {
		t.Fatalf("toolchain = %s", top["toolchain"])
	}
	if string(top["fonts"]) != `{"families":["montserrat","inter","jetbrains_mono"],"sizes":[12,14,16,20,24,28,32,40,48]}` {
		t.Fatalf("fonts = %s", top["fonts"])
	}
	if !strings.Contains(string(top["libraries"]), `"depends":[`) {
		t.Fatalf("libraries lack the pinned depends lists: %s", top["libraries"])
	}
	if len(caps.Boards) == 0 || len(caps.Themes) != 7 || len(caps.Widgets) == 0 || len(caps.Libraries) == 0 ||
		len(caps.Languages) != 16 || len(caps.Symbols) == 0 {
		t.Fatalf("catalogs incomplete: %d boards, %d themes, %d widgets", len(caps.Boards), len(caps.Themes), len(caps.Widgets))
	}

	p := env.createProject(t, "Job")
	if _, err := env.svc.db.Exec(`INSERT INTO cc_jobs(id,project_id,mode,status,phase,prompt,created_at)
		VALUES('ccj_1',?,'change','running','coding','x','2026-10-10T08:00:00.000Z')`, p.ID); err != nil {
		t.Fatal(err)
	}
	caps = env.svc.Capabilities(context.Background())
	if caps.ActiveJob == nil || *caps.ActiveJob != (ActiveJob{JobID: "ccj_1", ProjectID: p.ID, Phase: "coding", Status: "running"}) {
		t.Fatalf("active_job = %+v", caps.ActiveJob)
	}
}
