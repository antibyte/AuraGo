# go2rtc Source Types Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Network Cameras and the config page can add every go2rtc network/cloud camera type (RTSP, HTTP, ONVIF, RTMP, Tapo, Kasa, DVRIP, Bubble, EseeCloud, DoorBird, GoPro, Home Assistant, Ring, Nest, HomeKit, Roborock, Wyze, Ivideon, WebRTC, WebTorrent) through a type picker with guided forms; Tuya keeps its existing dialog.

**Architecture:** One Go catalog in `internal/config/go2rtc_source_types.go` validates every source URL per type and describes the forms; `GET /api/go2rtc/source-types` serves the form description; `ui/js/go2rtc-source-form.js` renders the picker/form in both UIs, builds the URL in the browser and hands it to the existing save paths (vault-only storage unchanged). `source_type` for every camera is derived from the stored source.

**Tech Stack:** Go 1.x (`net/url`), vanilla JS IIFE modules, Node `vm` regression tests (`scripts/test-ui-regressions.mjs`), Go UI tests under `ui/`.

**Spec:** `docs/superpowers/specs/2026-10-26-go2rtc-source-types-design.md` (read the "Revision after merging" section first — it overrides parts of the original design).

**Worktree:** `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\go2rtc-source-types`, branch `claude/go2rtc-source-types` (already contains the merged Codex go2rtc branch, merge commit `94451e7ee`). Run every command from this directory. Commit after every task. Other sessions work in the main checkout — never touch it.

**Conventions:** Go tests beside the source, table-driven. UI strings in all 16 locales (cs da de el en es fr hi it ja nl no pl pt sv zh). No `alert()/confirm()`. No external assets. If `go test` fails with disk-space errors, trim `GOCACHE` entries older than 3 days on D: (see memory `reference_go_cache_disk_d`).

---

## File map

| File | Change |
|---|---|
| `internal/config/go2rtc_source_types.go` | **new** — catalog, `ValidateGo2RTCSourceURL`, `Go2RTCSourceTypeOf`, `Go2RTCStreamSourceType`, `Go2RTCSourceTypes` |
| `internal/config/go2rtc_source_types_test.go` | **new** — table tests |
| `internal/tools/go2rtc.go` | `ValidateGo2RTCSource` delegates; `SourceType` via helper |
| `internal/tools/go2rtc_media.go` | `applyMediaStatus` uses helper |
| `internal/tools/go2rtc_test.go` | extend allowed/blocked table |
| `internal/server/go2rtc_source_types.go` | **new** — catalog handler |
| `internal/server/go2rtc_source_types_test.go` | **new** — handler + projection tests |
| `internal/server/go2rtc_handlers.go` | register route |
| `internal/server/go2rtc_app_handlers.go` | `source_type` for every camera; clearer 400 messages |
| `internal/server/go2rtc_sources.go` | strip `source_type`; wrap validator error |
| `internal/server/config_handlers_main.go` | config projection `source_type`; remove dead `validateGo2RTCSourceURL` |
| `ui/js/go2rtc-source-form.js` | **new** — shared picker/form/URL builder |
| `ui/css/go2rtc-source-form.css` | **new** — small styles (badge, hint, error) |
| `ui/js/desktop/apps/network-cameras.js` | setup method "Camera type", manage form, card badge |
| `ui/js/desktop/core/module-loader.js` | load the new script + stylesheet |
| `ui/cfg/go2rtc.js` | main source → guided form |
| `ui/config.html` | load the new script + stylesheet |
| `ui/lang/common/*.json` (16) | `common.go2rtc_source.*` |
| `ui/lang/desktop/*.json` (16) | `desktop.network_cameras.method_type*` |
| `ui/lang/config/go2rtc/*.json` (16) | updated `help.go2rtc.source`, `help.go2rtc.streams` mention of types |
| `ui/i18n_lint_test.go` | register the new module |
| `scripts/test-ui-regressions.mjs` | URL-builder tests, updated script list |
| `ui/desktop_network_cameras_detection_test.go` | updated loader marker |
| `documentation/go2rtc.md`, `internal/server/AGENTS.md`, `internal/tools/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md` | contract + docs |

---

## Batch A — backend (Tasks 1–3)

### Task 1: Source type catalog and validation (`internal/config`)

**Files:**
- Create: `internal/config/go2rtc_source_types.go`
- Create: `internal/config/go2rtc_source_types_test.go`

- [ ] **Step 1: Write the failing test**

```go
package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateGo2RTCSourceURLAcceptsCatalogTypes(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"rtsp://user:pass@192.168.1.20:554/stream1", "rtsp"},
		{"rtsps://camera.local/live#backchannel=0", "rtsp"},
		{"rtspx://camera.local/live", "rtsp"},
		{"http://camera.local/snapshot.jpg", "http"},
		{"https://camera.local/stream.mjpeg", "http"},
		{"onvif://admin:pw@192.168.1.20:8000", "onvif"},
		{"rtmp://192.168.1.20/live/camera1", "rtmp"},
		{"tapo://cloud-password@192.168.1.123?subtype=1", "tapo"},
		{"kasa://alex%40gmail.com:c2VjcmV0MQ==@192.168.1.123:19443/https/stream/mixed", "kasa"},
		{"dvrip://user:pass@192.168.1.123:34567?channel=0&subtype=0", "dvrip"},
		{"bubble://user:pass@192.168.1.123:34567/bubble/live?ch=0&stream=0", "bubble"},
		{"eseecloud://user:pass@192.168.1.123:80/livestream/12", "eseecloud"},
		{"doorbird://admin:pw@192.168.1.123?media=video", "doorbird"},
		{"gopro://172.20.100.51", "gopro"},
		{"hass://192.168.1.123:8123?entity_id=camera.door&token=abc", "hass"},
		{"ring:?device_id=1&refresh_token=x", "ring"},
		{"ring:?device_id=1&refresh_token=x&snapshot", "ring"},
		{"nest:?client_id=a&client_secret=b&refresh_token=c&project_id=d&device_id=e", "nest"},
		{"homekit://192.168.1.50:51827?client_id=a&client_private=b&device_id=c&device_public=d", "homekit"},
		{"roborock://mqtt.eu.roborock.com:8883?u=a&s=b&k=c&did=d&key=e", "roborock"},
		{"wyze://192.168.1.123?uid=U&enr=E&mac=AABBCCDDEEFF&model=HL_CAM4&dtls=true", "wyze"},
		{"ivideon:100-tu5dkUPct39cTp9oNEN2B6/0", "ivideon"},
		{"webrtc:http://192.168.1.123:5000/signaling/camera1?kvs#format=wyze", "webrtc"},
		{"webrtc:wss://example.org/ws", "webrtc"},
		{"webtorrent:?share=huofssuxaty00izc&pwd=k3l2j9djeg8v8r7e", "webtorrent"},
	} {
		got, err := ValidateGo2RTCSourceURL(tc.raw)
		if err != nil || got != tc.want {
			t.Errorf("ValidateGo2RTCSourceURL(%q) = %q, %v; want %q", tc.raw, got, err, tc.want)
		}
	}
}

func TestValidateGo2RTCSourceURLRejectsUnsafeOrIncompleteSources(t *testing.T) {
	for _, raw := range []string{
		"",
		"/data/video.mp4",
		"file:///data/video.mp4",
		"exec:ffmpeg -i secret",
		"echo:python3 x.py",
		"expr:let x = 1",
		"ffmpeg:rtsp://camera.local/live",
		"ffmpeg:device?video=0",
		"device:/dev/video0",
		"ftp://camera.local/live",
		"xiaomi://1:cn@192.168.1.123?did=1",
		"isapi://admin:pw@192.168.1.123:80/",
		"http:///missing-host",
		"rtsp://camera.local/live\nx",
		"hass:Camera1",
		"ring:?device_id=1",
		"ring://host?device_id=1&refresh_token=x",
		"nest:?client_id=a",
		"ivideon:",
		"webrtc:ftp://host/x",
		"webrtc:http:///nohost",
		"webtorrent:?share=a",
		"gopro:",
		"tuya://example.com?device_id=x&email=a&password=b",
	} {
		if got, err := ValidateGo2RTCSourceURL(raw); err == nil {
			t.Errorf("ValidateGo2RTCSourceURL(%q) = %q, want error", raw, got)
		}
	}
}

func TestValidateGo2RTCSourceURLErrorsNeverEchoTheSource(t *testing.T) {
	for _, raw := range []string{"ring:?device_id=secret-device", "hass://host?entity_id=x", "ftp://user:topsecret@host/x"} {
		_, err := ValidateGo2RTCSourceURL(raw)
		if err == nil {
			t.Fatalf("%q unexpectedly valid", raw)
		}
		for _, leak := range []string{"secret-device", "topsecret", "host?"} {
			if strings.Contains(err.Error(), leak) {
				t.Errorf("error %q leaks %q", err, leak)
			}
		}
	}
}

func TestGo2RTCSourceTypeHelpers(t *testing.T) {
	if got := Go2RTCSourceTypeOf("tapo://pw@192.168.1.2"); got != "tapo" {
		t.Errorf("Go2RTCSourceTypeOf = %q", got)
	}
	if got := Go2RTCSourceTypeOf("exec:x"); got != "" {
		t.Errorf("invalid source type = %q, want empty", got)
	}
	if got := Go2RTCStreamSourceType(Go2RTCStreamConfig{Tuya: &Go2RTCTuyaBinding{}}); got != "tuya" {
		t.Errorf("tuya binding type = %q", got)
	}
	if got := Go2RTCStreamSourceType(Go2RTCStreamConfig{Source: "rtsp://camera.local/live"}); got != "rtsp" {
		t.Errorf("rtsp stream type = %q", got)
	}
}

func TestGo2RTCSourceTypesCatalogIsConsistent(t *testing.T) {
	types := Go2RTCSourceTypes()
	seenID := map[string]bool{}
	seenScheme := map[string]bool{}
	for _, kind := range types {
		if kind.ID == "" || kind.Label == "" || len(kind.Schemes) == 0 || kind.DocsURL == "" {
			t.Errorf("incomplete catalog entry %+v", kind)
		}
		if seenID[kind.ID] {
			t.Errorf("duplicate id %q", kind.ID)
		}
		seenID[kind.ID] = true
		for _, scheme := range kind.Schemes {
			if seenScheme[scheme] {
				t.Errorf("duplicate scheme %q", scheme)
			}
			seenScheme[scheme] = true
		}
		switch kind.Mode {
		case "form":
			if len(kind.Fields) == 0 {
				t.Errorf("%s: form type without fields", kind.ID)
			}
		case "url":
			if kind.Template == "" {
				t.Errorf("%s: url type without template", kind.ID)
			}
		case "dialog":
		default:
			t.Errorf("%s: unknown mode %q", kind.ID, kind.Mode)
		}
		for _, field := range kind.Fields {
			if field.Name == "" || field.Placement == "" {
				t.Errorf("%s: incomplete field %+v", kind.ID, field)
			}
		}
	}
	// The catalog is sent to the browser: it must stay data only.
	data, err := json.Marshal(types)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "shape") || strings.Contains(string(data), "requiredQuery") {
		t.Errorf("internal validation rules leaked into JSON")
	}
	// Callers must not be able to mutate the shared catalog.
	types[0].Fields = nil
	if len(Go2RTCSourceTypes()[0].Fields) == 0 {
		t.Errorf("Go2RTCSourceTypes returned the shared slice")
	}
}
```

Note: the first catalog entry is `rtsp` (has fields), so the mutation check is meaningful.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run "Go2RTCSource" -count=1`
Expected: FAIL — `undefined: ValidateGo2RTCSourceURL` (compile error).

- [ ] **Step 3: Write the implementation**

`internal/config/go2rtc_source_types.go`:

```go
package config

import (
	"fmt"
	"net/url"
	"strings"
)

// Go2RTCSourceField is one guided-form input for a camera source type.
// Placement tells the browser where the value goes in the URL: scheme, user,
// password, host, port, path or query:<key>. The server never builds URLs;
// it validates the result with ValidateGo2RTCSourceURL.
type Go2RTCSourceField struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"` // text, host, port, number, secret, select
	Required  bool     `json:"required,omitempty"`
	Default   string   `json:"default,omitempty"`
	Options   []string `json:"options,omitempty"`
	Placement string   `json:"placement"`
	Encode    string   `json:"encode,omitempty"` // base64
}

// Go2RTCSourceType is one camera source kind AuraGo hands to go2rtc. Mode is
// form (guided fields), url (paste a URL from the vendor/go2rtc UI) or dialog
// (Tuya: its own setup dialog).
type Go2RTCSourceType struct {
	ID                   string              `json:"id"`
	Label                string              `json:"label"`
	Schemes              []string            `json:"schemes"`
	Mode                 string              `json:"mode"`
	Template             string              `json:"template,omitempty"`
	DocsURL              string              `json:"docs_url"`
	Hint                 bool                `json:"hint,omitempty"`
	PasswordOnlyUserinfo bool                `json:"password_only_userinfo,omitempty"`
	Fields               []Go2RTCSourceField `json:"fields,omitempty"`

	shape         go2RTCSourceShape
	requiredQuery []string
}

type go2RTCSourceShape int

const (
	go2RTCShapeHost   go2RTCSourceShape = iota // scheme://[userinfo@]host[:port][/path][?query]
	go2RTCShapeQuery                           // scheme:?key=value
	go2RTCShapeOpaque                          // scheme:id
	go2RTCShapeNested                          // webrtc:http(s)|ws(s)://host/...
	go2RTCShapeAny                             // vendor URL copied from go2rtc's UI
	go2RTCShapeTuya                            // ValidateGo2RTCTuyaSource
)

const go2RTCSourceDocs = "https://github.com/AlexxIT/go2rtc/blob/v1.9.14/README.md"

func go2RTCHostField() Go2RTCSourceField {
	return Go2RTCSourceField{Name: "host", Kind: "host", Required: true, Placement: "host"}
}

func go2RTCPortField(def string) Go2RTCSourceField {
	return Go2RTCSourceField{Name: "port", Kind: "port", Default: def, Placement: "port"}
}

func go2RTCUserField(required bool) Go2RTCSourceField {
	return Go2RTCSourceField{Name: "username", Kind: "text", Required: required, Placement: "user"}
}

func go2RTCPasswordField(required bool) Go2RTCSourceField {
	return Go2RTCSourceField{Name: "password", Kind: "secret", Required: required, Placement: "password"}
}

func go2RTCPathField(def string, required bool) Go2RTCSourceField {
	return Go2RTCSourceField{Name: "path", Kind: "text", Default: def, Required: required, Placement: "path"}
}

func go2RTCQueryField(name, kind string, required bool, options ...string) Go2RTCSourceField {
	return Go2RTCSourceField{Name: name, Kind: kind, Required: required, Options: options, Placement: "query:" + name}
}

func go2RTCSchemeField(options ...string) Go2RTCSourceField {
	return Go2RTCSourceField{Name: "scheme", Kind: "select", Required: true, Default: options[0], Options: options, Placement: "scheme"}
}

// go2RTCSourceCatalog is ordered for the type picker. Excluded on purpose:
// exec/echo/expr/ffmpeg/device/file (run programs or read the host), xiaomi
// (needs a go2rtc-side account login), isapi (two-way audio only) and
// host-less hass:<entity> (AuraGo's sidecar is not a Home Assistant add-on).
var go2RTCSourceCatalog = []Go2RTCSourceType{
	{ID: "rtsp", Label: "RTSP", Schemes: []string{"rtsp", "rtsps", "rtspx"}, Mode: "form", DocsURL: go2RTCSourceDocs + "#source-rtsp",
		Template: "rtsp://user:password@192.168.1.20:554/stream1",
		Fields:   []Go2RTCSourceField{go2RTCSchemeField("rtsp", "rtsps", "rtspx"), go2RTCHostField(), go2RTCPortField(""), go2RTCUserField(false), go2RTCPasswordField(false), go2RTCPathField("", false)}},
	{ID: "http", Label: "HTTP (JPEG, MJPEG, FLV, MPEG-TS)", Schemes: []string{"http", "https"}, Mode: "form", DocsURL: go2RTCSourceDocs + "#source-http",
		Template: "http://192.168.1.20/snapshot.jpg",
		Fields:   []Go2RTCSourceField{go2RTCSchemeField("http", "https"), go2RTCHostField(), go2RTCPortField(""), go2RTCUserField(false), go2RTCPasswordField(false), go2RTCPathField("", false)}},
	{ID: "onvif", Label: "ONVIF", Schemes: []string{"onvif"}, Mode: "form", DocsURL: go2RTCSourceDocs + "#source-onvif",
		Template: "onvif://admin:password@192.168.1.20:8000",
		Fields:   []Go2RTCSourceField{go2RTCHostField(), go2RTCPortField(""), go2RTCUserField(false), go2RTCPasswordField(false)}},
	{ID: "rtmp", Label: "RTMP", Schemes: []string{"rtmp"}, Mode: "form", DocsURL: go2RTCSourceDocs + "#source-rtmp",
		Template: "rtmp://192.168.1.20/live/camera1",
		Fields:   []Go2RTCSourceField{go2RTCHostField(), go2RTCPortField(""), go2RTCPathField("", true)}},
	{ID: "tuya", Label: "Tuya", Schemes: []string{"tuya"}, Mode: "dialog", DocsURL: go2RTCSourceDocs + "#source-tuya", shape: go2RTCShapeTuya},
	{ID: "tapo", Label: "TP-Link Tapo", Schemes: []string{"tapo"}, Mode: "form", DocsURL: go2RTCSourceDocs + "#source-tapo", Hint: true, PasswordOnlyUserinfo: true,
		Template: "tapo://cloud-password@192.168.1.20",
		Fields:   []Go2RTCSourceField{go2RTCHostField(), go2RTCPasswordField(true), go2RTCUserField(false), go2RTCQueryField("subtype", "select", false, "", "0", "1")}},
	{ID: "kasa", Label: "TP-Link Kasa", Schemes: []string{"kasa"}, Mode: "form", DocsURL: go2RTCSourceDocs + "#source-kasa", Hint: true,
		Template: "kasa://user%40example.com:base64password@192.168.1.20:19443/https/stream/mixed",
		Fields: []Go2RTCSourceField{go2RTCHostField(), go2RTCPortField("19443"), go2RTCUserField(true),
			{Name: "password", Kind: "secret", Required: true, Placement: "password", Encode: "base64"}, go2RTCPathField("/https/stream/mixed", true)}},
	{ID: "dvrip", Label: "DVR-IP (XMeye)", Schemes: []string{"dvrip"}, Mode: "form", DocsURL: go2RTCSourceDocs + "#source-dvrip", Hint: true,
		Template: "dvrip://user:password@192.168.1.20:34567?channel=0&subtype=0",
		Fields: []Go2RTCSourceField{go2RTCHostField(), go2RTCPortField("34567"), go2RTCUserField(false), go2RTCPasswordField(false),
			go2RTCQueryField("channel", "number", false), go2RTCQueryField("subtype", "select", false, "", "0", "1")}},
	{ID: "bubble", Label: "Bubble", Schemes: []string{"bubble"}, Mode: "form", DocsURL: go2RTCSourceDocs + "#source-bubble",
		Template: "bubble://user:password@192.168.1.20:34567/bubble/live?ch=0&stream=0",
		Fields: []Go2RTCSourceField{go2RTCHostField(), go2RTCPortField("34567"), go2RTCUserField(false), go2RTCPasswordField(false), go2RTCPathField("/bubble/live", true),
			go2RTCQueryField("ch", "number", false), go2RTCQueryField("stream", "number", false)}},
	{ID: "eseecloud", Label: "EseeCloud", Schemes: []string{"eseecloud"}, Mode: "form", DocsURL: go2RTCSourceDocs + "#source-eseecloud",
		Template: "eseecloud://user:password@192.168.1.20:80/livestream/12",
		Fields:   []Go2RTCSourceField{go2RTCHostField(), go2RTCPortField("80"), go2RTCUserField(false), go2RTCPasswordField(false), go2RTCPathField("/livestream/12", true)}},
	{ID: "doorbird", Label: "DoorBird", Schemes: []string{"doorbird"}, Mode: "form", DocsURL: "https://github.com/AlexxIT/go2rtc/blob/v1.9.14/internal/doorbird/README.md",
		Template: "doorbird://admin:password@192.168.1.20?media=video",
		Fields:   []Go2RTCSourceField{go2RTCHostField(), go2RTCUserField(true), go2RTCPasswordField(true), go2RTCQueryField("media", "select", false, "", "video", "audio")}},
	{ID: "gopro", Label: "GoPro", Schemes: []string{"gopro"}, Mode: "form", DocsURL: "https://github.com/AlexxIT/go2rtc/blob/v1.9.14/internal/gopro/README.md",
		Template: "gopro://172.20.100.51",
		Fields:   []Go2RTCSourceField{go2RTCHostField()}},
	{ID: "hass", Label: "Home Assistant", Schemes: []string{"hass"}, Mode: "form", DocsURL: go2RTCSourceDocs + "#source-hass", Hint: true,
		Template:      "hass://192.168.1.20:8123?entity_id=camera.front_door&token=…",
		Fields:        []Go2RTCSourceField{go2RTCHostField(), go2RTCPortField("8123"), go2RTCQueryField("entity_id", "text", true), go2RTCQueryField("token", "secret", true)},
		requiredQuery: []string{"entity_id", "token"}},
	{ID: "ring", Label: "Ring", Schemes: []string{"ring"}, Mode: "form", DocsURL: go2RTCSourceDocs + "#source-ring", Hint: true, shape: go2RTCShapeQuery,
		Template:      "ring:?device_id=…&refresh_token=…",
		Fields:        []Go2RTCSourceField{go2RTCQueryField("device_id", "text", true), go2RTCQueryField("refresh_token", "secret", true)},
		requiredQuery: []string{"device_id", "refresh_token"}},
	{ID: "nest", Label: "Google Nest", Schemes: []string{"nest"}, Mode: "form", DocsURL: go2RTCSourceDocs + "#source-nest", Hint: true, shape: go2RTCShapeQuery,
		Template: "nest:?client_id=…&client_secret=…&refresh_token=…&project_id=…&device_id=…",
		Fields: []Go2RTCSourceField{go2RTCQueryField("client_id", "text", true), go2RTCQueryField("client_secret", "secret", true), go2RTCQueryField("refresh_token", "secret", true),
			go2RTCQueryField("project_id", "text", true), go2RTCQueryField("device_id", "text", true)},
		requiredQuery: []string{"client_id", "client_secret", "refresh_token", "project_id", "device_id"}},
	{ID: "homekit", Label: "Apple HomeKit", Schemes: []string{"homekit"}, Mode: "url", DocsURL: go2RTCSourceDocs + "#source-homekit", Hint: true, shape: go2RTCShapeAny,
		Template: "homekit://192.168.1.50:51827?client_id=…&client_private=…&device_id=…&device_public=…"},
	{ID: "roborock", Label: "Roborock", Schemes: []string{"roborock"}, Mode: "url", DocsURL: go2RTCSourceDocs + "#source-roborock", Hint: true, shape: go2RTCShapeAny,
		Template: "roborock://…"},
	{ID: "wyze", Label: "Wyze", Schemes: []string{"wyze"}, Mode: "url", DocsURL: "https://github.com/AlexxIT/go2rtc/blob/v1.9.14/internal/wyze/README.md", Hint: true,
		Template: "wyze://192.168.1.20?uid=…&enr=…&mac=…&model=…&dtls=true"},
	{ID: "ivideon", Label: "Ivideon", Schemes: []string{"ivideon"}, Mode: "url", DocsURL: go2RTCSourceDocs + "#source-ivideon", shape: go2RTCShapeOpaque,
		Template: "ivideon:100-tu5dkUPct39cTp9oNEN2B6/0"},
	{ID: "webrtc", Label: "WebRTC (WHEP, Kinesis, OpenIPC, SwitchBot, …)", Schemes: []string{"webrtc"}, Mode: "url", DocsURL: go2RTCSourceDocs + "#source-webrtc", Hint: true, shape: go2RTCShapeNested,
		Template: "webrtc:http://192.168.1.20:1984/api/webrtc?src=camera1"},
	{ID: "webtorrent", Label: "WebTorrent (another go2rtc)", Schemes: []string{"webtorrent"}, Mode: "url", DocsURL: go2RTCSourceDocs + "#source-webtorrent", shape: go2RTCShapeQuery,
		Template:      "webtorrent:?share=…&pwd=…",
		requiredQuery: []string{"share", "pwd"}},
}

// Go2RTCSourceTypes returns a deep copy of the catalog for the browser.
func Go2RTCSourceTypes() []Go2RTCSourceType {
	out := make([]Go2RTCSourceType, len(go2RTCSourceCatalog))
	for i, kind := range go2RTCSourceCatalog {
		kind.Schemes = append([]string(nil), kind.Schemes...)
		fields := make([]Go2RTCSourceField, len(kind.Fields))
		for j, field := range kind.Fields {
			field.Options = append([]string(nil), field.Options...)
			fields[j] = field
		}
		kind.Fields = fields
		kind.requiredQuery = append([]string(nil), kind.requiredQuery...)
		out[i] = kind
	}
	return out
}

func go2RTCSourceTypeForScheme(scheme string) (Go2RTCSourceType, bool) {
	for _, kind := range go2RTCSourceCatalog {
		for _, candidate := range kind.Schemes {
			if candidate == scheme {
				return kind, true
			}
		}
	}
	return Go2RTCSourceType{}, false
}

// ValidateGo2RTCSourceURL checks a camera source against the catalog and
// returns its type ID. Messages name the type and the rule, never the URL.
func ValidateGo2RTCSourceURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || parsed.Scheme == "" {
		return "", fmt.Errorf("a complete camera source URL is required")
	}
	kind, ok := go2RTCSourceTypeForScheme(strings.ToLower(parsed.Scheme))
	if !ok {
		return "", fmt.Errorf("source scheme %q is not allowed", strings.ToLower(parsed.Scheme))
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", fmt.Errorf("%s source has an invalid query", kind.ID)
	}
	switch kind.shape {
	case go2RTCShapeTuya:
		if err := ValidateGo2RTCTuyaSource(raw); err != nil {
			return "", err
		}
	case go2RTCShapeHost:
		if parsed.Opaque != "" || strings.TrimSpace(parsed.Hostname()) == "" {
			return "", fmt.Errorf("%s source needs a host", kind.ID)
		}
	case go2RTCShapeQuery:
		if parsed.Opaque != "" || parsed.Host != "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery == "" {
			return "", fmt.Errorf("%s source takes only query parameters", kind.ID)
		}
	case go2RTCShapeOpaque:
		if strings.TrimSpace(parsed.Opaque) == "" || parsed.RawQuery != "" {
			return "", fmt.Errorf("%s source needs a camera ID", kind.ID)
		}
	case go2RTCShapeNested:
		inner, innerErr := url.Parse(parsed.Opaque)
		if innerErr != nil || inner.Hostname() == "" {
			return "", fmt.Errorf("%s source needs an http, https, ws or wss URL", kind.ID)
		}
		switch strings.ToLower(inner.Scheme) {
		case "http", "https", "ws", "wss":
		default:
			return "", fmt.Errorf("%s source needs an http, https, ws or wss URL", kind.ID)
		}
	case go2RTCShapeAny:
		if parsed.Host == "" && strings.TrimSpace(parsed.Opaque) == "" && parsed.RawQuery == "" {
			return "", fmt.Errorf("%s source is incomplete", kind.ID)
		}
	}
	for _, key := range kind.requiredQuery {
		if strings.TrimSpace(query.Get(key)) == "" {
			return "", fmt.Errorf("%s source needs %s", kind.ID, key)
		}
	}
	return kind.ID, nil
}

// Go2RTCSourceTypeOf returns the catalog type of a source, or "" when the
// source is empty or not allowed. The result is never secret.
func Go2RTCSourceTypeOf(raw string) string {
	id, err := ValidateGo2RTCSourceURL(raw)
	if err != nil {
		return ""
	}
	return id
}

// Go2RTCStreamSourceType is the non-secret type label of a configured camera.
func Go2RTCStreamSourceType(stream Go2RTCStreamConfig) string {
	if stream.Tuya != nil {
		return "tuya"
	}
	return Go2RTCSourceTypeOf(stream.Source)
}
```

Check: `"tuya://example.com?..."` must fail — `ValidateGo2RTCTuyaSource` rejects hosts outside its region list. `"ring:?device_id=1"` fails on `refresh_token`. `"gopro:"` → `url.Parse` gives empty host → host-shape error. `"rtsp://camera.local/live\nx"` → `url.Parse` control-character error.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -run "Go2RTCSource" -count=1 -v`
Expected: PASS for all five tests. If an accepted case fails, fix the catalog/shape — never weaken the rejected list.

- [ ] **Step 5: Commit**

```bash
git add internal/config/go2rtc_source_types.go internal/config/go2rtc_source_types_test.go
git commit -m "feat(go2rtc): add the camera source type catalog and per-type validation"
```

### Task 2: Use the catalog in tools and server

**Files:**
- Modify: `internal/tools/go2rtc.go` (`ValidateGo2RTCSource` ~line 989; `SourceType` assignment ~line 546)
- Modify: `internal/tools/go2rtc_media.go` (`applyMediaStatus` ~line 563)
- Modify: `internal/tools/go2rtc_test.go` (`TestValidateGo2RTCSourceAllowsOnlyNetworkSources`)
- Modify: `internal/server/go2rtc_app_handlers.go` (~line 85 `SourceType`; disabled streams; create ~line 551 and mutation ~line 695 messages)
- Modify: `internal/server/go2rtc_sources.go` (`go2RTCSourceChanges`)
- Modify: `internal/server/config_handlers_main.go` (`injectGo2RTCConfig` loop ~line 237; delete `validateGo2RTCSourceURL` ~line 2793)
- Create: `internal/server/go2rtc_source_types_test.go`

Before editing, per CLAUDE.md run impact analysis and report it:
`node ../../../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "ValidateGo2RTCSource" --direction upstream --repo "C:\Users\Andi\Documents\repo\AuraGo"` (index is stale and misses the merged branch files — treat `UNKNOWN` as unresolved and confirm with `git grep -n "ValidateGo2RTCSource\|validateGo2RTCSourceURL\|SourceType = \"tuya\""`).

- [ ] **Step 1: Write failing tests**

Append to `internal/tools/go2rtc_test.go` inside `TestValidateGo2RTCSourceAllowsOnlyNetworkSources` — extend the allowed list with `"tapo://cloud-password@192.168.1.123"`, `"ring:?device_id=1&refresh_token=x"`, `"webrtc:wss://example.org/ws"` and the blocked list with `"echo:python3 x.py"`, `"xiaomi://1:cn@192.168.1.123?did=1"`, `"hass:Camera1"`. Keep every existing entry (including `ffmpeg:rtsp://camera.local/live` as blocked).

Create `internal/server/go2rtc_source_types_test.go`:

```go
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
```

Also add, next to the existing config projection test `TestGo2RTCConfigProjectionAndVaultExtractionNeverExposeSource` in `internal/server/go2rtc_handlers_test.go`, an assertion that the projected stream contains `"source_type":"rtsp"` (that test's source is an RTSP URL). Read the test first and add one `strings.Contains(out, `"source_type":"rtsp"`)` check on the same marshalled output it already inspects.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tools/ -run TestValidateGo2RTCSourceAllowsOnlyNetworkSources -count=1` → FAIL (tapo not allowed).
Run: `go test ./internal/server/ -run "Go2RTCSourceTypes|Go2RTCSourceChanges|ProjectionAndVault" -count=1` → FAIL (`undefined: handleGo2RTCSourceTypes`).

- [ ] **Step 3: Implement**

`internal/tools/go2rtc.go` — replace the body of `ValidateGo2RTCSource` and its comment:

```go
// ValidateGo2RTCSource enforces the camera source catalog (config.ValidateGo2RTCSourceURL).
func ValidateGo2RTCSource(raw string) error {
	_, err := config.ValidateGo2RTCSourceURL(raw)
	return err
}
```

In `internal/tools/go2rtc.go` (the `Go2RTCStreamStatus` builder, currently `if stream.Tuya != nil { entry.SourceType = "tuya" }`) and in `internal/tools/go2rtc_media.go` `applyMediaStatus` (same three lines), replace the `if` block with:

```go
entry.SourceType = config.Go2RTCStreamSourceType(stream)   // in go2rtc.go
out.SourceType = config.Go2RTCStreamSourceType(s)          // in applyMediaStatus
```

`internal/server/go2rtc_app_handlers.go` ~line 84: change

```go
				if camera.Tuya != nil {
					entry.SourceType = "tuya"
					if canManage {
```
to
```go
				entry.SourceType = config.Go2RTCStreamSourceType(camera)
				if camera.Tuya != nil {
					if canManage {
```
(keep the rest of the block). In the same file, where `disabled_streams` entries are built (map with `"source_configured"`), add `"source_type": config.Go2RTCStreamSourceType(stream)` using that loop's stream variable. Replace the create-handler message `"A supported network stream URL or ONVIF profile is required"` with `"Unsupported camera source: "+err.Error()` and the mutation message `"The replacement source is not a supported network URL"` with `"Unsupported replacement source: "+err.Error()` (validator messages never contain the URL — Task 1 test). Then `git grep -n "supported network stream URL\|not a supported network URL"` and update any test that asserted the old text.

`internal/server/go2rtc_sources.go` `go2RTCSourceChanges`:
- change `return nil, fmt.Errorf("invalid %s for camera %q", field, id)` to `return nil, fmt.Errorf("invalid %s for camera %q: %w", field, id, err)`;
- after the role loop, inside the per-stream loop, add:

```go
		if strip {
			// Derived from the Vault source on every read; never persisted.
			delete(stream, "source_type")
		}
```

`internal/server/config_handlers_main.go` `injectGo2RTCConfig` loop: track the main source while the role loop runs and project its type. Replace the loop body's role handling so that, for `role == "main"`, the value read from the vault is kept:

```go
	for _, configured := range cfg.Go2RTC.Streams {
		entry := go2RTCMediaProjection(configured)
		mainSource := configured.Source
		for _, role := range []string{"main", "sub", "snapshot"} {
			field := "source"
			if role != "main" {
				field = role + "_source"
			}
			value := config.Go2RTCRoleSource(configured, role)
			if strings.TrimSpace(value) == "" && vault != nil {
				if stored, err := vault.ReadSecret(config.Go2RTCSourceKey(configured.ID, role)); err == nil {
					value = stored
				}
			}
			present := strings.TrimSpace(value) != ""
			if role == "main" {
				mainSource = value
			}
			entry[field+"_configured"] = present
			entry[field] = ""
			if present {
				entry[field] = "••••••••"
			}
		}
		withSource := configured
		withSource.Source = mainSource
		entry["source_type"] = config.Go2RTCStreamSourceType(withSource)
		entry["detection"] = go2RTCDetectionProjection(config.NormalizeGo2RTCStreamDetection(configured.Detection))
		streams = append(streams, entry)
	}
```

Delete the unused `validateGo2RTCSourceURL` function (verify first: `git grep -n "validateGo2RTCSourceURL"` must show only its definition and, if any, its own test — delete that test case too).

Create `internal/server/go2rtc_source_types.go`:

```go
package server

import (
	"net/http"

	"aurago/internal/config"
)

// handleGo2RTCSourceTypes serves the camera source catalog for the guided
// forms. Static, non-secret data; validation stays server-side.
func handleGo2RTCSourceTypes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=300")
		writeGo2RTCJSON(w, map[string]interface{}{"types": config.Go2RTCSourceTypes()})
	}
}
```

Register it in `registerGo2RTCRoutes` (`internal/server/go2rtc_handlers.go`), next to `/api/go2rtc/status`:

```go
	mux.HandleFunc("/api/go2rtc/source-types", admin(handleGo2RTCSourceTypes()))
```

The desktop app reaches it as `/api/desktop/integrations/go2rtc/source-types` (GET = read operation, admin desktop scope) without further registration.

- [ ] **Step 4: Run tests**

Run: `go build ./... && go vet ./internal/config/ ./internal/tools/ ./internal/server/`
Run: `go test ./internal/config/ ./internal/tools/ -run "(?i)(go2rtc|tuya)" -count=1`
Run: `go test ./internal/server/ -run "(?i)(go2rtc|tuya|camera|UpdateConfig|Route|Auth)" -count=1`
Expected: all `ok`. If a route-policy test enumerates `/api/go2rtc/*` routes, add the new route there with the same policy as `/api/go2rtc/status`.

- [ ] **Step 5: Commit**

```bash
git add internal/tools internal/server internal/config
git commit -m "feat(go2rtc): validate camera sources by type, serve the catalog and report every source type"
```

### Task 3: Backend docs and contracts

**Files:** `documentation/go2rtc.md`, `internal/server/AGENTS.md`, `internal/tools/AGENTS.md`

- [ ] **Step 1:** In `documentation/go2rtc.md`, replace the sentence "Only `rtsp`, `rtsps`, `rtspx`, `http`, `https`, and `onvif` network URLs are accepted." with a short "Camera types" section: a table of the catalog types (label, scheme, form/URL/dialog), the excluded types with the one-line reason each (see the catalog comment in Task 1), and a note that Network Cameras and the config page offer guided forms while Tuya keeps its own dialog (`documentation/go2rtc-tuya.md`).
- [ ] **Step 2:** In `internal/server/AGENTS.md` go2rtc Integration Contract, replace the line "Accept one network source per stable stream ID and only `rtsp`, `rtsps`, `rtspx`, `http`, `https`, or `onvif`. Reject files, devices, exec, custom ffmpeg, arbitrary URLs, and agent-side stream mutation." with: "Accept camera sources only through `config.ValidateGo2RTCSourceURL` (catalog in `internal/config/go2rtc_source_types.go`). Never add exec/echo/expr/ffmpeg/device/file schemes. `GET /api/go2rtc/source-types` serves the form description (admin); `source_type` is derived from the Vault source on every read and stripped on save. Reject agent-side stream mutation."
- [ ] **Step 3:** In `internal/tools/AGENTS.md` "Managed go2rtc media", add one bullet: "`ValidateGo2RTCSource` delegates to the config catalog; `Go2RTCStreamStatus.SourceType` comes from `config.Go2RTCStreamSourceType`."
- [ ] **Step 4:** `git grep -n "rtsps, rtspx, http, https" -- ':!*_test.go' ':!docs/'` and update any remaining user-visible list (agent tool descriptions, help texts outside `ui/lang`). `ui/lang` help texts are handled in Task 6.
- [ ] **Step 5: Commit** — `git commit -am "docs(go2rtc): document the camera source types"` (add new files explicitly if any).

---

## Batch B — browser (Tasks 4–6)

Endpoint contract (from Batch A): `GET /api/go2rtc/source-types` → `{"types":[{"id","label","schemes":[],"mode":"form|url|dialog","template","docs_url","hint","password_only_userinfo","fields":[{"name","kind","required","default","options":[],"placement","encode"}]}]}`. Stream objects carry `source_type` (`"rtsp"`, `"tapo"`, `"tuya"`, … or `""`).

### Task 4: Shared form module

**Files:**
- Create: `ui/js/go2rtc-source-form.js`
- Create: `ui/css/go2rtc-source-form.css`
- Modify: `scripts/test-ui-regressions.mjs` (new test + registration)
- Modify: `ui/i18n_lint_test.go` (register module)
- Modify: `ui/lang/common/<16 locales>.json`

- [ ] **Step 1: Write the failing test** in `scripts/test-ui-regressions.mjs` (add the function near `testNetworkCamerasDetectionAppContracts`, register `['go2rtc source form builds URLs', testGo2RTCSourceFormBuildsURLs]` in the `tests` array):

```js
function go2rtcSourceFormContext() {
  const context = { window: {}, TextEncoder, btoa };
  vm.createContext(context);
  vm.runInContext(read('ui/js/go2rtc-source-form.js'), context);
  return context.window.AuraGoSourceForm;
}

function testGo2RTCSourceFormBuildsURLs() {
  const form = go2rtcSourceFormContext();
  const host = { name: 'host', kind: 'host', required: true, placement: 'host' };
  const types = [
    { id: 'rtsp', label: 'RTSP', schemes: ['rtsp', 'rtsps', 'rtspx'], mode: 'form', fields: [
      { name: 'scheme', kind: 'select', required: true, default: 'rtsp', options: ['rtsp', 'rtsps', 'rtspx'], placement: 'scheme' }, host,
      { name: 'port', kind: 'port', placement: 'port' }, { name: 'username', kind: 'text', placement: 'user' },
      { name: 'password', kind: 'secret', placement: 'password' }, { name: 'path', kind: 'text', placement: 'path' }] },
    { id: 'tapo', label: 'Tapo', schemes: ['tapo'], mode: 'form', password_only_userinfo: true, fields: [host,
      { name: 'password', kind: 'secret', required: true, placement: 'password' }, { name: 'username', kind: 'text', placement: 'user' },
      { name: 'subtype', kind: 'select', options: ['', '0', '1'], placement: 'query:subtype' }] },
    { id: 'kasa', label: 'Kasa', schemes: ['kasa'], mode: 'form', fields: [host, { name: 'port', kind: 'port', default: '19443', placement: 'port' },
      { name: 'username', kind: 'text', required: true, placement: 'user' }, { name: 'password', kind: 'secret', required: true, placement: 'password', encode: 'base64' },
      { name: 'path', kind: 'text', default: '/https/stream/mixed', required: true, placement: 'path' }] },
    { id: 'ring', label: 'Ring', schemes: ['ring'], mode: 'form', fields: [
      { name: 'device_id', kind: 'text', required: true, placement: 'query:device_id' }, { name: 'refresh_token', kind: 'secret', required: true, placement: 'query:refresh_token' }] },
    { id: 'homekit', label: 'HomeKit', schemes: ['homekit'], mode: 'url', template: 'homekit://…' },
    { id: 'tuya', label: 'Tuya', schemes: ['tuya'], mode: 'dialog' },
  ];
  const build = (type, values) => JSON.parse(JSON.stringify(form.buildURL(types, { type, values, error: '' })));

  assert.deepEqual(build('rtsp', { host: '192.168.1.20', port: '554', username: 'ad min', password: 'p@ss:w/rd#1', path: 'stream1' }),
    { url: 'rtsp://ad%20min:p%40ss%3Aw%2Frd%231@192.168.1.20:554/stream1' });
  assert.deepEqual(build('rtsp', { scheme: 'rtsps', host: 'fe80::1' }), { url: 'rtsps://[fe80::1]' });
  assert.deepEqual(build('rtsp', { host: 'cam.local:554' }), { error: 'invalid_host', field: 'host' }, 'a port typed into the host field is rejected');
  assert.deepEqual(build('rtsp', { host: 'cam.local', port: '70000' }), { error: 'invalid_port', field: 'port' });
  assert.deepEqual(build('rtsp', { host: 'cam.local', password: 'x' }), { error: 'required', field: 'username' }, 'a password needs a user unless the type allows password-only');
  assert.deepEqual(build('rtsp', { host: 'cam.local', path: '/a#raw=-i' }), { error: 'invalid_path', field: 'path' });
  assert.deepEqual(build('tapo', { host: '192.168.1.123', password: 'cloud pw', subtype: '1' }), { url: 'tapo://cloud%20pw@192.168.1.123?subtype=1' });
  assert.deepEqual(build('tapo', { host: '192.168.1.123', password: 'HASH', username: 'admin' }), { url: 'tapo://admin:HASH@192.168.1.123' });
  assert.deepEqual(build('kasa', { host: '192.168.1.123', username: 'alex@gmail.com', password: 'secret1' }),
    { url: 'kasa://alex%40gmail.com:c2VjcmV0MQ%3D%3D@192.168.1.123:19443/https/stream/mixed' });
  assert.deepEqual(build('ring', { device_id: '12', refresh_token: 'a&b' }), { url: 'ring:?device_id=12&refresh_token=a%26b' });
  assert.deepEqual(build('ring', { device_id: '12' }), { error: 'required', field: 'refresh_token' });
  assert.deepEqual(build('homekit', { url: '  homekit://x?y=1 ' }), { url: 'homekit://x?y=1' });
  assert.deepEqual(build('homekit', {}), { error: 'required', field: 'url' });
  assert.deepEqual(build('tuya', {}), { error: 'choose_type' });
  assert.deepEqual(build('', {}), { error: 'choose_type' });

  const t = key => key;
  const html = form.markup(types, { type: 'rtsp', values: { password: 'should-not-render' }, error: '' }, { t, emptyLabel: 'keep' });
  assert.doesNotMatch(html, /should-not-render/, 'secrets are never written back into markup');
  assert.doesNotMatch(html, /value="tuya"/, 'Tuya is not offered by the guided form');
  assert.match(html, /<option value="">keep<\/option>/);
  assert.match(html, /data-source-field="host"/);
  assert.match(html, /type="password"[^>]*data-source-field="password"|data-source-field="password"[^>]*type="password"/);
  const escaped = form.markup([{ id: 'x', label: '<b>', schemes: ['x'], mode: 'url', template: '"><script>' }], { type: 'x', values: {}, error: '' }, { t });
  assert.doesNotMatch(escaped, /<b>|"><script>/, 'catalog strings are escaped');
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `node scripts/test-ui-regressions.mjs 2>&1 | grep -E "FAIL|go2rtc source form"`
Expected: `FAIL go2rtc source form builds URLs` (ENOENT on the module).

- [ ] **Step 3: Implement** `ui/js/go2rtc-source-form.js`:

```js
// Guided go2rtc camera source forms shared by Network Cameras and the config page.
// The server catalog (/api/go2rtc/source-types) describes the fields; this module
// renders them, builds the URL in memory and never writes a secret back into markup.
(function () {
    'use strict';

    const prefix = 'common.go2rtc_source.';
    const escape = value => String(value == null ? '' : value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    const enc = encodeURIComponent;
    let catalog = null;

    function loadCatalog(fetchTypes) {
        if (!catalog) {
            catalog = Promise.resolve(fetchTypes()).then(result => Array.isArray(result && result.types) ? result.types : []);
            catalog.catch(() => { catalog = null; });
        }
        return catalog;
    }

    function newState(type) {
        return { type: type || '', values: {}, error: '' };
    }

    function offered(types) {
        return (types || []).filter(type => type.mode === 'form' || type.mode === 'url');
    }

    function find(types, id) {
        return offered(types).find(type => type.id === id) || null;
    }

    function current(state, field) {
        const has = Object.prototype.hasOwnProperty.call(state.values, field.name);
        return String(has ? state.values[field.name] : (field.default || '')).trim();
    }

    function base64(text) {
        let binary = '';
        new TextEncoder().encode(text).forEach(byte => { binary += String.fromCharCode(byte); });
        return btoa(binary);
    }

    function hostPart(value) {
        const colons = (value.match(/:/g) || []).length;
        if (colons >= 2) return /^[0-9A-Fa-f:.%]+$/.test(value) ? '[' + value + ']' : null;
        if (colons === 1) return null;
        return /^[A-Za-z0-9.-]+$/.test(value) ? value : null;
    }

    function buildURL(types, state) {
        const type = find(types, state && state.type);
        if (!type) return { error: 'choose_type' };
        if (type.mode === 'url') {
            const url = String(state.values.url || '').trim();
            return url ? { url } : { error: 'required', field: 'url' };
        }
        const parts = { scheme: type.schemes[0], user: '', password: '', host: '', port: '', path: '', query: [] };
        for (const field of type.fields || []) {
            let value = current(state, field);
            if (!value) {
                if (field.required) return { error: 'required', field: field.name };
                continue;
            }
            if (field.kind === 'port' && !(/^\d{1,5}$/.test(value) && Number(value) >= 1 && Number(value) <= 65535)) return { error: 'invalid_port', field: field.name };
            if (field.kind === 'number' && !/^\d{1,6}$/.test(value)) return { error: 'invalid_number', field: field.name };
            if (field.kind === 'select' && Array.isArray(field.options) && field.options.length && !field.options.includes(value)) return { error: 'invalid_choice', field: field.name };
            if (field.kind === 'host') {
                value = hostPart(value);
                if (!value) return { error: 'invalid_host', field: field.name };
            }
            if (field.encode === 'base64') value = base64(value);
            const placement = field.placement || '';
            if (placement === 'scheme') parts.scheme = value;
            else if (placement === 'user') parts.user = value;
            else if (placement === 'password') parts.password = value;
            else if (placement === 'host') parts.host = value;
            else if (placement === 'port') parts.port = value;
            else if (placement === 'path') {
                if (value.includes('#')) return { error: 'invalid_path', field: field.name };
                parts.path = value.startsWith('/') ? value : '/' + value;
            } else if (placement.startsWith('query:')) parts.query.push(enc(placement.slice(6)) + '=' + enc(value));
        }
        let userinfo = '';
        if (parts.user && parts.password) userinfo = enc(parts.user) + ':' + enc(parts.password) + '@';
        else if (parts.user) userinfo = enc(parts.user) + '@';
        else if (parts.password && type.password_only_userinfo) userinfo = enc(parts.password) + '@';
        else if (parts.password) return { error: 'required', field: 'username' };
        const query = parts.query.length ? '?' + parts.query.join('&') : '';
        if (!parts.host) return { url: parts.scheme + ':' + query };
        return { url: parts.scheme + '://' + userinfo + parts.host + (parts.port ? ':' + parts.port : '') + parts.path + query };
    }

    function errorText(t, result) {
        const field = result.field ? t(prefix + 'field_' + result.field) : '';
        return t(prefix + result.error, { field });
    }

    function fieldControl(field, state, t, c) {
        const label = t(prefix + 'field_' + field.name) + (field.required ? ' *' : '');
        const attrs = ' class="' + escape(c.input) + '" data-source-field="' + escape(field.name) + '"';
        let control;
        if (field.kind === 'select') {
            const selected = current(state, field);
            control = '<select' + attrs + '>' + (field.options || []).map(option => '<option value="' + escape(option) + '"' + (option === selected ? ' selected' : '') + '>' + escape(option === '' ? t(prefix + 'option_default') : option) + '</option>').join('') + '</select>';
        } else if (field.kind === 'secret') {
            control = '<input' + attrs + ' type="password" autocomplete="new-password" value="">';
        } else {
            const numeric = field.kind === 'port' || field.kind === 'number' ? ' inputmode="numeric"' : '';
            const placeholder = field.kind === 'host' ? ' placeholder="192.168.1.20"' : '';
            control = '<input' + attrs + ' type="text" autocomplete="off"' + numeric + placeholder + ' value="' + escape(current(state, field)) + '">';
        }
        return '<label class="' + escape(c.field) + '"><span class="' + escape(c.label) + '">' + escape(label) + '</span>' + control + '</label>';
    }

    function markup(types, state, options) {
        const t = options.t;
        const c = Object.assign({ field: '', label: '', input: '' }, options.classes || {});
        const type = find(types, state.type);
        const choices = '<option value="">' + escape(options.emptyLabel || t(prefix + 'choose_type')) + '</option>' +
            offered(types).map(item => '<option value="' + escape(item.id) + '"' + (type && item.id === type.id ? ' selected' : '') + '>' + escape(item.label) + '</option>').join('');
        let html = '<label class="' + escape(c.field) + '"><span class="' + escape(c.label) + '">' + escape(t(prefix + 'type')) + '</span><select class="' + escape(c.input) + '" data-source-type>' + choices + '</select></label>';
        if (type) {
            if (type.hint) html += '<p class="go2rtc-source-hint">' + escape(t(prefix + 'hint_' + type.id)) + '</p>';
            if (type.mode === 'url') {
                html += '<label class="' + escape(c.field) + '"><span class="' + escape(c.label) + '">' + escape(t(prefix + 'field_url')) + ' *</span><input class="' + escape(c.input) + '" data-source-field="url" type="password" autocomplete="off" value="" placeholder="' + escape(type.template || '') + '"></label>';
            } else {
                html += (type.fields || []).map(field => fieldControl(field, state, t, c)).join('');
            }
            if (type.docs_url) html += '<p class="go2rtc-source-docs"><a href="' + escape(type.docs_url) + '" target="_blank" rel="noopener noreferrer">' + escape(t(prefix + 'docs')) + '</a></p>';
        }
        html += '<p class="go2rtc-source-error" role="alert"' + (state.error ? '' : ' hidden') + '>' + escape(state.error || '') + '</p>';
        return '<div class="go2rtc-source-form" data-source-form>' + html + '</div>';
    }

    function bind(root, state, onChange) {
        const select = root.querySelector('[data-source-type]');
        if (select) select.addEventListener('change', () => {
            state.type = select.value;
            state.values = {};
            state.error = '';
            onChange(true);
        });
        root.querySelectorAll('[data-source-field]').forEach(input => input.addEventListener('input', () => {
            state.values[input.dataset.sourceField] = input.value;
            state.error = '';
            onChange(false);
        }));
    }

    function label(types, id) {
        const type = (types || []).find(item => item.id === id);
        return type ? type.label : String(id || '').toUpperCase();
    }

    window.AuraGoSourceForm = Object.freeze({ loadCatalog, newState, buildURL, errorText, markup, bind, label });
})();
```

Notes for the implementer:
- Secrets: `secret` inputs and the `url` input always render `value=""`; typed values live only in `state.values` (memory). Changing the type clears `state.values`.
- Because secret inputs render empty, a full redraw while the user is typing would lose them — callers redraw only on type change (`onChange(true)`), never on `onChange(false)`.

`ui/css/go2rtc-source-form.css` (theme variables only, no external assets):

```css
.go2rtc-source-form { display: grid; gap: 10px; }
.go2rtc-source-hint, .go2rtc-source-docs { margin: 0; font-size: 0.85em; opacity: 0.85; }
.go2rtc-source-error { margin: 0; color: var(--ds-color-danger-fg, var(--danger, #c62828)); font-size: 0.9em; }
.go2rtc-source-badge { display: inline-block; margin-left: 6px; padding: 1px 6px; border-radius: 999px; font-size: 0.75em; border: 1px solid currentColor; opacity: 0.8; }
```

`ui/i18n_lint_test.go`: add `"js/go2rtc-source-form.js": {name: "t", prefix: "common.go2rtc_source."},` to the map next to `js/go2rtc-tuya.js`. Read how the linter uses `name`/`prefix` first; if it requires literal `name('key')` calls and cannot follow `t(prefix + ...)`, register it the way other dynamic-key modules are registered (check for an existing entry with dynamic keys) or leave it out and explain in the commit message.

Common locale keys (`ui/lang/common/<locale>.json`, append at the end, all 16 locales, real translations; English shown):

```json
"common.go2rtc_source.type": "Camera type",
"common.go2rtc_source.choose_type": "Choose a camera type",
"common.go2rtc_source.keep_current": "Keep the current source",
"common.go2rtc_source.docs": "go2rtc documentation for this type",
"common.go2rtc_source.option_default": "Camera default",
"common.go2rtc_source.field_url": "Source URL",
"common.go2rtc_source.field_scheme": "Protocol",
"common.go2rtc_source.field_host": "IP address or host name",
"common.go2rtc_source.field_port": "Port",
"common.go2rtc_source.field_username": "User name",
"common.go2rtc_source.field_password": "Password",
"common.go2rtc_source.field_path": "Path",
"common.go2rtc_source.field_subtype": "Stream (0 = main, 1 = sub)",
"common.go2rtc_source.field_channel": "Channel",
"common.go2rtc_source.field_ch": "Channel",
"common.go2rtc_source.field_stream": "Stream",
"common.go2rtc_source.field_media": "Media",
"common.go2rtc_source.field_entity_id": "Camera entity ID",
"common.go2rtc_source.field_token": "Long-lived access token",
"common.go2rtc_source.field_device_id": "Device ID",
"common.go2rtc_source.field_refresh_token": "Refresh token",
"common.go2rtc_source.field_client_id": "Client ID",
"common.go2rtc_source.field_client_secret": "Client secret",
"common.go2rtc_source.field_project_id": "Project ID",
"common.go2rtc_source.required": "{{field}} is required.",
"common.go2rtc_source.invalid_port": "{{field}} must be a number from 1 to 65535.",
"common.go2rtc_source.invalid_number": "{{field}} must be a whole number.",
"common.go2rtc_source.invalid_host": "Enter only the IP address or host name, without port or path.",
"common.go2rtc_source.invalid_choice": "Choose a valid value for {{field}}.",
"common.go2rtc_source.invalid_path": "The path must not contain #.",
"common.go2rtc_source.load_failed": "Camera types could not be loaded.",
"common.go2rtc_source.hint_tapo": "Use the TP-Link cloud password, not the RTSP password. Leave the user empty, or use admin with the uppercase MD5/SHA256 hash of the cloud password.",
"common.go2rtc_source.hint_kasa": "User is the Kasa account e-mail. AuraGo encodes the password as go2rtc expects.",
"common.go2rtc_source.hint_dvrip": "Leave channel and stream empty for the defaults. Only TCP is supported.",
"common.go2rtc_source.hint_hass": "Connects to an external Home Assistant with a long-lived access token. The camera stream is rented for about five minutes at a time.",
"common.go2rtc_source.hint_ring": "Get the device ID and refresh token from the go2rtc web interface (Add → Ring).",
"common.go2rtc_source.hint_nest": "Needs a Google Device Access project. Only WebRTC Nest cameras are supported.",
"common.go2rtc_source.hint_homekit": "Pair the camera in the go2rtc web interface first, then paste the homekit:// URL it shows.",
"common.go2rtc_source.hint_roborock": "Copy the roborock:// URL from the go2rtc web interface after logging in there.",
"common.go2rtc_source.hint_wyze": "Copy the wyze:// URL from the go2rtc web interface (Add → Wyze).",
"common.go2rtc_source.hint_webrtc": "WebRTC sources such as WHEP, AWS Kinesis, OpenIPC or SwitchBot. See the documentation for the format option."
```

Every locale must get the same key set; translate naturally (German uses "Kameratyp", etc.).

- [ ] **Step 4: Run tests**

Run: `node scripts/test-ui-regressions.mjs 2>&1 | grep -v "^PASS"` → no output.
Run: `node --check ui/js/go2rtc-source-form.js`
Run: `go test ./ui/ -run "(?i)(i18n|locale|lang)" -count=1` → `ok`.

- [ ] **Step 5: Commit**

```bash
git add ui/js/go2rtc-source-form.js ui/css/go2rtc-source-form.css scripts/test-ui-regressions.mjs ui/i18n_lint_test.go ui/lang/common
git commit -m "feat(go2rtc): add the shared guided camera source form"
```

### Task 5: Network Cameras app

**Files:**
- Modify: `ui/js/desktop/core/module-loader.js` (`'network-cameras'` entry ~line 271)
- Modify: `ui/js/desktop/apps/network-cameras.js`
- Modify: `ui/lang/desktop/<16 locales>.json`
- Modify: `scripts/test-ui-regressions.mjs` (`expectedScripts` ~line 2311; new contract asserts)
- Modify: `ui/desktop_network_cameras_detection_test.go` (loader marker)

- [ ] **Step 1: Failing contract asserts** — in `scripts/test-ui-regressions.mjs`, change `expectedScripts` to insert `'/js/go2rtc-source-form.js'` right after `'/js/go2rtc-tuya.js'`, and add to `testNetworkCamerasDetectionAppContracts` (or a new `testNetworkCamerasSourceTypeContracts`, registered in `tests`):

```js
  const cams = read('ui/js/desktop/apps/network-cameras.js');
  assert.match(cams, /setupMethod\(ctx, 'type', /, 'setup offers the camera type method');
  assert.match(cams, /\/api\/desktop\/integrations\/go2rtc\/source-types/);
  assert.match(sourceBetween(cams, 'async function advanceSetup(state)', 'async function createStream'), /modal\.method === 'type'[\s\S]*AuraGoSourceForm\.buildURL/);
  assert.match(sourceBetween(cams, 'async function createStream(state)', 'async function saveManagedStream'), /modal\.method === 'url' \|\| modal\.method === 'type'/);
  assert.match(sourceBetween(cams, 'async function saveManagedStream(state)', 'async function deleteManagedStream'), /AuraGoSourceForm\.buildURL[\s\S]*confirmDialog/, 'the replacement source is built before the confirmation');
  assert.match(cams, /stream\.source_type/, 'cards show the source type');
```

In `ui/desktop_network_cameras_detection_test.go` the styles marker becomes `"styles: appStyles('/css/desktop-app-network-cameras.css', '/css/desktop-app-network-cameras-detection.css', '/css/go2rtc-tuya.css', '/css/go2rtc-source-form.css')"`.

Run: `node scripts/test-ui-regressions.mjs 2>&1 | grep FAIL` → FAIL lines for the new asserts.

- [ ] **Step 2: Implement**

`module-loader.js` entry:

```js
        'network-cameras': {
            styles: appStyles('/css/desktop-app-network-cameras.css', '/css/desktop-app-network-cameras-detection.css', '/css/go2rtc-tuya.css', '/css/go2rtc-source-form.css'),
            scripts: [
                '/js/go2rtc-tuya.js',
                '/js/go2rtc-source-form.js',
                '/js/desktop/apps/network-cameras-detection-views.js',
                '/js/desktop/apps/network-cameras-detection-zones.js',
                '/js/desktop/apps/network-cameras-detection.js',
                '/js/desktop/apps/network-cameras-media.js',
                '/js/desktop/apps/network-cameras.js'
            ]
        },
```

`network-cameras.js` changes (read each function before editing; keep surrounding code):

1. Helper near `openTuya`:

```js
    async function ensureSourceTypes(state) {
        if (!state.sourceTypes) state.sourceTypes = await window.AuraGoSourceForm.loadCatalog(() => request(state, '/api/desktop/integrations/go2rtc/source-types'));
        return state.sourceTypes;
    }
```

2. `openSetupModal`: add `sourceForm: null` to the modal object.

3. `setupStepMarkup` step 1: add `setupMethod(ctx, 'type', 'camera', 'method_type', 'method_type_help', true, '') +` between the `tuya` and `url` buttons. Step 2: at the top of `if (modal.step === 2) {` add

```js
            if (modal.method === 'type') return '<div class="nc-form">' + window.AuraGoSourceForm.markup(state.sourceTypes || [], modal.sourceForm, { t: ctx.t }) + '</div>';
```

4. `chooseSetupMethod`, after the tuya line:

```js
        if (method === 'type') {
            modal.method = 'type';
            modal.error = '';
            modal.step = 2;
            modal.sourceForm = window.AuraGoSourceForm.newState('');
            modal.busy = true;
            drawModal(state);
            try {
                await ensureSourceTypes(state);
            } catch (error) {
                modal.error = text(state.ctx, 'source_types_failed');
            } finally {
                modal.busy = false;
            }
            return drawModal(state);
        }
```

5. `bindModalEvents`, at the end:

```js
        const sourceRoot = host.querySelector('[data-source-form]');
        if (sourceRoot && state.modal.sourceForm) window.AuraGoSourceForm.bind(sourceRoot, state.modal.sourceForm, redraw => { if (redraw) drawModal(state); });
```

6. `modalAction` back: `if (action === 'back') { modal.step = modal.method === 'type' && modal.step === 4 ? 2 : Math.max(1, modal.step - 1); return drawModal(state); }`

7. `advanceSetup`, right after `modal.error = '';`:

```js
        if (modal.step === 2 && modal.method === 'type') {
            const built = window.AuraGoSourceForm.buildURL(state.sourceTypes || [], modal.sourceForm);
            if (built.error) { modal.error = window.AuraGoSourceForm.errorText(state.ctx.t, built); return drawModal(state); }
            modal.source = built.url;
            modal.step = 4;
            return drawModal(state);
        }
```

8. `createStream`: both `modal.method === 'url'` checks become `(modal.method === 'url' || modal.method === 'type')`, i.e. `source: modal.method === 'url' || modal.method === 'type' ? modal.source : ''`.

9. Manage dialog. `openManageModal`: add `sourceForm: window.AuraGoSourceForm.newState('')` to the modal object; replace the final `drawModal(state);` with

```js
        drawModal(state);
        if (!state.sourceTypes) ensureSourceTypes(state).then(() => { if (state.modal && state.modal.type === 'manage' && state.modal.streamID === streamID && !state.modal.sourceForm.type) drawModal(state); }).catch(() => {});
```

(redraw only while no type is chosen, so nothing typed is lost). In `manageMarkup`, replace the non-Tuya `sourceField` with:

```js
        const sourceField = modal.tuya_account_id ? '' : '<fieldset class="nc-source-fieldset"><legend>' + ctx.esc(text(ctx, 'replace_source')) + '</legend>' + window.AuraGoSourceForm.markup(state.sourceTypes || [], modal.sourceForm, { t: ctx.t, emptyLabel: ctx.t('common.go2rtc_source.keep_current') }) + '</fieldset>';
```

In `saveManagedStream`, before `const confirmations = [];`:

```js
        if (modal.sourceForm && modal.sourceForm.type) {
            const built = window.AuraGoSourceForm.buildURL(state.sourceTypes || [], modal.sourceForm);
            if (built.error) { modal.error = window.AuraGoSourceForm.errorText(state.ctx.t, built); modal.tab = 'general'; return drawModal(state); }
            modal.source = built.url;
        }
```

The existing `replace_confirm_message` confirmation then covers it (it checks `modal.source.trim()`); a successful save already drops the whole modal, including the in-memory form values.

10. `cardMarkup`: in the `<small>` with codecs, append the type: replace `'<small>' + ctx.esc(codecs) + '</small>'` with `'<small>' + ctx.esc(codecs) + (stream.source_type ? '<span class="go2rtc-source-badge">' + ctx.esc(window.AuraGoSourceForm.label(state.sourceTypes, stream.source_type)) + '</span>' : '') + '</small>'`.

11. CSS: in `ui/css/desktop-app-network-cameras.css` add `.nc-source-fieldset { border: 0; padding: 0; margin: 0; display: grid; gap: 10px; } .nc-source-fieldset legend { font-weight: 600; margin-bottom: 6px; }`.

Desktop locale keys (16 locales, append; English):

```json
"desktop.network_cameras.method_type": "Camera type",
"desktop.network_cameras.method_type_help": "Tapo, Kasa, Ring, Nest, DVR, Home Assistant and more",
"desktop.network_cameras.source_types_failed": "Camera types could not be loaded."
```

- [ ] **Step 3: Run tests**

Run: `node --check ui/js/desktop/apps/network-cameras.js && node --check ui/js/desktop/core/module-loader.js`
Run: `node scripts/test-ui-regressions.mjs 2>&1 | grep -v "^PASS"` → no output.
Run: `go test ./ui/ -run "(?i)(NetworkCameras|i18n|locale|lang|External|CookieConsent)" -count=1` → `ok`.

- [ ] **Step 4: Commit**

```bash
git add ui/js/desktop/apps/network-cameras.js ui/js/desktop/core/module-loader.js ui/css/desktop-app-network-cameras.css ui/lang/desktop scripts/test-ui-regressions.mjs ui/desktop_network_cameras_detection_test.go
git commit -m "feat(network-cameras): add cameras of any go2rtc type through guided forms"
```

### Task 6: Config page

**Files:**
- Modify: `ui/config.html` (after the `go2rtc-tuya` tags, lines ~33–34)
- Modify: `ui/cfg/go2rtc.js`
- Modify: `ui/lang/config/go2rtc/<16 locales>.json`
- Modify: `scripts/test-ui-regressions.mjs` (contract asserts)

- [ ] **Step 1: Failing asserts** — add to `testConfigGo2RTCStreamsSaveAsCompleteList` (or a new registered test):

```js
  const cfg = read('ui/cfg/go2rtc.js');
  const html = read('ui/config.html');
  assert.match(html, /<script src="\/js\/go2rtc-source-form\.js\?v=\{\{\.BuildVersion\}\}"><\/script>/);
  assert.match(html, /go2rtc-source-form\.css/);
  assert.match(cfg, /type="hidden" data-path="' \+ escapeAttr\(base \+ '\.source'\)/, 'the built URL travels through the existing source path');
  assert.match(cfg, /\/api\/go2rtc\/source-types/);
  assert.doesNotMatch(sourceBetween(cfg, 'function go2rtcSourceBlock', 'function go2rtcApplySource'), /go2rtcSecret\(base \+ '\.source'/, 'the main source no longer uses the raw secret field');
```

Run `node scripts/test-ui-regressions.mjs 2>&1 | grep FAIL` → FAIL.

- [ ] **Step 2: Implement**

`ui/config.html` after line 34:

```html
    <script src="/js/go2rtc-source-form.js?v={{.BuildVersion}}"></script>
    <link rel="stylesheet" href="/css/go2rtc-source-form.css?v={{.BuildVersion}}">
```

`ui/cfg/go2rtc.js`:

```js
let go2rtcSourceTypes = null;
let go2rtcSourceForms = {};

function go2rtcLoadSourceTypes() {
    return window.AuraGoSourceForm.loadCatalog(() => fetch('/api/go2rtc/source-types', { credentials: 'same-origin' }).then(r => r.ok ? r.json() : Promise.reject(new Error(String(r.status)))));
}

function go2rtcSourceBlock(stream, index, base) {
    const state = go2rtcSourceForms[index] || (go2rtcSourceForms[index] = window.AuraGoSourceForm.newState(''));
    const badge = stream.source_type ? '<span class="go2rtc-source-badge">' + escapeHtml(window.AuraGoSourceForm.label(go2rtcSourceTypes, stream.source_type)) + '</span>' : '';
    const empty = t(stream.source_configured ? 'common.go2rtc_source.keep_current' : 'common.go2rtc_source.choose_type');
    return '<div class="field-group"><div class="field-label">' + t('config.go2rtc.source') + badge + '</div><div class="field-help">' + t('help.go2rtc.source') + '</div>' +
        '<input type="hidden" data-path="' + escapeAttr(base + '.source') + '" data-original="' + escapeAttr(stream.source || '') + '" value="' + escapeAttr(stream.source || '') + '">' +
        '<div data-go2rtc-source="' + index + '">' + window.AuraGoSourceForm.markup(go2rtcSourceTypes || [], state, { t, emptyLabel: empty, classes: { field: 'field-group', label: 'field-label', input: 'field-input' } }) + '</div></div>';
}

function go2rtcApplySource(index) {
    const state = go2rtcSourceForms[index];
    const hidden = document.querySelector('[data-path="go2rtc.streams.' + index + '.source"]');
    const root = document.querySelector('[data-go2rtc-source="' + index + '"]');
    if (!state || !hidden || !root) return;
    const built = state.type ? window.AuraGoSourceForm.buildURL(go2rtcSourceTypes || [], state) : null;
    hidden.value = built && built.url ? built.url : hidden.dataset.original;
    const error = root.querySelector('.go2rtc-source-error');
    if (error) {
        error.textContent = built && built.error ? window.AuraGoSourceForm.errorText(t, built) : '';
        error.hidden = !(built && built.error);
    }
    hidden.dispatchEvent(new Event('change', { bubbles: true }));
}

function go2rtcSourceMarkup(index, state) {
    const stream = (go2rtcEnsureData().streams || [])[index] || {};
    const empty = t(stream.source_configured ? 'common.go2rtc_source.keep_current' : 'common.go2rtc_source.choose_type');
    return window.AuraGoSourceForm.markup(go2rtcSourceTypes || [], state, { t, emptyLabel: empty, classes: { field: 'field-group', label: 'field-label', input: 'field-input' } });
}

// Binds one camera's form; a type change redraws and rebinds only that form.
function go2rtcBindSourceForm(root) {
    const index = Number(root.dataset.go2rtcSource);
    const state = go2rtcSourceForms[index];
    const form = root.querySelector('[data-source-form]');
    if (!state || !form) return;
    window.AuraGoSourceForm.bind(form, state, redraw => {
        if (redraw) {
            root.innerHTML = go2rtcSourceMarkup(index, state);
            go2rtcBindSourceForm(root);
        }
        go2rtcApplySource(index);
    });
}

function go2rtcBindSourceForms() {
    document.querySelectorAll('[data-go2rtc-source]').forEach(go2rtcBindSourceForm);
}
```

`go2rtcSourceBlock` above should call `go2rtcSourceMarkup(index, state)` for the inner markup instead of repeating the `markup(...)` call (same output, one place).

In `renderGo2RTCSection`, replace the non-Tuya branch `else html += go2rtcSecret(base + '.source', …);` with `else html += go2rtcSourceBlock(stream, index, base);`. After `attachChangeListeners();` add `go2rtcBindSourceForms();` and, once per page:

```js
    if (go2rtcSourceTypes === null) {
        go2rtcSourceTypes = [];
        go2rtcLoadSourceTypes().then(types => {
            go2rtcSourceTypes = types;
            if (window.AuraConfigState && window.AuraConfigState.syncFromDOM) window.AuraConfigState.syncFromDOM();
            renderGo2RTCSection(null);
        }).catch(() => { go2rtcSourceTypes = null; showToast(t('common.go2rtc_source.load_failed'), 'error'); });
    }
```

(check `showToast` exists in the config page — it is used by `go2rtcTuyaOpen`.) In `go2rtcAddStream` and `go2rtcRemoveStream` reset `go2rtcSourceForms = {};` before `go2rtcSetStreams(...)` (indexes shift).

`ui/lang/config/go2rtc/<locale>.json` (16 locales): update `help.go2rtc.source` to: "Choose the camera type and fill in its fields; AuraGo builds the go2rtc source URL. Supported: RTSP, HTTP, ONVIF, RTMP, Tapo, Kasa, DVR-IP, Bubble, EseeCloud, DoorBird, GoPro, Home Assistant, Ring, Nest, HomeKit, Roborock, Wyze, Ivideon, WebRTC and WebTorrent. Tuya cameras use “Add Tuya camera”. The complete URL is stored only in the encrypted Vault." Also update the sub/snapshot help if it lists schemes (`git grep -n "rtsps" ui/lang/config/go2rtc/en.json`).

- [ ] **Step 3: Run tests**

Run: `node --check ui/cfg/go2rtc.js`
Run: `node scripts/test-ui-regressions.mjs 2>&1 | grep -v "^PASS"` → no output.
Run: `go test ./ui/ -run "(?i)(go2rtc|i18n|locale|lang|External)" -count=1` → `ok` (`TestGo2RTCConfigLocalesHaveCompleteKeyCoverage` must pass).

- [ ] **Step 4: Commit**

```bash
git add ui/config.html ui/cfg/go2rtc.js ui/lang/config/go2rtc scripts/test-ui-regressions.mjs
git commit -m "feat(config): configure any go2rtc camera type from the config page"
```

---

## Batch C — verification and landing

### Task 7: Browser check, review, land

- [ ] **Step 1: Browser tests that already exist**: `go test ./ui/ -run "(?i)(go2rtc|NetworkCameras)" -count=1 -timeout 30m` (these include Chrome-driven tests; if one fails, reproduce with Windows animations both on and off — memory `reference_windows_motion_browser_tests`).
- [ ] **Step 2: UI docs**: add to `ui/js/desktop/apps/AGENTS.md` under "Tuya camera setup" a sibling section "Camera source types": shared module `js/go2rtc-source-form.js` (+ css), strings `common.go2rtc_source.*`, secrets only in memory, redraw only on type change, Tuya stays in its dialog.
- [ ] **Step 3: Full checks**: `go build ./... && go vet ./internal/... ./ui/`, `go test ./internal/config/ ./internal/tools/ ./internal/server/ -count=1`, `go test ./ui/ -count=1` (known unrelated failure on main: `TestDashboardCardsFollowSectionPlacementAndAnatomy`), `node scripts/test-ui-regressions.mjs`, `git diff --check main...HEAD`.
- [ ] **Step 4: One combined review** (Critical/Important only) over `git diff 94451e7ee..HEAD`.
- [ ] **Step 5: GitNexus** `detect-changes --scope compare --base-ref main --repo "C:\Users\Andi\Documents\repo\AuraGo"` (expect PARTIAL; note it).
- [ ] **Step 6: Land** (coordinator, not a subagent): if `main` moved, `git merge main` in the worktree and re-run Step 3. Then fast-forward main with `git -C C:\Users\Andi\Documents\repo\AuraGo merge --ff-only claude/go2rtc-source-types` — only when `.git/index.lock` is absent; if the ff merge refuses because of other sessions' uncommitted files, stop and report instead of forcing. Do not push. Do not delete the Codex worktree or branch.
