package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGo2RTCSourceTypesHandlerServesCatalogWithoutValidationRules(t *testing.T) {
	rec := httptest.NewRecorder()
	handleGo2RTCSourceTypes()(rec, httptest.NewRequest(http.MethodGet, "/api/go2rtc/source-types", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=300" {
		t.Errorf("Cache-Control = %q", got)
	}
	var body struct {
		Types []struct {
			ID     string `json:"id"`
			Mode   string `json:"mode"`
			Fields []struct {
				Name string `json:"name"`
			} `json:"fields"`
		} `json:"types"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, kind := range body.Types {
		ids[kind.ID] = kind.Mode
	}
	for id, mode := range map[string]string{"rtsp": "form", "tapo": "form", "ring": "form", "homekit": "url", "tuya": "dialog"} {
		if ids[id] != mode {
			t.Errorf("type %q mode = %q, want %q", id, ids[id], mode)
		}
	}
	for _, forbidden := range []string{"exec", "ffmpeg", "xiaomi", "isapi", "requiredQuery", "shape"} {
		if strings.Contains(rec.Body.String(), `"`+forbidden) {
			t.Errorf("catalog contains %q", forbidden)
		}
	}
	rec = httptest.NewRecorder()
	handleGo2RTCSourceTypes()(rec, httptest.NewRequest(http.MethodPost, "/api/go2rtc/source-types", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d", rec.Code)
	}
}

func TestGo2RTCSourceChangesStripSourceType(t *testing.T) {
	stream := map[string]interface{}{"id": "front-door", "name": "Front", "source_type": "rtsp", "source": "tapo://pw@192.168.1.2"}
	changes, err := go2RTCSourceChanges([]interface{}{stream}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Value != "tapo://pw@192.168.1.2" {
		t.Fatalf("changes = %+v", changes)
	}
	if _, exists := stream["source_type"]; exists {
		t.Errorf("source_type must not reach config.yaml")
	}
}

func TestGo2RTCSourceChangesNameTheRuleNotTheSource(t *testing.T) {
	stream := map[string]interface{}{"id": "front-door", "source": "ring:?device_id=secret-device"}
	_, err := go2RTCSourceChanges([]interface{}{stream}, true)
	if err == nil || !strings.Contains(err.Error(), "ring source needs refresh_token") || strings.Contains(err.Error(), "secret-device") {
		t.Fatalf("error = %v", err)
	}
}
