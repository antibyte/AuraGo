package render

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/storymakertest"
)

// mixTestStory has two scenes with one sentence, so both narration blocks share one piece hash.
func mixTestStory() *storymaker.Story {
	const sentence = "Das ist ein ganzer Satz zur Firewall."
	return &storymaker.Story{
		Schema: storymaker.Schema, Title: "Mix",
		Format: storymaker.Format{Aspect: "16:9", FPS: 30},
		Voice:  &storymaker.Voice{Provider: "phonon", Voice: "thorsten", Language: "de"},
		Scenes: []storymaker.Scene{{ID: "s1", Narration: sentence}, {ID: "s2", Narration: sentence}},
	}
}

// seededMixRun seeds 2 s pieces and returns a run whose report places them at 500 ms and 3000 ms
// of a 5 s film (150 frames at 30 fps).
func seededMixRun(t *testing.T, fx *runFixture, story *storymaker.Story) (*renderRun, storymaker.AudioBlock) {
	t.Helper()
	ctx := context.Background()
	if err := storymakertest.SeedNarration(ctx, fx.projects, fx.projectID, story, 2); err != nil {
		t.Fatal(err)
	}
	answer, err := fx.projects.AudioAnswer(ctx, fx.projectID, story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Missing) != 0 {
		t.Fatalf("missing after the seed: %+v", answer.Missing)
	}
	block := answer.Narration["s1"].Blocks[0]
	hash := block.Pieces[0].Hash
	rep := compileReport{FrameCount: 150, AudioEvents: []reportAudioEvent{
		{Kind: "narration", Ref: hash, StartMS: 500, SceneID: "s1", BlockID: "s1:n"},
		{Kind: "narration", Ref: hash, StartMS: 3000, SceneID: "s2", BlockID: "s2:n"},
	}}
	plan := &Plan{ProjectID: fx.projectID, Story: *story, Blocks: storymaker.Blocks(story), AudioInfo: answer.Info()}
	return &renderRun{r: fx.runner, job: Job{ID: "mix-job"}, plan: plan, st: fx.settings, fps: 30, report: rep}, block
}

func TestDecodeArgsAreExact(t *testing.T) {
	got := decodeArgs("SRC.mp3", "DST.s16", 3.5)
	// 3.5 s at 192000 bytes per second is 672000 bytes, and times 1.1 that is 739200.
	want := []string{"-hide_banner", "-nostdin", "-loglevel", "error",
		"-protocol_whitelist", "file", "-format_whitelist", "wav,mp3", "-i", "SRC.mp3",
		"-map", "0:a:0", "-ar", "48000", "-ac", "2", "-f", "s16le",
		"-t", "3.500", "-fs", "739200", "-y", "DST.s16"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args =\n%v\nwant\n%v", got, want)
	}
}

func TestEstimateMixBytes(t *testing.T) {
	// input 100 + decoded 1000 + 48000 frames * 4 bytes = 193100.
	if got := EstimateMixBytes(1000, 100, 48000, false); got != 193100 {
		t.Errorf("without stems = %d", got)
	}
	// Stems add 48000 * 4 * 3 * 2 = 1152000.
	if got := EstimateMixBytes(1000, 100, 48000, true); got != 1345100 {
		t.Errorf("with stems = %d", got)
	}
}

func TestHasAudio(t *testing.T) {
	measured := storymaker.AudioInfo{Narration: map[string]storymaker.SceneAudio{"s1": {Blocks: []storymaker.AudioBlock{{ID: "s1:n"}}}}}
	cases := []struct {
		name string
		plan *Plan
		rep  compileReport
		want bool
	}{
		{"nothing", &Plan{}, compileReport{}, false},
		{"a measured block", &Plan{AudioInfo: measured}, compileReport{}, true},
		{"music", &Plan{Music: &storymaker.MusicRef{Key: "k"}}, compileReport{}, true},
		{"an sfx event", &Plan{}, compileReport{AudioEvents: []reportAudioEvent{{Kind: "sfx", Ref: "sfx.pop"}}}, true},
		{"a narration event alone decides nothing", &Plan{}, compileReport{AudioEvents: []reportAudioEvent{{Kind: "narration", Ref: "x"}}}, false},
	}
	for _, tc := range cases {
		if got := HasAudio(tc.plan, tc.rep); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}

func TestBuildMixInputDecodesEachHashOnceWithItsLimits(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	story := mixTestStory()
	story.Music = &storymaker.Music{Mode: "generated", Mood: "calm-ambient"}
	run, block := seededMixRun(t, fx, story)
	key := storymaker.MusicKeyGenerated("calm-ambient", "")
	if err := storymakertest.SeedMusic(ctx, fx.projects, fx.projectID, key, 3); err != nil {
		t.Fatal(err)
	}
	run.plan.Music = &storymaker.MusicRef{Key: key, Ext: "wav", DurationMS: 3000}

	var calls []string
	orig := decodeCommand
	decodeCommand = func(ctx context.Context, ffmpeg string, args []string) error {
		calls = append(calls, strings.Join(args, " "))
		return orig(ctx, ffmpeg, args)
	}
	t.Cleanup(func() { decodeCommand = orig })

	plan, err := run.planMixInput(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	in, err := run.buildMixInput(ctx, plan, dir)
	if err != nil {
		t.Fatal(err)
	}

	// Both scenes hold one hash: one decode for the piece, one for the music.
	if len(calls) != 2 {
		t.Fatalf("decodes = %d: %v", len(calls), calls)
	}
	piece := block.Pieces[0]
	// out_ms/1000+1 seconds at 192000 bytes per second, times 1.1.
	maxPiece := float64(piece.OutMS)/1000 + 1
	wantPiece := "-t " + strconv.FormatFloat(maxPiece, 'f', 3, 64) + " -fs " + strconv.FormatInt(int64(maxPiece*192000)*11/10, 10)
	if !strings.Contains(calls[0], wantPiece) {
		t.Errorf("piece decode lacks %q: %s", wantPiece, calls[0])
	}
	// The film is 150/30 = 5 s, so music gets 7 s: 7*192000 = 1344000, times 1.1 = 1478400.
	if !strings.Contains(calls[1], "-t 7.000 -fs 1478400") {
		t.Errorf("music decode: %s", calls[1])
	}
	for _, c := range calls {
		if !strings.Contains(c, "-protocol_whitelist file -format_whitelist wav,mp3") {
			t.Errorf("decode without whitelists: %s", c)
		}
	}

	// 150 frames * 48000 / 30.
	if in.Frames != 240000 {
		t.Errorf("frames = %d", in.Frames)
	}
	if len(in.Narration) != 2 {
		t.Fatalf("narration clips = %+v", in.Narration)
	}
	for i, startMS := range []int64{500, 3000} {
		c := in.Narration[i]
		if c.Start != startMS*48 || c.In != int64(piece.InMS)*48 || c.Voice != "phonon|thorsten" {
			t.Errorf("clip %d = %+v", i, c)
		}
		// The decode may end up to 10 ms early, which is 480 frames.
		wantLen := int64(piece.OutMS-piece.InMS) * 48
		if c.Len > wantLen || wantLen-c.Len > 480 {
			t.Errorf("clip %d: len %d, want about %d", i, c.Len, wantLen)
		}
		// The piece is trimmed at its start, because in_ms is 60. 5 ms at 48 frames per ms is 240.
		if c.Fade != 240 {
			t.Errorf("clip %d: fade %d", i, c.Fade)
		}
		if filepath.Dir(c.Path) != filepath.Join(dir, "dec") {
			t.Errorf("clip %d: path %s", i, c.Path)
		}
		if info, err := os.Stat(c.Path); err != nil || info.Size() == 0 || info.Size()%4 != 0 {
			t.Errorf("clip %d: raw file %v, %v", i, info, err)
		}
	}
	span := block.Spans[0]
	if len(in.Spans) != 2 || in.Spans[0].From != (500+int64(span[0]))*48 || in.Spans[1].From != (3000+int64(span[0]))*48 {
		t.Errorf("spans = %+v, block span %v", in.Spans, span)
	}
	// 3 s of music at 48000 frames per second, within 10 ms.
	if in.Bed == nil || in.Bed.Volume != 0.25 || math.Abs(float64(in.Bed.Frames-144000)) > 480 {
		t.Errorf("bed = %+v", in.Bed)
	}
}

func TestBuildMixInputReadsTheTakeOfTheEvent(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	entry, ok := fx.deps.Catalog.Lookup("sfx.pop")
	if !ok || len(entry.Takes) < 3 || entry.Hit == nil {
		t.Fatalf("catalog entry = %+v", entry)
	}
	take := entry.Takes[2]
	rep := compileReport{FrameCount: 150, AudioEvents: []reportAudioEvent{{Kind: "sfx", Ref: "sfx.pop", StartMS: 1000, SceneID: "s1", Take: 2}}}
	story := mixTestStory()
	run := &renderRun{r: fx.runner, job: Job{ID: "mix-job"}, plan: &Plan{ProjectID: fx.projectID, Story: *story}, st: fx.settings, fps: 30, report: rep}
	plan, err := run.planMixInput(ctx)
	if err != nil {
		t.Fatal(err)
	}
	in, err := run.buildMixInput(ctx, plan, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(in.SFX) != 1 {
		t.Fatalf("sfx = %+v", in.SFX)
	}
	c := in.SFX[0]
	if c.Key != take.File || c.Start != 48000 || c.LevelOffsetDB != entry.Hit.OverDB-take.L50LUFS {
		t.Errorf("clip key %q start %d offset %v, take %+v", c.Key, c.Start, c.LevelOffsetDB, take)
	}
	// Stereo from a mono file: the two channels of a frame are equal.
	if c.Len == 0 || int64(len(c.PCM)) != c.Len*2 || c.PCM[0] != c.PCM[1] {
		t.Errorf("clip len %d, pcm %d", c.Len, len(c.PCM))
	}
	if math.Abs(float64(c.Len)-take.DurMS*48) > 240 {
		t.Errorf("clip len %d, take lasts %v ms", c.Len, take.DurMS)
	}
	wantAt := c.Start + int64(math.Round(take.ImpactMS*48))
	if len(in.Impacts) != 1 || in.Impacts[0].At != wantAt || in.Impacts[0].DepthDB != entry.Hit.DipDB {
		t.Errorf("impacts = %+v, want at %d", in.Impacts, wantAt)
	}
}

func TestBuildMixInputRefusesLowDiskBeforeAnyCopy(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	run, _ := seededMixRun(t, fx, mixTestStory())
	orig := freeDiskBytes
	freeDiskBytes = func(string) (int64, error) { return 1 << 20, nil }
	t.Cleanup(func() { freeDiskBytes = orig })
	plan, err := run.planMixInput(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	_, err = run.buildMixInput(ctx, plan, dir)
	var je *JobError
	if !errors.As(err, &je) || je.Code != CodeRenderFailed {
		t.Fatalf("err = %v", err)
	}
	needed := EstimateMixBytes(plan.decodedBytes, plan.inputBytes, plan.filmFrames, plan.stems) + minFreeStagingByte
	wantMB := strconv.FormatInt((needed+(1<<20)-1)>>20, 10) + " MB"
	if !strings.Contains(je.Message, wantMB) {
		t.Errorf("message %q lacks %q", je.Message, wantMB)
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "dec")); err == nil && len(entries) != 0 {
		t.Errorf("dec holds %v", entries)
	}
}

func TestBuildMixInputComparesTheMP3Timelines(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	data, err := os.ReadFile("../testdata/narration/one.mp3")
	if err != nil {
		t.Fatal(err)
	}
	decoded, ext, err := storymaker.DecodeAudio(data)
	if err != nil || ext != "mp3" {
		t.Fatalf("decode: ext %q, %v", ext, err)
	}
	env := storymaker.ComputeEnvelope(decoded)
	goRuns := env.SpeechRuns()
	if len(goRuns) == 0 {
		t.Fatal("one.mp3 has no speech run")
	}
	story := mixTestStory()
	story.Scenes = story.Scenes[:1]
	first := storymaker.Blocks(story)[0]
	hash := storymaker.PieceKey(first.Voice, storymaker.PiecesOf(first)[0].Text)
	if err := fx.projects.WritePiece(ctx, fx.projectID, hash, ext, data, env); err != nil {
		t.Fatal(err)
	}
	answer, err := fx.projects.AudioAnswer(ctx, fx.projectID, story, nil)
	if err != nil {
		t.Fatal(err)
	}
	piece := answer.Narration["s1"].Blocks[0].Pieces[0]
	rep := compileReport{FrameCount: 300, AudioEvents: []reportAudioEvent{{Kind: "narration", Ref: hash, StartMS: 0, SceneID: "s1", BlockID: "s1:n"}}}
	run := &renderRun{r: fx.runner, job: Job{ID: "mix-job"}, plan: &Plan{ProjectID: fx.projectID, Story: *story, Blocks: storymaker.Blocks(story), AudioInfo: answer.Info()}, st: fx.settings, fps: 30, report: rep}
	plan, err := run.planMixInput(ctx)
	if err != nil {
		t.Fatal(err)
	}
	in, err := run.buildMixInput(ctx, plan, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ffStart, found, err := rawFirstSpeechMS(in.Narration[0].Path, goRuns[0][0]+mp3SearchMS)
	if err != nil || !found {
		t.Fatalf("FFmpeg copy: found %v, %v", found, err)
	}
	diff := ffStart - goRuns[0][0]
	t.Logf("first speech: go-mp3 %d ms, FFmpeg %d ms, difference %d ms", goRuns[0][0], ffStart, diff)
	wantIn := piece.InMS
	if diff > mp3ToleranceMS || diff < -mp3ToleranceMS {
		if plan.mp3OffsetMS[hash] != diff {
			t.Errorf("recorded offset %d, measured %d", plan.mp3OffsetMS[hash], diff)
		}
		wantIn = max(0, piece.InMS+diff)
	} else if _, recorded := plan.mp3OffsetMS[hash]; recorded {
		t.Errorf("an offset within the tolerance was recorded: %v", plan.mp3OffsetMS)
	}
	if in.Narration[0].In != int64(wantIn)*48 {
		t.Errorf("clip in = %d, want %d", in.Narration[0].In, int64(wantIn)*48)
	}
}
