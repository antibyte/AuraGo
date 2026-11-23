package chibisensei

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readOutputFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "outputs", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseStatus(t *testing.T) {
	fx := readOutputFixture(t, "chibi-status.json")
	s, err := ParseStatus(fx)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Succeeded() || s.GPU.Capability != "7.5" || s.Train.EvalLossTuned == nil || *s.Train.EvalLossTuned != 1.12 ||
		s.Train.TruncatedRows != 2 || s.WarningCode != "base_gguf_skipped" || s.Discovery["converter_env"] != "system" {
		t.Errorf("status %+v", s)
	}
	replace := func(old, new string) []byte {
		if !bytes.Contains(fx, []byte(old)) {
			t.Fatalf("fixture lacks %q", old)
		}
		return bytes.Replace(fx, []byte(old), []byte(new), 1)
	}
	if s, err := ParseStatus(replace(`"capability":"7.5"`, `"capability":7.5`)); err != nil || s.GPU.Capability != "7.5" {
		t.Errorf("numeric capability: %v %q", err, s.GPU.Capability)
	}
	if s, err := ParseStatus(replace(`"train_loss":0.92`, `"train_loss":null`)); err != nil || s.Train.TrainLoss != nil {
		t.Errorf("null train_loss: %v", err)
	}
	if s, err := ParseStatus(replace(`"message":""`, `"message":"`+strings.Repeat("ä", 3000)+`"`)); err != nil || len([]rune(s.Message)) != 2000 {
		t.Errorf("message cut: %v %d", err, len([]rune(s.Message)))
	}
	rejects := map[string][]byte{
		"version":     replace(`"v":1`, `"v":2`),
		"state":       replace(`"succeeded"`, `"running"`),
		"stage":       replace(`"stage":"manifest"`, `"stage":"compile"`),
		"code":        replace(`"code":""`, `"code":"Bad-Code"`),
		"stop reason": replace(`"stop_reason":"done"`, `"stop_reason":"later"`),
		"run id":      replace(`"run_id":"0123abcd-4567-4890-abcd-ef0123456789"`, `"run_id":"x"`),
		"too large":   append(append([]byte{}, fx...), bytes.Repeat([]byte(" "), 1<<20)...),
	}
	for name, b := range rejects {
		if _, err := ParseStatus(b); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseManifest(t *testing.T) {
	fx := readOutputFixture(t, "chibi-manifest.json")
	m, err := ParseManifest(fx)
	if err != nil || len(m.Files) != 3 {
		t.Fatalf("fixture: %v %+v", err, m)
	}
	const (
		gguf    = `{"path":"chibi-model-q8_0.gguf","kind":"gguf_tuned","bytes":291234567,"sha256":"1111111111111111111111111111111111111111111111111111111111111111"}`
		runID   = `"run_id":"0123abcd-4567-4890-abcd-ef0123456789"`
		bigSize = `"bytes":291234567`
	)
	mut := func(old, new string) []byte {
		if !bytes.Contains(fx, []byte(old)) {
			t.Fatalf("fixture lacks %q", old)
		}
		return bytes.Replace(fx, []byte(old), []byte(new), 1)
	}
	rejects := map[string][]byte{
		"path dotdot":    mut(`"path":"chibi-model-q8_0.gguf"`, `"path":"../chibi-x"`),
		"path slash":     mut(`"path":"chibi-model-q8_0.gguf"`, `"path":"chibi-a/b"`),
		"path prefix":    mut(`"path":"chibi-model-q8_0.gguf"`, `"path":"model.gguf"`),
		"path colon":     mut(`"path":"chibi-model-q8_0.gguf"`, `"path":"chibi-x:y"`),
		"kind unknown":   mut(`"kind":"gguf_tuned"`, `"kind":"weights"`),
		"kind vs path":   mut(`"kind":"metrics"`, `"kind":"gguf_base"`),
		"listed twice":   mut(`"files":[`, `"files":[`+gguf+`,`),
		"sha":            mut(`"sha256":"1111111111111111111111111111111111111111111111111111111111111111"`, `"sha256":"ABC"`),
		"negative bytes": mut(bigSize, `"bytes":-1`),
		"huge bytes":     mut(bigSize, fmt.Sprintf(`"bytes":%d`, int64(9)<<30)),
		"version":        mut(`"v":1`, `"v":2`),
		"run id":         mut(runID, `"run_id":"x"`),
		"no files":       []byte(`{"v":1,` + runID + `,"profile_id":"gemma3-270m","profile_sha":"` + strings.Repeat("a", 64) + `","files":[]}`),
	}
	for name, b := range rejects {
		if _, err := ParseManifest(b); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	for p, want := range map[string]bool{"chibi-model-q8_0.gguf": true, "..": false, "chibi-": false, " chibi-x": false, "chibi-..x": false} {
		if ValidManifestPath(p) != want {
			t.Errorf("ValidManifestPath(%q) != %v", p, want)
		}
	}
}

func TestParseMetricsFile(t *testing.T) {
	ms, err := ParseMetricsFile(bytes.NewReader(readOutputFixture(t, "chibi-metrics.jsonl")))
	if err != nil {
		t.Fatal(err)
	}
	type pt struct {
		step int
		kind string
		loss float64
	}
	want := []pt{{10, "train", 2.5}, {20, "eval", 2.0}, {20, "train", 2.2}, {40, "train", 1.9}}
	if len(ms) != len(want) {
		t.Fatalf("points %+v", ms)
	}
	for i, w := range want {
		if ms[i].Step != w.step || ms[i].Kind != w.kind || ms[i].Loss != w.loss {
			t.Errorf("point %d: %+v, want %+v", i, ms[i], w)
		}
	}
	long := strings.Repeat("a", 5000) + "\n" + `{"step":1,"loss":1.5}` + "\n"
	if ms, _ := ParseMetricsFile(strings.NewReader(long)); len(ms) != 1 || ms[0].Step != 1 {
		t.Errorf("long line: %+v", ms)
	}
	var many strings.Builder
	for i := 0; i < 20005; i++ {
		fmt.Fprintf(&many, "{\"step\":%d,\"loss\":1}\n", i)
	}
	if ms, _ := ParseMetricsFile(strings.NewReader(many.String())); len(ms) != 20000 {
		t.Errorf("cap: %d points", len(ms))
	}
}

func TestParseEvalSamples(t *testing.T) {
	fx := readOutputFixture(t, "chibi-eval_samples.jsonl")
	ss, err := ParseEvalSamples(bytes.NewReader(fx), 0)
	if err != nil || len(ss) != 2 {
		t.Fatalf("%v %+v", err, ss)
	}
	if ss[0].ID != "0" || ss[1].ID != "1" || len(ss[0].PromptMessages) != 2 || ss[1].PromptText == "" {
		t.Errorf("samples %+v", ss)
	}
	if ss, _ := ParseEvalSamples(bytes.NewReader(fx), 1); len(ss) != 1 {
		t.Errorf("limit: %d", len(ss))
	}
	big := `{"id":0,"prompt_text":"p","expected":"e","base_output":"b","tuned_output":"` + strings.Repeat("x", 9000) + `"}`
	if ss, _ := ParseEvalSamples(strings.NewReader(big), 0); len(ss) != 1 || ss[0].TunedOutput == nil ||
		len([]rune(*ss[0].TunedOutput)) != 8000 {
		t.Errorf("cut: %+v", len(ss))
	}
	// An answer the trainer did not generate is null and stays nil; an empty answer stays an empty string.
	missing := `{"id":"eval-1","prompt_text":"p","expected":"e","base_output":null,"tuned_output":"","base_ms":null,"tuned_ms":-3}`
	ss, err = ParseEvalSamples(strings.NewReader(missing), 0)
	if err != nil || len(ss) != 1 {
		t.Fatalf("missing answers: %v %d", err, len(ss))
	}
	if s := ss[0]; s.ID != "eval-1" || s.BaseOutput != nil || s.BaseMS != nil || s.TunedOutput == nil || *s.TunedOutput != "" ||
		s.TunedMS == nil || *s.TunedMS != 0 {
		t.Errorf("missing answers: %+v", s)
	}
}
