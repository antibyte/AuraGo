package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

// go2RTCDetectionConfigFixture has go2rtc off, a global detection block with a
// schedule, a fully configured camera with explicit zero cooldown and desktop off,
// an unconfigured camera and a camera whose detection block sets only some keys.
const go2RTCDetectionConfigFixture = "go2rtc:\n" +
	"  enabled: false\n" +
	"  detection:\n" +
	"    enabled: true\n" +
	"    model: accurate\n" +
	"    backend: cpu\n" +
	"    retention_days: 7\n" +
	"    max_storage_mb: 500\n" +
	"    schedule:\n" +
	"      - days: [mon, fri]\n" +
	"        from: \"22:00\"\n" +
	"        to: \"06:00\"\n" +
	"  streams:\n" +
	"    - id: front-door\n" +
	"      name: Front door\n" +
	"      enabled: true\n" +
	"      detection:\n" +
	"        enabled: true\n" +
	"        interval_seconds: 3\n" +
	"        min_score: 0.6\n" +
	"        cooldown_seconds: 0\n" +
	"        zones:\n" +
	"          - name: Driveway\n" +
	"            points: [[0.1, 0.5], [0.9, 0.5], [0.9, 1.0]]\n" +
	"          - name: Gate\n" +
	"            points: [[0, 0], [0.25, 0], [0.25, 0.25], [0, 0.25]]\n" +
	"        actions:\n" +
	"          message: true\n" +
	"          channel: telegram\n" +
	"          agent: true\n" +
	"          agent_instruction: Check the driveway\n" +
	"          desktop: false\n" +
	"    - id: garage\n" +
	"      name: Garage\n" +
	"      enabled: false\n" +
	"    - id: porch\n" +
	"      name: Porch\n" +
	"      enabled: true\n" +
	"      detection:\n" +
	"        enabled: true\n" +
	"        min_score: 0.7\n"

// newGo2RTCDetectionConfigServer writes the fixture and returns a server for the
// real GET/PUT /api/config handlers plus the loaded go2rtc settings before any save.
func newGo2RTCDetectionConfigServer(t *testing.T) (*Server, string, config.Go2RTCConfig) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(path, []byte(go2RTCDetectionConfigFixture), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.ConfigPath = path
	vault, err := security.NewVault(strings.Repeat("77", 32), filepath.Join(root, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	// The handlers replace the live config; keep a detached copy of the original.
	return &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}, path, cfg.Clone().Go2RTC
}

// getGo2RTCConfigSection returns the go2rtc section of GET /api/config as the browser
// receives it.
func getGo2RTCConfigSection(t *testing.T, s *Server) map[string]interface{} {
	t.Helper()
	rec := httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/config: %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode GET /api/config: %v", err)
	}
	section, ok := body["go2rtc"].(map[string]interface{})
	if !ok {
		t.Fatalf("GET /api/config go2rtc = %#v", body["go2rtc"])
	}
	return section
}

func putGo2RTCConfigSection(t *testing.T, s *Server, section interface{}) *httptest.ResponseRecorder {
	t.Helper()
	patch, err := json.Marshal(map[string]interface{}{"go2rtc": section})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(string(patch))))
	return rec
}

func TestInjectGo2RTCConfigProjectsDetection(t *testing.T) {
	cfg := &config.Config{}
	cfg.Go2RTC.Detection = config.Go2RTCDetectionConfig{Enabled: true, Model: "accurate", Backend: "cpu", RetentionDays: 7, MaxStorageMB: 500}
	front := config.DefaultGo2RTCStreamDetection(true)
	front.CooldownSeconds = 0
	front.Actions.Desktop = false
	front.Zones = []config.Go2RTCDetectionZone{{Name: "Driveway", Points: [][2]float64{{0.1, 0.5}, {0.9, 0.5}, {0.9, 1}}}}
	cfg.Go2RTC.Streams = []config.Go2RTCStreamConfig{
		{ID: "front-door", Name: "Front door", Enabled: true, Detection: front},
		{ID: "garage", Name: "Garage"},
	}
	raw := map[string]interface{}{}
	injectGo2RTCConfig(raw, cfg, nil)
	section := raw["go2rtc"].(map[string]interface{})
	global, ok := section["detection"].(map[string]interface{})
	if !ok || global["model"] != "accurate" || global["retention_days"] != float64(7) || global["schedule"] == nil {
		t.Fatalf("global detection projection = %#v", section["detection"])
	}
	streams := section["streams"].([]interface{})
	frontDetection := streams[0].(map[string]interface{})["detection"].(map[string]interface{})
	zones := frontDetection["zones"].([]interface{})
	if len(zones) != 1 || zones[0].(map[string]interface{})["name"] != "Driveway" {
		t.Fatalf("front-door zones = %#v", frontDetection["zones"])
	}
	wantPoints := []interface{}{[]interface{}{0.1, 0.5}, []interface{}{0.9, 0.5}, []interface{}{0.9, float64(1)}}
	if points := zones[0].(map[string]interface{})["points"]; !reflect.DeepEqual(points, wantPoints) {
		t.Fatalf("front-door points = %#v, want [[x, y], …]", points)
	}
	actions, _ := frontDetection["actions"].(map[string]interface{})
	if frontDetection["cooldown_seconds"] != float64(0) || actions["desktop"] != false {
		t.Fatalf("explicit cooldown 0 / desktop false not projected: %#v", frontDetection)
	}
	garage := streams[1].(map[string]interface{})["detection"].(map[string]interface{})
	if garage["interval_seconds"] != float64(2) || garage["enabled"] != false || garage["min_score"] != 0.5 {
		t.Fatalf("an unconfigured stream must project the default set: %#v", garage)
	}
}

// A config page save that sends the go2rtc section back unchanged goes GET
// projection → JSON → PUT merge → file → config.Load and must keep every detection
// setting, including zones, an explicit cooldown of 0 and desktop alerts off.
func TestConfigUISaveKeepsGo2RTCDetection(t *testing.T) {
	s, path, want := newGo2RTCDetectionConfigServer(t)
	if want.Streams[0].Detection.CooldownSeconds != 0 || want.Streams[0].Detection.Actions.Desktop || len(want.Streams[0].Detection.Zones) != 2 {
		t.Fatalf("fixture did not load the explicit values: %+v", want.Streams[0].Detection)
	}

	rec := putGo2RTCConfigSection(t, s, getGo2RTCConfigSection(t, s))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/config: %d %s", rec.Code, rec.Body.String())
	}
	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	for name, got := range map[string]config.Go2RTCConfig{"file": reloaded.Go2RTC, "live": s.ConfigSnapshot().Go2RTC} {
		if !reflect.DeepEqual(got.Detection, want.Detection) {
			t.Fatalf("%s global detection after config UI save = %+v, want %+v", name, got.Detection, want.Detection)
		}
		if len(got.Streams) != len(want.Streams) {
			t.Fatalf("%s streams after config UI save = %d, want %d", name, len(got.Streams), len(want.Streams))
		}
		for i := range want.Streams {
			if !reflect.DeepEqual(got.Streams[i].Detection, want.Streams[i].Detection) {
				t.Fatalf("%s stream %s detection after config UI save = %+v, want %+v", name, want.Streams[i].ID, got.Streams[i].Detection, want.Streams[i].Detection)
			}
		}
	}
	if garage := reloaded.Go2RTC.Streams[1].Detection; !reflect.DeepEqual(garage, config.DefaultGo2RTCStreamDetection(false)) {
		t.Fatalf("unconfigured camera after save = %+v, want the default set", garage)
	}

	// The file is stable: a second unchanged save writes the same bytes.
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if rec := putGo2RTCConfigSection(t, s, getGo2RTCConfigSection(t, s)); rec.Code != http.StatusOK {
		t.Fatalf("second PUT /api/config: %d %s", rec.Code, rec.Body.String())
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("an unchanged second save rewrote the file:\n--- first\n%s\n--- second\n%s", first, second)
	}
}

// A save from a config page that does not know detection sends stream objects
// without it. deepMerge replaces go2rtc.streams wholesale, so those cameras fall
// back to the default set (detection off); the global block is a map and is merged,
// so it is kept. This pins the merge semantics the camera app must respect.
func TestConfigUISaveOfStreamsWithoutDetectionResetsThemToDefaults(t *testing.T) {
	s, path, want := newGo2RTCDetectionConfigServer(t)
	legacy := map[string]interface{}{"streams": []interface{}{
		map[string]interface{}{"id": "front-door", "name": "Front door", "enabled": true, "source": "", "source_configured": false},
		map[string]interface{}{"id": "garage", "name": "Garage", "enabled": false, "source": "", "source_configured": false},
		map[string]interface{}{"id": "porch", "name": "Porch", "enabled": true, "source": "", "source_configured": false},
	}}
	if rec := putGo2RTCConfigSection(t, s, legacy); rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/config: %d %s", rec.Code, rec.Body.String())
	}
	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(reloaded.Go2RTC.Detection, want.Detection) {
		t.Fatalf("global detection = %+v, want it kept: %+v", reloaded.Go2RTC.Detection, want.Detection)
	}
	if len(reloaded.Go2RTC.Streams) != 3 {
		t.Fatalf("streams = %+v", reloaded.Go2RTC.Streams)
	}
	for _, stream := range reloaded.Go2RTC.Streams {
		if !reflect.DeepEqual(stream.Detection, config.DefaultGo2RTCStreamDetection(false)) {
			t.Fatalf("stream %s detection = %+v, want the default set (the payload had none)", stream.ID, stream.Detection)
		}
	}
}

// Save-time validation runs while go2rtc is off and answers 400 with the C5 field
// path; the file config.Load would reject is never written.
func TestConfigUISaveRejectsInvalidGo2RTCDetectionWithFieldPath(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(section map[string]interface{})
		field  string
	}{
		{
			name: "point with one value",
			mutate: func(section map[string]interface{}) {
				go2RTCSectionZone(section, 0, 0)["points"] = []interface{}{[]interface{}{0.1, 0.5}, []interface{}{0.9}, []interface{}{0.9, 1.0}}
			},
			field: "go2rtc.streams[front-door].detection.zones[0].points[1]",
		},
		{
			name: "point with three values",
			mutate: func(section map[string]interface{}) {
				go2RTCSectionZone(section, 0, 1)["points"] = []interface{}{[]interface{}{0.0, 0.0}, []interface{}{0.25, 0.0}, []interface{}{0.25, 0.25, 0.5}}
			},
			field: "go2rtc.streams[front-door].detection.zones[1].points[2]",
		},
		{
			name: "min score below range",
			mutate: func(section map[string]interface{}) {
				go2RTCSectionStreamDetection(section, 2)["min_score"] = 0.1
			},
			field: "go2rtc.streams[porch].detection.min_score",
		},
		{
			name: "duplicate zone name",
			mutate: func(section map[string]interface{}) {
				go2RTCSectionZone(section, 0, 1)["name"] = "DRIVEWAY"
			},
			field: "go2rtc.streams[front-door].detection.zones[1].name",
		},
		{
			name: "global retention",
			mutate: func(section map[string]interface{}) {
				section["detection"].(map[string]interface{})["retention_days"] = 999
			},
			field: "go2rtc.detection.retention_days",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, path, _ := newGo2RTCDetectionConfigServer(t)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			section := getGo2RTCConfigSection(t, s)
			tc.mutate(section)
			rec := putGo2RTCConfigSection(t, s, section)
			var body struct {
				Status  string `json:"status"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response %q: %v", rec.Body.String(), err)
			}
			message, ok := strings.CutPrefix(body.Message, tc.field+": ")
			if rec.Code != http.StatusBadRequest || body.Status != "error" || !ok || strings.TrimSpace(message) == "" {
				t.Fatalf("PUT /api/config = %d %+v, want 400 with message %q", rec.Code, body, tc.field+": <reason>")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatalf("a rejected save changed the file:\n%s", after)
			}
			if _, err := config.Load(path); err != nil {
				t.Fatalf("file after rejected save no longer loads: %v", err)
			}
		})
	}
}

func go2RTCSectionStreamDetection(section map[string]interface{}, stream int) map[string]interface{} {
	return section["streams"].([]interface{})[stream].(map[string]interface{})["detection"].(map[string]interface{})
}

func go2RTCSectionZone(section map[string]interface{}, stream, zone int) map[string]interface{} {
	return go2RTCSectionStreamDetection(section, stream)["zones"].([]interface{})[zone].(map[string]interface{})
}

func TestValidateGo2RTCSettingsChecksDetectionEvenWhenDisabled(t *testing.T) {
	cfg := config.Go2RTCConfig{Streams: []config.Go2RTCStreamConfig{{ID: "cam", Detection: config.Go2RTCStreamDetectionConfig{
		IntervalSeconds: 2, MinScore: 0.1, Actions: config.Go2RTCDetectionActions{Channel: "all"},
		Zones: []config.Go2RTCDetectionZone{{Name: "  Yard  ", Points: [][2]float64{{0, 0}, {1, 0}, {1, 1}}}},
	}}}}
	err := validateGo2RTCSettings(cfg, config.Runtime{}, nil)
	var fieldErr *config.Go2RTCDetectionFieldError
	if !errors.As(err, &fieldErr) || fieldErr.Field != "go2rtc.streams[cam].detection.min_score" {
		t.Fatalf("err = %v, want the min_score field error", err)
	}
	if cfg.Streams[0].Detection.MinScore != 0.1 || cfg.Streams[0].Detection.Zones[0].Name != "  Yard  " {
		t.Fatal("validation mutated the caller's config")
	}
	cfg.Streams[0].Detection = config.Go2RTCStreamDetectionConfig{}
	if err := validateGo2RTCSettings(cfg, config.Runtime{}, nil); err != nil {
		t.Fatalf("a stream without detection (written by older AuraGo) must stay valid: %v", err)
	}
	cfg.Detection.RetentionDays = 999
	if err := validateGo2RTCSettings(cfg, config.Runtime{}, nil); !errors.As(err, &fieldErr) || fieldErr.Field != "go2rtc.detection.retention_days" {
		t.Fatalf("err = %v, want the retention_days field error", err)
	}
}

// Detection settings are not container settings: a detection-only edit takes the
// soft path (Configure + reconcile) and never recreates the go2rtc container.
func TestGo2RTCRuntimeTransitionDetectionOnlyChangeKeepsContainer(t *testing.T) {
	oldCfg := config.Config{}
	oldCfg.Go2RTC = config.Go2RTCConfig{
		Enabled: true, Image: config.Go2RTCDefaultImage, ContainerName: "aurago_go2rtc", URL: "http://127.0.0.1:1984", APIHostPort: 1984,
		Detection: config.NormalizeGo2RTCDetectionSettings(config.Go2RTCDetectionConfig{Enabled: true}),
		Streams: []config.Go2RTCStreamConfig{{
			ID: "front-door", Name: "Front door", Enabled: true, Source: "rtsp://camera.local/live",
			Detection: config.DefaultGo2RTCStreamDetection(true),
		}},
	}
	newCfg := *oldCfg.Clone()
	newCfg.Go2RTC.Detection.RetentionDays = 30
	newCfg.Go2RTC.Detection.Schedule = []config.Go2RTCDetectionWindow{{Days: []string{"mon"}, From: "22:00", To: "06:00"}}
	newCfg.Go2RTC.Streams[0].Detection.MinScore = 0.7
	newCfg.Go2RTC.Streams[0].Detection.Zones = []config.Go2RTCDetectionZone{{Name: "Driveway", Points: [][2]float64{{0, 0}, {1, 0}, {1, 1}}}}
	newCfg.Go2RTC.Streams[0].Detection.Enabled = false

	changed, recreate := go2RTCRuntimeTransition(oldCfg, newCfg)
	if !changed {
		t.Fatal("a detection-only change must count as a go2rtc change")
	}
	if recreate {
		t.Fatal("a detection-only change must not recreate the go2rtc container")
	}
}

func TestDecodeGo2RTCStreamDetectionIsStrict(t *testing.T) {
	good := `{"enabled":true,"interval_seconds":3,"min_score":0.6,"cooldown_seconds":60,"zones":[{"name":"A","points":[[0,0],[1,0],[1,1]]}],"actions":{"message":false,"channel":"all","agent":false,"agent_instruction":"","desktop":true}}`
	decoded, err := decodeGo2RTCStreamDetection(json.RawMessage(good))
	if err != nil || decoded.IntervalSeconds != 3 || decoded.Zones[0].Points[2] != [2]float64{1, 1} {
		t.Fatalf("decode = %+v, %v", decoded, err)
	}
	assertGo2RTCDetectionDecodeErrors(t, []go2RTCDetectionDecodeCase{
		{`{"zones":[{"name":"A","points":[[0,0],[1,0],[1,1,1]]}]}`, "detection.zones[0].points[2]", "must be [x, y]"},
		{`{"zones":[{"name":"A","points":[[0,0],[1],[1,1]]}]}`, "detection.zones[0].points[1]", "must be [x, y]"},
		{`{"zones":[{"name":"A","points":[[0,0],[1,"0"],[1,1]]}]}`, "detection.zones[0].points[1]", "must be [x, y]"},
		{`{"zones":[{"name":"A","points":[[0,0],[1,1e400],[1,1]]}]}`, "detection.zones[0].points[1]", "must be [x, y]"},
		{`{"zones":[{"name":"A","points":"0,0 1,0 1,1"}]}`, "detection.zones[0].points", ""},
		{`{"zones":[{"name":5,"points":[[0,0],[1,0],[1,1]]}]}`, "detection.zones[0].name", "must be text"},
		{`{"zones":[{"name":"A","points":[[0,0],[1,0],[1,1]],"color":"red"}]}`, "detection.zones[0].color", ""},
		{`{"zones":[5]}`, "detection.zones[0]", ""},
		{`{"zones":{"name":"A"}}`, "detection.zones", ""},
		{`{"interval_seconds":2,"unknown":true}`, "detection.unknown", ""},
		{`{"Enabled":true}`, "detection.Enabled", ""},
		{`{"":1}`, "detection", `"" is not a known setting`},
		{`{"actions":{"":true}}`, "detection.actions", `"" is not a known setting`},
		{`{"actions":{"desktop":true,"sms":true}}`, "detection.actions.sms", ""},
		{`{"actions":"all"}`, "detection.actions", ""},
		{`{"interval_seconds":"2"}`, "detection.interval_seconds", ""},
		{`{"interval_seconds":2.5}`, "detection.interval_seconds", ""},
		{`{"actions":{"desktop":"yes"}}`, "detection.actions.desktop", ""},
		{`{"enabled":true} {"enabled":false}`, "detection", ""},
		{`{"enabled":true} }`, "detection", ""},
		{`{"enabled":`, "detection", ""},
		{``, "detection", ""},
		{`5`, "detection", ""},
	})
}

// JSON null never stands for "default" or 0: a NaN from a zone editor becomes null
// through JSON.stringify and must not be saved as a coordinate of 0.
func TestDecodeGo2RTCStreamDetectionRejectsNull(t *testing.T) {
	const points = `[[0,0],[1,0],[1,1]]`
	assertGo2RTCDetectionDecodeErrors(t, []go2RTCDetectionDecodeCase{
		{`null`, "detection", "must not be null"},
		{`{"enabled":null}`, "detection.enabled", "must not be null"},
		{`{"interval_seconds":null}`, "detection.interval_seconds", "must not be null"},
		{`{"min_score":null}`, "detection.min_score", "must not be null"},
		{`{"cooldown_seconds":null}`, "detection.cooldown_seconds", "must not be null"},
		{`{"zones":null}`, "detection.zones", "must not be null"},
		{`{"actions":null}`, "detection.actions", "must not be null"},
		{`{"actions":{"message":null}}`, "detection.actions.message", "must not be null"},
		{`{"actions":{"channel":null}}`, "detection.actions.channel", "must not be null"},
		{`{"actions":{"agent":null}}`, "detection.actions.agent", "must not be null"},
		{`{"actions":{"agent_instruction":null}}`, "detection.actions.agent_instruction", "must not be null"},
		{`{"actions":{"desktop":null}}`, "detection.actions.desktop", "must not be null"},
		{`{"zones":[null]}`, "detection.zones[0]", "must not be null"},
		{`{"zones":[{"name":null,"points":` + points + `}]}`, "detection.zones[0].name", "must not be null"},
		{`{"zones":[{"name":"A","points":null}]}`, "detection.zones[0].points", "must not be null"},
		{`{"zones":[{"name":"A","points":[[0,0],[0.5,null],[1,1]]}]}`, "detection.zones[0].points[1]", "must be [x, y]"},
		{`{"zones":[{"name":"A","points":[[0,0],null,[1,1]]}]}`, "detection.zones[0].points[1]", "must be [x, y]"},
	})
}

// The typed decode after the pre-pass reports its errors with field paths too.
func TestGo2RTCDetectionJSONErrorKeepsFieldPaths(t *testing.T) {
	decode := func(raw string) error {
		var out config.Go2RTCStreamDetectionConfig
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.DisallowUnknownFields()
		return decoder.Decode(&out)
	}
	for raw, want := range map[string]string{
		`{"color":"red"}`:                        "detection.color",
		`{"":1}`:                                 "detection",
		`{"actions":{"desktop":"yes"}}`:          "detection.actions.desktop",
		`{"zones":[{"name":"A"},{"name":5}]}`:    "detection.zones[1].name",
		`{"zones":[{"points":[[0,0],[0,"a"]]}]}`: "detection.zones[0].points[1]",
	} {
		err := go2RTCDetectionJSONError(decode(raw))
		var fieldErr *config.Go2RTCDetectionFieldError
		if !errors.As(err, &fieldErr) || fieldErr.Field != want || fieldErr.Message == "" || fieldErr.Message == "must be a detection object" {
			t.Fatalf("go2RTCDetectionJSONError(%s) = %v, want field %q with a specific message", raw, err, want)
		}
	}
}

type go2RTCDetectionDecodeCase struct {
	raw, field, message string // message "" accepts any non-empty message
}

func assertGo2RTCDetectionDecodeErrors(t *testing.T, cases []go2RTCDetectionDecodeCase) {
	t.Helper()
	for _, tc := range cases {
		decoded, err := decodeGo2RTCStreamDetection(json.RawMessage(tc.raw))
		var fieldErr *config.Go2RTCDetectionFieldError
		if !errors.As(err, &fieldErr) || fieldErr.Field != tc.field || fieldErr.Message == "" || (tc.message != "" && fieldErr.Message != tc.message) {
			t.Errorf("decode(%s) = %+v, %v; want field %q message %q", tc.raw, decoded, err, tc.field, tc.message)
		}
	}
}

// A request that replaces a camera's detection block follows the YAML rule (C5): a
// missing key gets its default, an explicit value is kept, also in nested actions.
func TestDecodeGo2RTCStreamDetectionDefaultsMissingKeys(t *testing.T) {
	decoded, err := decodeGo2RTCStreamDetection(json.RawMessage(`{"enabled":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, config.DefaultGo2RTCStreamDetection(true)) {
		t.Fatalf("decode({enabled:true}) = %+v, want the default set", decoded)
	}
	decoded, err = decodeGo2RTCStreamDetection(json.RawMessage(`{"cooldown_seconds":0,"actions":{"desktop":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := config.DefaultGo2RTCStreamDetection(false)
	want.CooldownSeconds = 0
	want.Actions.Desktop = false
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decode(explicit zeros) = %+v, want %+v", decoded, want)
	}
	if normalized := config.NormalizeGo2RTCStreamDetection(decoded); normalized.CooldownSeconds != 0 || normalized.Actions.Desktop {
		t.Fatalf("normalization dropped explicit zeros: %+v", normalized)
	}
}

// /api/setup and the local LLM setup save through applyConfigPatch. A detection block
// config.Load rejects would keep AuraGo from starting, so it is never written.
func TestApplyConfigPatchRejectsInvalidGo2RTCDetection(t *testing.T) {
	s, path, _ := newGo2RTCDetectionConfigServer(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = applyConfigPatch(s, map[string]interface{}{"go2rtc": map[string]interface{}{"detection": map[string]interface{}{"retention_days": 999}}})
	var fieldErr *config.Go2RTCDetectionFieldError
	if !errors.As(err, &fieldErr) || fieldErr.Field != "go2rtc.detection.retention_days" {
		t.Fatalf("applyConfigPatch err = %v, want the go2rtc.detection.retention_days field error", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("a rejected patch changed the file:\n%s", after)
	}
}

func TestSetupSaveRejectsInvalidGo2RTCDetectionWithBadRequest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	initial := []byte("server:\n  host: 127.0.0.1\nauth:\n  enabled: false\n")
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.ConfigPath = path
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	if !needsSetup(cfg) {
		t.Fatal("fixture must still need setup")
	}
	addSetupCSRFTokenForTest(s, "detection-setup-token")
	req := httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(`{"go2rtc":{"detection":{"retention_days":999}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "detection-setup-token")
	withSetupBootstrapToken(s, req)
	rec := httptest.NewRecorder()
	handleSetupSave(s).ServeHTTP(rec, req)

	var body struct {
		Error   string `json:"error"`
		Details string `json:"details"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusBadRequest || !strings.HasPrefix(body.Details, "go2rtc.detection.retention_days: ") {
		t.Fatalf("POST /api/setup = %d %s, want 400 with the go2rtc.detection.retention_days field error", rec.Code, rec.Body.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(initial) {
		t.Fatalf("a rejected setup save changed the file:\n%s", after)
	}
}

// The first pass of decodeGo2RTCStreamDetection (checkGo2RTCDetectionJSON) descends
// only into structs, slices and [2]float64 points and reads only direct fields. A
// detection type that grows another shape must extend that walker first, or null and
// unknown keys below it would slip through to encoding/json.
func TestGo2RTCDetectionWalkerCoversDetectionTypes(t *testing.T) {
	point := reflect.TypeOf([2]float64{})
	const extend = "extend the first-pass walker checkGo2RTCDetectionJSON in go2rtc_detection_config.go first"
	seen := map[reflect.Type]bool{}
	var visit func(typ reflect.Type, path string)
	visit = func(typ reflect.Type, path string) {
		if seen[typ] {
			return
		}
		seen[typ] = true
		switch typ.Kind() {
		case reflect.Pointer, reflect.Map, reflect.Interface:
			t.Errorf("%s has kind %s: %s", path, typ.Kind(), extend)
		case reflect.Array:
			if typ != point {
				t.Errorf("%s is %s, the only supported array is [2]float64: %s", path, typ, extend)
			}
		case reflect.Slice:
			visit(typ.Elem(), path+"[]")
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				if field.Anonymous {
					t.Errorf("%s embeds %s (encoding/json promotes its fields): %s", path, field.Type, extend)
				}
				if field.IsExported() {
					visit(field.Type, path+"."+field.Name)
				}
			}
		}
	}
	visit(reflect.TypeOf(config.Go2RTCStreamDetectionConfig{}), "Go2RTCStreamDetectionConfig")
}

// A duplicate object key reaches the walker only with its last value, while the typed
// decode merges every occurrence (null into a bool is a no-op there). JSON.stringify
// never writes duplicate keys, so this pins the behaviour instead of adding a token
// walker.
func TestDecodeGo2RTCStreamDetectionDuplicateKeysMergeLikeEncodingJSON(t *testing.T) {
	decoded, err := decodeGo2RTCStreamDetection(json.RawMessage(`{"actions":{"desktop":null},"actions":{}}`))
	if err != nil || !reflect.DeepEqual(decoded, config.DefaultGo2RTCStreamDetection(false)) {
		t.Fatalf("decode(duplicate actions with a null) = %+v, %v; want the default set", decoded, err)
	}
	decoded, err = decodeGo2RTCStreamDetection(json.RawMessage(`{"actions":{"desktop":false,"channel":"telegram"},"actions":{"message":true}}`))
	want := config.DefaultGo2RTCStreamDetection(false)
	want.Actions = config.Go2RTCDetectionActions{Message: true, Channel: "telegram", Desktop: false}
	if err != nil || !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decode(duplicate actions) = %+v, %v; want the union %+v", decoded, err, want)
	}
}
