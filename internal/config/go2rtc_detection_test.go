package config

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func loadGo2RTCDetectionYAML(t *testing.T, content string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return Load(path)
}

func TestGo2RTCDetectionLoadDefaults(t *testing.T) {
	cfg, err := loadGo2RTCDetectionYAML(t, "go2rtc:\n  streams:\n    - id: front-door\n      name: Front door\n      enabled: true\n    - id: garage\n      detection:\n        enabled: true\n")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Go2RTCDetectionConfig{Model: "standard", Backend: "auto", RetentionDays: 14, MaxStorageMB: 2048, Schedule: []Go2RTCDetectionWindow{}}
	if !reflect.DeepEqual(cfg.Go2RTC.Detection, want) {
		t.Fatalf("global detection = %+v, want %+v", cfg.Go2RTC.Detection, want)
	}
	if got := cfg.Go2RTC.Streams[0].Detection; !reflect.DeepEqual(got, DefaultGo2RTCStreamDetection(false)) {
		t.Fatalf("unconfigured stream detection = %+v", got)
	}
	if got := cfg.Go2RTC.Streams[1].Detection; !reflect.DeepEqual(got, DefaultGo2RTCStreamDetection(true)) {
		t.Fatalf("interval 0 must apply the default set and keep enabled: %+v", got)
	}
	d := DefaultGo2RTCStreamDetection(true)
	if !d.Enabled || d.IntervalSeconds != 2 || d.MinScore != 0.5 || d.CooldownSeconds != 300 || d.Zones == nil || len(d.Zones) != 0 {
		t.Fatalf("default set = %+v", d)
	}
	if d.Actions != (Go2RTCDetectionActions{Channel: "all", Desktop: true}) {
		t.Fatalf("default actions = %+v", d.Actions)
	}
}

func TestGo2RTCDetectionExplicitValuesSurviveLoad(t *testing.T) {
	cfg, err := loadGo2RTCDetectionYAML(t, `go2rtc:
  detection:
    enabled: true
    model: Accurate
    backend: GPU
    retention_days: 30
    max_storage_mb: 500
    schedule:
      - days: [Mon, fri, fri]
        from: "22:00"
        to: "06:00"
  streams:
    - id: front-door
      enabled: true
      detection:
        enabled: true
        interval_seconds: 5
        min_score: 0.7
        cooldown_seconds: 0
        zones:
          - name: " Driveway "
            points: [[0.1, 0.5], [0.9, 0.5], [0.9, 1.0]]
        actions:
          message: true
          channel: Telegram
          agent: true
          agent_instruction: "Check the driveway"
          desktop: false
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	d := cfg.Go2RTC.Detection
	if !d.Enabled || d.Model != "accurate" || d.Backend != "gpu" || d.RetentionDays != 30 || d.MaxStorageMB != 500 {
		t.Fatalf("global detection = %+v", d)
	}
	if want := []Go2RTCDetectionWindow{{Days: []string{"mon", "fri"}, From: "22:00", To: "06:00"}}; !reflect.DeepEqual(d.Schedule, want) {
		t.Fatalf("schedule = %+v, want %+v", d.Schedule, want)
	}
	s := cfg.Go2RTC.Streams[0].Detection
	if s.IntervalSeconds != 5 || s.MinScore != 0.7 || s.CooldownSeconds != 0 || len(s.Zones) != 1 {
		t.Fatalf("stream detection = %+v", s)
	}
	if s.Zones[0].Name != "Driveway" || s.Zones[0].Points[2] != [2]float64{0.9, 1.0} {
		t.Fatalf("zone = %+v", s.Zones[0])
	}
	if s.Actions != (Go2RTCDetectionActions{Message: true, Channel: "telegram", Agent: true, AgentInstruction: "Check the driveway"}) {
		t.Fatalf("actions = %+v", s.Actions)
	}
}

func TestGo2RTCDetectionLoadRejectsInvalidValues(t *testing.T) {
	for name, tc := range map[string]struct{ yaml, field string }{
		"retention": {"go2rtc:\n  detection:\n    retention_days: 400\n", "go2rtc.detection.retention_days"},
		"model":     {"go2rtc:\n  detection:\n    model: huge\n", "go2rtc.detection.model"},
		"schedule":  {"go2rtc:\n  detection:\n    schedule:\n      - days: [mon]\n        from: \"25:00\"\n        to: \"06:00\"\n", "go2rtc.detection.schedule[0].from"},
		"min score": {"go2rtc:\n  streams:\n    - id: cam\n      detection:\n        interval_seconds: 2\n        min_score: 0.2\n        actions:\n          channel: all\n", "go2rtc.streams[cam].detection.min_score"},
	} {
		_, err := loadGo2RTCDetectionYAML(t, tc.yaml)
		var fieldErr *Go2RTCDetectionFieldError
		if err == nil || !errors.As(err, &fieldErr) || fieldErr.Field != tc.field {
			t.Errorf("%s: err = %v, want field %q", name, err, tc.field)
		}
	}
}

func validStreamDetection() Go2RTCStreamDetectionConfig {
	d := DefaultGo2RTCStreamDetection(true)
	d.Zones = []Go2RTCDetectionZone{{Name: "Driveway", Points: [][2]float64{{0, 0}, {1, 0}, {1, 1}}}}
	return d
}

func manyDetectionZones(n int) []Go2RTCDetectionZone {
	zones := make([]Go2RTCDetectionZone, n)
	for i := range zones {
		zones[i] = Go2RTCDetectionZone{Name: "Zone " + string(rune('A'+i)), Points: [][2]float64{{0, 0}, {1, 0}, {1, 1}}}
	}
	return zones
}

func TestValidateGo2RTCStreamDetectionFields(t *testing.T) {
	if err := ValidateGo2RTCStreamDetection(validStreamDetection()); err != nil {
		t.Fatalf("valid detection rejected: %v", err)
	}
	for name, tc := range map[string]struct {
		mutate func(*Go2RTCStreamDetectionConfig)
		field  string
	}{
		"interval zero":     {func(d *Go2RTCStreamDetectionConfig) { d.IntervalSeconds = 0 }, "interval_seconds"},
		"interval high":     {func(d *Go2RTCStreamDetectionConfig) { d.IntervalSeconds = 31 }, "interval_seconds"},
		"min score low":     {func(d *Go2RTCStreamDetectionConfig) { d.MinScore = 0.29 }, "min_score"},
		"min score high":    {func(d *Go2RTCStreamDetectionConfig) { d.MinScore = 0.96 }, "min_score"},
		"min score NaN":     {func(d *Go2RTCStreamDetectionConfig) { d.MinScore = math.NaN() }, "min_score"},
		"cooldown negative": {func(d *Go2RTCStreamDetectionConfig) { d.CooldownSeconds = -1 }, "cooldown_seconds"},
		"cooldown high":     {func(d *Go2RTCStreamDetectionConfig) { d.CooldownSeconds = 86_401 }, "cooldown_seconds"},
		"nine zones":        {func(d *Go2RTCStreamDetectionConfig) { d.Zones = manyDetectionZones(9) }, "zones"},
		"empty zone name":   {func(d *Go2RTCStreamDetectionConfig) { d.Zones[0].Name = "" }, "zones[0].name"},
		"long zone name":    {func(d *Go2RTCStreamDetectionConfig) { d.Zones[0].Name = strings.Repeat("ä", 49) }, "zones[0].name"},
		"control character": {func(d *Go2RTCStreamDetectionConfig) { d.Zones[0].Name = "Drive\nway" }, "zones[0].name"},
		"duplicate zone": {func(d *Go2RTCStreamDetectionConfig) {
			d.Zones = append(d.Zones, Go2RTCDetectionZone{Name: "driveway", Points: [][2]float64{{0, 0}, {1, 0}, {1, 1}}})
		}, "zones[1].name"},
		"two points":         {func(d *Go2RTCStreamDetectionConfig) { d.Zones[0].Points = d.Zones[0].Points[:2] }, "zones[0].points"},
		"33 points":          {func(d *Go2RTCStreamDetectionConfig) { d.Zones[0].Points = make([][2]float64, 33) }, "zones[0].points"},
		"coordinate outside": {func(d *Go2RTCStreamDetectionConfig) { d.Zones[0].Points[1] = [2]float64{1.5, 0} }, "zones[0].points[1]"},
		"channel":            {func(d *Go2RTCStreamDetectionConfig) { d.Actions.Channel = "sms" }, "actions.channel"},
		"empty channel":      {func(d *Go2RTCStreamDetectionConfig) { d.Actions.Channel = "" }, "actions.channel"},
		"instruction":        {func(d *Go2RTCStreamDetectionConfig) { d.Actions.AgentInstruction = strings.Repeat("x", 501) }, "actions.agent_instruction"},
	} {
		d := validStreamDetection()
		tc.mutate(&d)
		err := ValidateGo2RTCStreamDetection(d)
		var fieldErr *Go2RTCDetectionFieldError
		if !errors.As(err, &fieldErr) || fieldErr.Field != tc.field {
			t.Errorf("%s: err = %v, want field %q", name, err, tc.field)
		}
	}
	edge := validStreamDetection()
	edge.IntervalSeconds, edge.MinScore, edge.CooldownSeconds = 30, 0.95, 86_400
	edge.Actions.AgentInstruction = strings.Repeat("ü", 500)
	edge.Zones = manyDetectionZones(8)
	edge.Zones[0].Name = strings.Repeat("ä", 48)
	edge.Zones[1].Points = make([][2]float64, 32)
	if err := ValidateGo2RTCStreamDetection(edge); err != nil {
		t.Fatalf("upper bounds rejected: %v", err)
	}
	edge.IntervalSeconds, edge.MinScore, edge.CooldownSeconds = 1, 0.30, 0
	if err := ValidateGo2RTCStreamDetection(edge); err != nil {
		t.Fatalf("lower bounds rejected: %v", err)
	}
	for _, channel := range Go2RTCDetectionChannels {
		d := validStreamDetection()
		d.Actions.Channel = channel
		if err := ValidateGo2RTCStreamDetection(d); err != nil {
			t.Fatalf("channel %q rejected: %v", channel, err)
		}
	}
}

func TestValidateGo2RTCDetectionSettingsFields(t *testing.T) {
	valid := func() Go2RTCDetectionConfig {
		return Go2RTCDetectionConfig{Enabled: true, Model: "standard", Backend: "auto", RetentionDays: 14, MaxStorageMB: 2048,
			Schedule: []Go2RTCDetectionWindow{{Days: []string{"mon"}, From: "22:00", To: "06:00"}}}
	}
	if err := ValidateGo2RTCDetectionSettings(valid()); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
	tooMany := make([]Go2RTCDetectionWindow, Go2RTCDetectionMaxWindows+1)
	for i := range tooMany {
		tooMany[i] = Go2RTCDetectionWindow{Days: []string{"mon"}, From: "00:00", To: "01:00"}
	}
	for name, tc := range map[string]struct {
		mutate func(*Go2RTCDetectionConfig)
		field  string
	}{
		"model":          {func(d *Go2RTCDetectionConfig) { d.Model = "huge" }, "model"},
		"backend":        {func(d *Go2RTCDetectionConfig) { d.Backend = "cuda" }, "backend"},
		"retention low":  {func(d *Go2RTCDetectionConfig) { d.RetentionDays = 0 }, "retention_days"},
		"retention high": {func(d *Go2RTCDetectionConfig) { d.RetentionDays = 366 }, "retention_days"},
		"storage low":    {func(d *Go2RTCDetectionConfig) { d.MaxStorageMB = 99 }, "max_storage_mb"},
		"storage high":   {func(d *Go2RTCDetectionConfig) { d.MaxStorageMB = 1_000_001 }, "max_storage_mb"},
		"no days":        {func(d *Go2RTCDetectionConfig) { d.Schedule[0].Days = nil }, "schedule[0].days"},
		"bad day":        {func(d *Go2RTCDetectionConfig) { d.Schedule[0].Days = []string{"mon", "xyz"} }, "schedule[0].days[1]"},
		"bad from":       {func(d *Go2RTCDetectionConfig) { d.Schedule[0].From = "24:00" }, "schedule[0].from"},
		"short to":       {func(d *Go2RTCDetectionConfig) { d.Schedule[0].To = "6:00" }, "schedule[0].to"},
		"too many":       {func(d *Go2RTCDetectionConfig) { d.Schedule = tooMany }, "schedule"},
	} {
		d := valid()
		tc.mutate(&d)
		err := ValidateGo2RTCDetectionSettings(d)
		var fieldErr *Go2RTCDetectionFieldError
		if !errors.As(err, &fieldErr) || fieldErr.Field != tc.field {
			t.Errorf("%s: err = %v, want field %q", name, err, tc.field)
		}
	}
	edge := valid()
	edge.RetentionDays, edge.MaxStorageMB = 365, 1_000_000
	edge.Schedule[0].From, edge.Schedule[0].To = "08:00", "08:00"
	if err := ValidateGo2RTCDetectionSettings(edge); err != nil {
		t.Fatalf("upper bounds rejected: %v", err)
	}
	edge.RetentionDays, edge.MaxStorageMB, edge.Model, edge.Backend = 1, 100, "accurate", "gpu"
	if err := ValidateGo2RTCDetectionSettings(edge); err != nil {
		t.Fatalf("lower bounds rejected: %v", err)
	}
}

func TestParseGo2RTCDetectionClockAndDay(t *testing.T) {
	for in, want := range map[string]int{"00:00": 0, "06:15": 375, "23:59": 1439} {
		if got, ok := ParseGo2RTCDetectionClock(in); !ok || got != want {
			t.Fatalf("ParseGo2RTCDetectionClock(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "24:00", "12:60", "1:00", "12-00", "+1:00", "12:5a", "012:00"} {
		if _, ok := ParseGo2RTCDetectionClock(bad); ok {
			t.Fatalf("ParseGo2RTCDetectionClock(%q) accepted", bad)
		}
	}
	if d, ok := ParseGo2RTCDetectionDay(" SUN "); !ok || d != time.Sunday {
		t.Fatalf("sun = %v, %v", d, ok)
	}
	if d, ok := ParseGo2RTCDetectionDay("mon"); !ok || d != time.Monday {
		t.Fatalf("mon = %v, %v", d, ok)
	}
	if _, ok := ParseGo2RTCDetectionDay("monday"); ok {
		t.Fatal("long day names are not part of the contract")
	}
}

func TestNormalizeGo2RTCDetectionDoesNotAliasInput(t *testing.T) {
	in := Go2RTCConfig{Streams: []Go2RTCStreamConfig{{ID: "cam", Detection: Go2RTCStreamDetectionConfig{
		IntervalSeconds: 3, MinScore: 0.6,
		Zones:   []Go2RTCDetectionZone{{Name: " Porch ", Points: [][2]float64{{0, 0}, {1, 0}, {1, 1}}}},
		Actions: Go2RTCDetectionActions{Channel: " Telegram "},
	}}}}
	out := in
	NormalizeGo2RTCDetection(&out)
	if got := in.Streams[0].Detection; got.Zones[0].Name != " Porch " || got.Actions.Channel != " Telegram " {
		t.Fatalf("normalization mutated its input: %+v", got)
	}
	if got := out.Streams[0].Detection; got.Zones[0].Name != "Porch" || got.Actions.Channel != "telegram" {
		t.Fatalf("normalized = %+v", got)
	}
	out.Streams[0].Detection.Zones[0].Points[0][0] = 0.5
	if in.Streams[0].Detection.Zones[0].Points[0][0] != 0 {
		t.Fatal("normalized points share memory with the input")
	}
}
