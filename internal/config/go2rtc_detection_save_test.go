package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
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
