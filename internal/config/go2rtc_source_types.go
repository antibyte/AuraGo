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
		scheme := strings.ToLower(parsed.Scheme)
		// Only name schemes that are deliberately blocked; an unknown "scheme"
		// may be pasted userinfo (user:pw@host) and must not be echoed.
		switch scheme {
		case "exec", "echo", "expr", "ffmpeg", "device", "file", "xiaomi", "isapi", "ftp":
			return "", fmt.Errorf("source scheme %q is not allowed", scheme)
		}
		return "", fmt.Errorf("this camera source type is not supported")
	}
	// Legacy sources may have sloppy queries (";" or stray "%"). ParseQuery
	// still returns every pair it could parse, so the error is ignored on purpose.
	query, _ := url.ParseQuery(parsed.RawQuery)
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
