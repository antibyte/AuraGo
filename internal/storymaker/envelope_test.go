package storymaker

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// envelopeOfRuns builds an envelope whose windows inside the runs (ms) hold level 170 (-20 dBFS) and 0 elsewhere.
func envelopeOfRuns(durationMS int, runs ...[2]int) Envelope {
	levels := make(Levels, (durationMS+9)/10)
	for _, r := range runs {
		for i := r[0] / 10; i < r[1]/10 && i < len(levels); i++ {
			levels[i] = 170
		}
	}
	return Envelope{V: 1, DurationMS: durationMS, Rate: 100, Levels: levels}
}

func TestComputeEnvelopeLevelsAndRuns(t *testing.T) {
	// 1 s at 24 kHz: 0.5 s of silence, then a constant 0.1, a mean power of 0.01, or -20 dBFS.
	// The level byte is round((-20+60)*255/60) = 170. Window 50 starts at floor(50*24000/100) = 12000.
	samples := make([]float32, 24000)
	for i := 12000; i < 24000; i++ {
		samples[i] = 0.1
	}
	env := ComputeEnvelope(DecodedAudio{Rate: 24000, Channels: 1, Samples: samples})
	if env.V != 1 || env.Rate != 100 || env.DurationMS != 1000 || len(env.Levels) != 100 {
		t.Fatalf("v %d rate %d duration %d levels %d", env.V, env.Rate, env.DurationMS, len(env.Levels))
	}
	if env.Levels[0] != 0 || env.Levels[49] != 0 || env.Levels[50] != 170 || env.Levels[99] != 170 {
		t.Errorf("levels %v %v %v %v", env.Levels[0], env.Levels[49], env.Levels[50], env.Levels[99])
	}
	if runs := env.SpeechRuns(); !reflect.DeepEqual(runs, [][2]int{{500, 1000}}) {
		t.Errorf("runs %v", runs)
	}
}

func TestComputeEnvelopeWindowCountFollowsTheFloorRule(t *testing.T) {
	// 20 s at 22050 Hz is 441000 frames. Window i starts at floor(i*22050/100), so the count is
	// ceil(441000*100/22050) = 2000. A fixed window of floor(22050*0.01) = 220 samples would give ceil(441000/220) = 2005.
	env := ComputeEnvelope(DecodedAudio{Rate: 22050, Channels: 1, Samples: make([]float32, 441000)})
	if len(env.Levels) != 2000 || env.DurationMS != 20000 {
		t.Fatalf("levels %d, duration %d", len(env.Levels), env.DurationMS)
	}
}

func TestSpeechRunsBridgeGapsUnder100ms(t *testing.T) {
	if got := envelopeOfRuns(600, [2]int{0, 200}, [2]int{290, 500}).SpeechRuns(); !reflect.DeepEqual(got, [][2]int{{0, 500}}) {
		t.Errorf("a 90 ms gap: %v", got)
	}
	if got := envelopeOfRuns(600, [2]int{0, 200}, [2]int{310, 500}).SpeechRuns(); !reflect.DeepEqual(got, [][2]int{{0, 200}, {310, 500}}) {
		t.Errorf("a 110 ms gap: %v", got)
	}
}

func TestAssembleBlockTimesAndLevels(t *testing.T) {
	block := NarrationBlock{ID: "s1:n", Kind: BlockKindNarration, Beat: -1, Speaker: "narrator"}
	pieces := []AudioPieceInput{
		{Hash: "a", Ext: "wav", Chars: 10, Join: JoinSentence, Env: envelopeOfRuns(1000, [2]int{100, 800})},
		{Hash: "b", Ext: "mp3", Chars: 20, Env: envelopeOfRuns(500, [2]int{50, 400})},
	}
	got := AssembleBlock(block, pieces, 30)

	// Piece a: in 100-40 = 60, out 800+100 = 900, used 840, at 0.
	// Piece b: in 50-40 = 10, out min(500, 400+100) = 500, used 490, at 840+350 = 1190. The block lasts 1190+490 = 1680.
	wantPieces := []AudioPiece{
		{Hash: "a", Ext: "wav", AtMS: 0, InMS: 60, OutMS: 900, Chars: 10},
		{Hash: "b", Ext: "mp3", AtMS: 1190, InMS: 10, OutMS: 500, Chars: 20},
	}
	if !reflect.DeepEqual(got.Pieces, wantPieces) {
		t.Errorf("pieces %+v", got.Pieces)
	}
	if got.ID != "s1:n" || got.Kind != BlockKindNarration || got.Speaker != "narrator" || got.Beat != nil || !got.SpeechOK || got.DurationMS != 1680 {
		t.Errorf("block %+v", got)
	}
	// Runs shifted to block time: a 100-60 = 40, 800-60 = 740; b 1190+40 = 1230, 1190+390 = 1580.
	if !reflect.DeepEqual(got.Spans, [][2]int{{40, 740}, {1230, 1580}}) {
		t.Errorf("spans %v", got.Spans)
	}
	// ceil(1680*30/1000) = ceil(50.4) = 51 frames.
	if len(got.Levels) != 51 {
		t.Fatalf("%d levels", len(got.Levels))
	}
	// Frame 10: bins 33..35 are block 330..350 ms, file 390..410 ms, all speech.
	if got.Levels[10] != 170 {
		t.Errorf("frame 10 is %d", got.Levels[10])
	}
	// Frame 22: bins 73..75 are file 790, 800, 810 ms, so one speech bin and two silent ones.
	// Mean power 0.01/3 is -24.77 dBFS, and (60-24.77)*255/60 = 149.7 rounds to 150. A mean of the bytes would give 57.
	if got.Levels[22] != 150 {
		t.Errorf("frame 22 is %d", got.Levels[22])
	}
	// Frame 30: bins 100..102 are block 1000..1020 ms, inside the pause from 840 to 1190.
	if got.Levels[30] != 0 {
		t.Errorf("frame 30 is %d", got.Levels[30])
	}
	// Frame 40: bins 133..135 are block 1330..1350 ms, file 10+140 = 150 ms and on, inside the run of piece b.
	if got.Levels[40] != 170 {
		t.Errorf("frame 40 is %d", got.Levels[40])
	}

	pieces[0].Join = JoinSplit
	if at := AssembleBlock(block, pieces, 30).Pieces[1].AtMS; at != 990 { // 840+150
		t.Errorf("a split join puts piece b at %d", at)
	}
}

func TestAssembleBlockSilentPiece(t *testing.T) {
	block := NarrationBlock{ID: "s1:b2", Kind: BlockKindDialogue, Beat: 2, Speaker: "anna"}
	got := AssembleBlock(block, []AudioPieceInput{{Hash: "q", Ext: "wav", Chars: 5, Env: envelopeOfRuns(300)}}, 30)
	if got.SpeechOK || got.Pieces[0].InMS != 0 || got.Pieces[0].OutMS != 300 || got.DurationMS != 300 {
		t.Fatalf("block %+v", got)
	}
	// 300*30/1000 = 9 frames exactly.
	if len(got.Levels) != 9 {
		t.Fatalf("%d levels", len(got.Levels))
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"beat":2`, `"spans":[]`, `"speech_ok":false`, `"levels":[0,0,0,0,0,0,0,0,0]`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("%s has no %s", data, want)
		}
	}
}

func TestLevelsMarshalAsAnArray(t *testing.T) {
	data, err := json.Marshal(Levels{0, 85, 255})
	if err != nil || string(data) != "[0,85,255]" {
		t.Errorf("got %s, err %v", data, err)
	}
	empty, err := json.Marshal(Levels(nil))
	if err != nil || string(empty) != "[]" {
		t.Errorf("nil gave %s, err %v", empty, err)
	}
	var back Levels
	if err := json.Unmarshal([]byte("[1,2]"), &back); err != nil || !reflect.DeepEqual(back, Levels{1, 2}) {
		t.Errorf("got %v, err %v", back, err)
	}
	for _, bad := range []string{"[256]", "[-1]", `"AQI="`} {
		if err := json.Unmarshal([]byte(bad), &back); err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
}

func TestParseEnvelope(t *testing.T) {
	// 12345 frames at 22050 Hz: 559.86 ms rounds to 560, windows ceil(55.986) = 56, and ceil(560/10) = 56.
	computed := ComputeEnvelope(DecodedAudio{Rate: 22050, Channels: 2, Samples: make([]float32, 2*12345)})
	data, err := json.Marshal(computed)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseEnvelope(data)
	if err != nil || parsed.DurationMS != 560 || len(parsed.Levels) != 56 {
		t.Fatalf("parsed %+v, err %v", parsed, err)
	}

	levels := func(n int) string { return "[" + strings.TrimSuffix(strings.Repeat("0,", n), ",") + "]" }
	envelope := func(n int) string { return `{"v":1,"duration_ms":1000,"rate":100,"levels":` + levels(n) + `}` }
	// ceil(1000/10) = 100, so 99, 100 and 101 levels pass.
	for _, n := range []int{99, 100, 101} {
		if _, err := ParseEnvelope([]byte(envelope(n))); err != nil {
			t.Errorf("%d levels: %v", n, err)
		}
	}
	for name, bad := range map[string]string{
		"too few levels":  envelope(98),
		"too many levels": envelope(102),
		"huge duration":   `{"v":1,"duration_ms":2000000000,"rate":100,"levels":[0]}`,
		"above 600 s":     `{"v":1,"duration_ms":600001,"rate":100,"levels":` + levels(60001) + `}`,
		"zero duration":   `{"v":1,"duration_ms":0,"rate":100,"levels":[]}`,
		"wrong version":   `{"v":2,"duration_ms":1000,"rate":100,"levels":` + levels(100) + `}`,
		"wrong rate":      `{"v":1,"duration_ms":1000,"rate":50,"levels":` + levels(100) + `}`,
		"level above 255": `{"v":1,"duration_ms":10,"rate":100,"levels":[300]}`,
		"not json":        `{`,
	} {
		if _, err := ParseEnvelope([]byte(bad)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}
