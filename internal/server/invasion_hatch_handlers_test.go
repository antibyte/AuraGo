package server

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/invasion"
)

func TestEggVaultExportKeysUsesOnlyEggAndNestRefs(t *testing.T) {
	egg := invasion.EggRecord{APIKeyRef: " egg_api_key "}
	nest := invasion.NestRecord{VaultSecretID: " nest_secret "}

	got := eggVaultExportKeys(egg, nest)
	want := []string{"egg_api_key", "nest_secret"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("eggVaultExportKeys() = %#v, want %#v", got, want)
	}
}

func TestEggVaultExportKeysDeduplicatesAndSkipsEmptyKeys(t *testing.T) {
	egg := invasion.EggRecord{APIKeyRef: "shared_key"}
	nest := invasion.NestRecord{VaultSecretID: " shared_key "}

	got := eggVaultExportKeys(egg, nest)
	want := []string{"shared_key"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("eggVaultExportKeys() = %#v, want %#v", got, want)
	}
}

func TestInvasionReadonlyBlocksMutatingHatchHandlers(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		handler func(*Server) http.HandlerFunc
	}{
		{"hatch", "/api/invasion/nests/n1/hatch", handleInvasionNestHatch},
		{"stop", "/api/invasion/nests/n1/stop", handleInvasionNestStop},
		{"send-secret", "/api/invasion/nests/n1/send-secret", handleInvasionNestSendSecret},
		{"send-task", "/api/invasion/nests/n1/send-task", handleInvasionNestSendTask},
		{"rotate-key", "/api/invasion/nests/n1/rotate-key", handleInvasionNestRotateKey},
		{"rollback", "/api/invasion/nests/n1/rollback", handleInvasionNestRollback},
		{"safe-reconfigure", "/api/invasion/nests/n1/safe-reconfigure", handleInvasionNestSafeReconfigure},
		{"config-rollback", "/api/invasion/nests/n1/config-rollback", handleInvasionNestConfigRollback},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{Cfg: &config.Config{}}
			s.Cfg.InvasionControl.ReadOnly = true
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(`{}`))
			rec := httptest.NewRecorder()

			tt.handler(s).ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
		})
	}
}
