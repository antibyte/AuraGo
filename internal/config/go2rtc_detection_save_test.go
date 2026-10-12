package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGo2RTCDetectionSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("go2rtc:\n  enabled: false\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Go2RTC.Detection = Go2RTCDetectionConfig{
		Enabled: true, Model: "accurate", Backend: "gpu", RetentionDays: 30, MaxStorageMB: 4096,
		Schedule: []Go2RTCDetectionWindow{{Days: []string{"sat", "sun"}, From: "08:00", To: "20:00"}},
	}
	cfg.Go2RTC.Streams = []Go2RTCStreamConfig{{ID: "garage", Name: "Garage", Enabled: true, Detection: Go2RTCStreamDetectionConfig{
		Enabled: true, IntervalSeconds: 5, MinScore: 0.7, CooldownSeconds: 0,
		Zones:   []Go2RTCDetectionZone{{Name: "Door", Points: [][2]float64{{0, 0}, {0.5, 0}, {0.5, 1}}}},
		Actions: Go2RTCDetectionActions{Message: true, Channel: "discord", Agent: true, AgentInstruction: "Look at the door", Desktop: false},
	}}}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(reloaded.Go2RTC.Detection, cfg.Go2RTC.Detection) {
		t.Fatalf("global detection after save = %+v, want %+v", reloaded.Go2RTC.Detection, cfg.Go2RTC.Detection)
	}
	if len(reloaded.Go2RTC.Streams) != 1 {
		t.Fatalf("streams after save = %+v, want exactly the garage stream", reloaded.Go2RTC.Streams)
	}
	if !reflect.DeepEqual(reloaded.Go2RTC.Streams[0].Detection, cfg.Go2RTC.Streams[0].Detection) {
		t.Fatalf("stream detection after save = %+v, want %+v", reloaded.Go2RTC.Streams[0].Detection, cfg.Go2RTC.Streams[0].Detection)
	}
}

func TestGo2RTCDetectionTemplateMatchesLoadDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	var doc struct {
		Go2RTC struct {
			Detection yaml.Node `yaml:"detection"`
		} `yaml:"go2rtc"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse template: %v", err)
	}
	if doc.Go2RTC.Detection.Kind == 0 {
		t.Fatal("config_template.yaml has no go2rtc.detection block")
	}
	raw, err := yaml.Marshal(&doc.Go2RTC.Detection)
	if err != nil {
		t.Fatal(err)
	}
	var tpl Go2RTCDetectionConfig
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&tpl); err != nil {
		t.Fatalf("decode go2rtc.detection block: %v", err)
	}
	cfg, err := loadGo2RTCDetectionYAML(t, "{}\n")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(tpl, cfg.Go2RTC.Detection) {
		t.Fatalf("template = %+v, load defaults = %+v", tpl, cfg.Go2RTC.Detection)
	}
}

// Save replaces the value node of go2rtc.detection, so only comments on the key
// survive. The enum and range notes and the schedule example therefore live in the
// head comment above the key; this test saves the real template twice and checks
// that the second save changes nothing and that those head comments are still there.
func TestGo2RTCDetectionTemplateNotesSurviveSaves(t *testing.T) {
	template, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, template, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var saved [2][]byte
	for i := range saved {
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load before save %d: %v", i+1, err)
		}
		if err := cfg.Save(path); err != nil {
			t.Fatalf("Save %d: %v", i+1, err)
		}
		if saved[i], err = os.ReadFile(path); err != nil {
			t.Fatalf("read config after save %d: %v", i+1, err)
		}
	}
	if !bytes.Equal(saved[0], saved[1]) {
		t.Fatalf("a second save changed the file:\n--- first save ---\n%s\n--- second save ---\n%s", saved[0], saved[1])
	}

	var root yaml.Node
	if err := yaml.Unmarshal(saved[1], &root); err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	go2rtc := mappingNodeValue(yamlDocumentRoot(&root), "go2rtc")
	if go2rtc == nil || go2rtc.Kind != yaml.MappingNode {
		t.Fatal("saved config has no go2rtc mapping")
	}
	headComment := func(key string) string {
		for i := 0; i+1 < len(go2rtc.Content); i += 2 {
			if go2rtc.Content[i].Value == key {
				return go2rtc.Content[i].HeadComment
			}
		}
		t.Fatalf("saved go2rtc mapping has no %q key", key)
		return ""
	}
	for _, tc := range []struct {
		key  string
		want []string
	}{
		{"streams", []string{"front-door", "detection:", "Driveway", "agent_instruction"}},
		{"detection", []string{"master switch", "go2rtc.enabled", "model:", "backend:", "retention_days", "max_storage_mb", "schedule:", `from: "22:00"`, "crosses midnight"}},
	} {
		comment := headComment(tc.key)
		for _, want := range tc.want {
			if !strings.Contains(comment, want) {
				t.Errorf("head comment of go2rtc.%s lost %q after saving; it is:\n%s", tc.key, want, comment)
			}
		}
	}
}
