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
		{"http://192.168.1.2/snap.cgi?user=admin;pwd=x", "http"},
		{"http://192.168.1.2/snap.cgi?user=admin&pwd=50%off", "http"},
		{"rtsp://cam.local/live?a=1;b=2", "rtsp"},
		{"rtsp://u:p@cam.local/live?x=50%zz", "rtsp"},
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

func TestValidateGo2RTCSourceURLNeverEchoesUnknownSchemes(t *testing.T) {
	_, err := ValidateGo2RTCSourceURL("admin:secretpw@192.168.1.2/stream")
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "admin") || strings.Contains(err.Error(), "secretpw") {
		t.Fatalf("error leaks URL text: %q", err)
	}
	_, err = ValidateGo2RTCSourceURL("exec:x")
	if err == nil || !strings.Contains(err.Error(), `"exec"`) {
		t.Fatalf("exec error = %v; want it to name \"exec\"", err)
	}
}
