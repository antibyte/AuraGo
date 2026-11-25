package chibisensei

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func loadLogFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "logs", name))
	if err != nil {
		t.Fatal(err)
	}
	return bytes.ReplaceAll(b, []byte("{{PAD4200}}"), []byte(strings.Repeat("a", 4200)))
}

func seqsOf(ev []LogEvent) []int64 {
	out := make([]int64, len(ev))
	for i, e := range ev {
		out[i] = e.Seq
	}
	return out
}

func sameSeqs(got []LogEvent, want ...int64) bool {
	g := seqsOf(got)
	if len(g) != len(want) {
		return false
	}
	for i := range g {
		if g[i] != want[i] {
			return false
		}
	}
	return true
}

func TestParseKernelLogPlainText(t *testing.T) {
	ev := ParseKernelLog(loadLogFixture(t, "kernel-log.txt"))
	if !sameSeqs(ev, 1, 2, 3, 7, 8, ExitSeq) {
		t.Fatalf("seqs %v", seqsOf(ev))
	}
	want := Metric{Step: 10, MaxSteps: 40, Kind: "train", Epoch: 0.25, Loss: 2.5, LR: 0.0002, GradNorm: 1.5, TokS: 2400,
		ElapsedS: 30, EtaS: 90, VRAMGB: 6.1, TS: "2026-11-23T12:00:03Z"}
	if m, ok := ev[2].Metric(); !ok || m != want {
		t.Errorf("metric %+v %v", m, ok)
	}
	if m, ok := ev[4].Metric(); !ok || m.Kind != "eval" || m.Step != 20 || m.Loss != 2.1 {
		t.Errorf("eval metric %+v %v", m, ok)
	}
	if e, ok := ev[3].Error(); !ok || e != (ErrorEvent{Stage: "train", Code: "oom", Message: "CUDA out of memory"}) {
		t.Errorf("error %+v %v", e, ok)
	}
	if s, ok := ev[0].Stage(); !ok || s != (StageEvent{Stage: "boot", State: "start"}) {
		t.Errorf("stage %+v %v", s, ok)
	}
	if c, ok := ev[5].ExitCode(); !ok || c != 0 {
		t.Errorf("exit %d %v", c, ok)
	}
	if _, ok := ev[0].Metric(); ok {
		t.Error("a stage event must not decode as a metric")
	}
}

func TestParseKernelLogJSONArray(t *testing.T) {
	ev := ParseKernelLog(loadLogFixture(t, "kernel-log.json"))
	if !sameSeqs(ev, 1, 2, 3, ExitSeq) {
		t.Fatalf("seqs %v", seqsOf(ev))
	}
	if s, ok := ev[1].Stage(); !ok || s != (StageEvent{Stage: "boot", State: "done"}) {
		t.Errorf("reassembled stage %+v %v", s, ok)
	}
	if m, ok := ev[2].Metric(); !ok || m.Loss != 2.5 {
		t.Errorf("first seq 3 must win: %+v", m)
	}
}

func TestParseKernelLogTailCut(t *testing.T) {
	body := loadLogFixture(t, "kernel-log.json")
	key := []byte(`{"stream_name"`)
	off, n := 0, 0
	for ; n < 3; n++ {
		i := bytes.Index(body[off:], key)
		if i < 0 {
			t.Fatal("fixture too short")
		}
		off += i + len(key)
	}
	cut := body[off-len(key)-3:]
	if ev := ParseKernelLog(cut); !sameSeqs(ev, 2, 3, ExitSeq) {
		t.Fatalf("seqs %v", seqsOf(ev))
	}
}

func TestParseKernelLogLines(t *testing.T) {
	const head = `{"v":1,"seq":1,"t":"info","pad":"`
	padded := func(total int) string {
		return "@@chibi " + head + strings.Repeat("a", total-8-len(head)-2) + `"}`
	}
	cases := []struct {
		name string
		body string
		want int
	}{
		{"crlf", "@@chibi {\"v\":1,\"seq\":1,\"t\":\"info\"}\r\n@@chibi {\"v\":1,\"seq\":2,\"t\":\"info\"}\r\n", 2},
		{"4096 bytes", padded(4096), 1},
		{"4097 bytes", padded(4097), 0},
		{"negative seq", `@@chibi {"v":1,"seq":-1,"t":"info"}`, 0},
		{"no seq", `@@chibi {"v":1,"t":"info"}`, 0},
		{"trailing garbage", `@@chibi {"v":1,"seq":1,"t":"info"} x`, 0},
		{"empty", "", 0},
	}
	for _, tc := range cases {
		ev := ParseKernelLog([]byte(tc.body))
		if ev == nil || len(ev) != tc.want {
			t.Errorf("%s: got %d events (nil=%v), want %d", tc.name, len(ev), ev == nil, tc.want)
		}
	}
	ev := ParseKernelLog([]byte(`@@chibi {"v":1,"seq":1,"t":"stage","stage":"compile","state":"start"}`))
	if len(ev) != 1 {
		t.Fatalf("unknown stage event dropped: %d", len(ev))
	}
	if _, ok := ev[0].Stage(); ok {
		t.Error("stage compile accepted")
	}
	ev = ParseKernelLog([]byte(`@@chibi {"v":1,"seq":1,"t":"error","code":"Bad Code","message":"x"}`))
	if len(ev) != 1 {
		t.Fatal("error event dropped")
	}
	if _, ok := ev[0].Error(); ok {
		t.Error("error code with a space accepted")
	}
}

func chibiLine(seq int) string {
	return fmt.Sprintf(`@@chibi {"v":1,"seq":%d,"ts":"2026-11-23T12:00:00Z","t":"info"}`, seq)
}

func TestParseKernelLogPlainTextWithEmbeddedStreamObject(t *testing.T) {
	body := chibiLine(1) + "\n" +
		`{"stream_name":"stdout","time":1.5,"data":"` + chibiLine(2) + `\n"}` + "\n" +
		chibiLine(3) + "\n"
	ev := ParseKernelLog([]byte(body))
	if !sameSeqs(ev, 1, 2, 3) && !sameSeqs(ev, 1, 3) {
		t.Fatalf("seqs %v", seqsOf(ev))
	}
	if len(ev) < 2 || ev[0].Seq != 1 || ev[len(ev)-1].Seq != 3 {
		t.Fatalf("the plain lines were lost: %v", seqsOf(ev))
	}
}

func TestParseKernelLogStreamKeyFloodIsLinear(t *testing.T) {
	body := []byte(strings.Repeat(`{"stream_name":`, (4<<20)/len(`{"stream_name":`)))
	start := time.Now()
	ev := ParseKernelLog(body)
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("parse took %v", d)
	}
	if len(ev) != 0 {
		t.Fatalf("events %v", seqsOf(ev))
	}
	if len(logTailText(body)) == 0 {
		t.Error("log tail empty")
	}
}

func TestParseKernelLogBlankLines(t *testing.T) {
	body := []byte(strings.Repeat("\n", 4<<20) + chibiLine(5) + "\n")
	if ev := ParseKernelLog(body); !sameSeqs(ev, 5) {
		t.Fatalf("seqs %v", seqsOf(ev))
	}
}

type streamEntry struct {
	Stream string  `json:"stream_name"`
	Time   float64 `json:"time"`
	Data   string  `json:"data"`
}

// streamTestEntries are log entries with stage and metric lines, one stdout line split across two entries and one
// stderr entry in between.
func streamTestEntries() []streamEntry {
	split := fakeLogLine(3, "info", map[string]any{"msg": "split across entries"})
	cut := len(split) / 2
	return []streamEntry{
		{"stdout", 1.5, fakeLogLine(1, "stage", map[string]any{"stage": "boot", "state": "start"}) + "\n"},
		{"stdout", 2.5, split[:cut]},
		{"stderr", 2.6, "warning: something on stderr\n"},
		{"stdout", 2.7, split[cut:] + "\n"},
		{"stdout", 3.5, fakeLogLine(4, "metric", map[string]any{"kind": "train", "step": 10, "max_steps": 40, "loss": 2.5}) + "\n"},
	}
}

// eventStreamBody renders entries as the answer of a running session: one "data: {entry}" event per entry, each
// followed by an empty line.
func eventStreamBody(entries []streamEntry) []byte {
	var b bytes.Buffer
	for _, e := range entries {
		j, _ := json.Marshal(e)
		b.WriteString("data: " + string(j) + "\n\n")
	}
	return b.Bytes()
}

func TestParseKernelLogEventStream(t *testing.T) {
	entries := streamTestEntries()
	arrayForm, _ := json.Marshal(entries)
	want := ParseKernelLog(arrayForm)
	if !sameSeqs(want, 1, 3, 4) {
		t.Fatalf("array form seqs %v", seqsOf(want))
	}
	got := ParseKernelLog(eventStreamBody(entries))
	if !sameSeqs(got, 1, 3, 4) {
		t.Fatalf("event stream seqs %v", seqsOf(got))
	}
	for i := range want {
		if got[i].Seq != want[i].Seq || got[i].T != want[i].T || !bytes.Equal(got[i].Raw, want[i].Raw) {
			t.Errorf("event %d differs: %+v vs %+v", i, got[i], want[i])
		}
	}
	if m, ok := got[2].Metric(); !ok || m.Step != 10 || m.Loss != 2.5 {
		t.Errorf("metric %+v %v", m, ok)
	}

	// A stream cut in the middle of the last event keeps the complete ones.
	full := eventStreamBody(entries)
	if ev := ParseKernelLog(full[:len(full)-40]); !sameSeqs(ev, 1, 3) {
		t.Fatalf("cut stream seqs %v", seqsOf(ev))
	}
}
