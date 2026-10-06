package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"aurago/internal/invasion"
)

func TestEggVaultExportKeysCopyTheNestSecretOnlyForOptedInNests(t *testing.T) {
	egg := invasion.EggRecord{APIKeyRef: "egg_api_1"}
	cases := []struct {
		name string
		nest invasion.NestRecord
		want []string
	}{
		{"opted out", invasion.NestRecord{VaultSecretID: "nest_1"}, []string{"egg_api_1"}},
		{"opted in", invasion.NestRecord{VaultSecretID: "nest_1", ExportNestSecret: true}, []string{"egg_api_1", "nest_1"}},
		{"opted in without a secret", invasion.NestRecord{ExportNestSecret: true}, []string{"egg_api_1"}},
	}
	for _, tc := range cases {
		if got := eggVaultExportKeys(egg, tc.nest); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: keys = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func createNestForExport(t *testing.T, s *Server, extra map[string]any) string {
	t.Helper()
	body := map[string]any{"name": "nest", "access_type": "ssh", "host": "10.0.0.5", "username": "deploy", "secret": "pw", "deploy_method": "ssh", "active": true}
	for k, v := range extra {
		body[k] = v
	}
	rec := invasionTLSRequest(t, handleInvasionNests(s), http.MethodPost, "/api/invasion/nests", body)
	var created struct {
		ID string `json:"id"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &created) != nil {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	return created.ID
}

func TestCreateNestStartsWithoutTheNestSecretExport(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	for name, extra := range map[string]map[string]any{"older client": nil, "explicit off": {"export_nest_secret": false}} {
		if n, _ := invasion.GetNest(s.InvasionDB, createNestForExport(t, s, extra)); n.ExportNestSecret {
			t.Fatalf("%s: a new nest exports its secret", name)
		}
	}
	if n, _ := invasion.GetNest(s.InvasionDB, createNestForExport(t, s, map[string]any{"export_nest_secret": true})); !n.ExportNestSecret {
		t.Fatal("opting in on create was ignored")
	}
}

func TestUpdateNestWithoutTheExportFieldKeepsIt(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	id, err := invasion.CreateNest(s.InvasionDB, invasion.NestRecord{Name: "grandfathered", AccessType: "ssh", Host: "10.0.0.5", Port: 22, Active: true, DeployMethod: "ssh", VaultSecretID: "nest_x", ExportNestSecret: true})
	if err != nil {
		t.Fatal(err)
	}
	base := map[string]any{"name": "grandfathered", "access_type": "ssh", "host": "10.0.0.5", "port": 22, "active": true, "deploy_method": "ssh", "target_arch": "linux/amd64", "route": "direct"}
	put := func(extra map[string]any) bool {
		body := map[string]any{}
		for k, v := range base {
			body[k] = v
		}
		for k, v := range extra {
			body[k] = v
		}
		if rec := invasionTLSRequest(t, handleInvasionNest(s), http.MethodPut, "/api/invasion/nests/"+id, body); rec.Code != http.StatusOK {
			t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
		}
		n, _ := invasion.GetNest(s.InvasionDB, id)
		return n.ExportNestSecret
	}
	if !put(nil) {
		t.Fatal("an update from an older client switched the export off")
	}
	if put(map[string]any{"export_nest_secret": false}) {
		t.Fatal("explicit false was ignored")
	}
	if !put(map[string]any{"export_nest_secret": true}) {
		t.Fatal("explicit true was ignored")
	}
}

func TestNestReadsReturnTheExportSetting(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	id := createNestForExport(t, s, map[string]any{"export_nest_secret": true})
	rec := httptest.NewRecorder()
	handleInvasionNest(s)(rec, httptest.NewRequest(http.MethodGet, "/api/invasion/nests/"+id, nil))
	var one map[string]any
	if json.Unmarshal(rec.Body.Bytes(), &one) != nil || one["export_nest_secret"] != true {
		t.Fatalf("GET = %s, want export_nest_secret true", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleInvasionNests(s)(rec, httptest.NewRequest(http.MethodGet, "/api/invasion/nests", nil))
	var list []map[string]any
	if json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list) != 1 || list[0]["export_nest_secret"] != true {
		t.Fatalf("list = %s, want export_nest_secret true (the UI edits from the list)", rec.Body.String())
	}
}
