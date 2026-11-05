package render

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"aurago/internal/fileutil"
	"aurago/internal/storymaker"
)

const (
	mixBytesPerSecond = 192000 // 48000 frames, 2 channels, 2 bytes each
	mixBytesPerMS     = 192
	mixFramesPerMS    = MixRate / 1000
	mp3ToleranceMS    = 10
	mp3SearchMS       = 2000 // FFmpeg's copy is searched for speech up to this far past the go-mp3 start
)

// freeDiskBytes and decodeCommand are variables so a test can report a full disk and count decodes.
var (
	freeDiskBytes = fileutil.FreeDiskBytes
	decodeCommand = runDecodeCommand
)

// mixPlan describes the inputs of a mix before any file is copied.
type mixPlan struct {
	filmFrames  int64
	filmSeconds float64
	voices      []plannedVoice
	sfx         []plannedSFX
	sources     map[string]*plannedSource // by piece hash
	order       []string                  // hashes in the order of their first event
	music       *plannedSource
	spans       []Span

	inputBytes   int64
	decodedBytes int64
	stems        bool

	// mp3OffsetMS holds FFmpeg's first speech minus go-mp3's for each stored MP3 beyond the tolerance.
	// buildMixInput fills it, because the value needs the decoded file.
	mp3OffsetMS map[string]int
}

// plannedVoice is one narration or dialogue event.
type plannedVoice struct {
	hash    string
	voice   string
	startMS int64
	inMS    int
	outMS   int
}

// plannedSFX is one sfx event with the take the report named.
type plannedSFX struct {
	take    storymaker.SFXTake
	hit     *storymaker.SFXHit
	startMS int64
}

// plannedSource is one stored audio file to stage and decode.
type plannedSource struct {
	folder     string // narration or music
	name       string // piece hash or music key
	ext        string
	size       int64
	maxSeconds float64
	rawPath    string // set by buildMixInput
	rawFrames  int64  // set by buildMixInput
}

// mixStems says whether this job writes stems. Stems are asked for through the render options,
// which do not exist yet, so no job writes them.
func (run *renderRun) mixStems() bool { return false }

func msToMixFrames(ms int64) int64 { return ms * mixFramesPerMS }

// decodeByteCap is the decode limit: maxSeconds of 48 kHz stereo s16, plus 10 percent.
func decodeByteCap(maxSeconds float64) int64 {
	return int64(maxSeconds*mixBytesPerSecond) * 11 / 10
}

// decodeArgs are the FFmpeg arguments of one decode. The limits stop a hand-replaced low-bitrate
// file from decoding to many GiB.
func decodeArgs(src, dst string, maxSeconds float64) []string {
	return []string{"-hide_banner", "-nostdin", "-loglevel", "error",
		"-protocol_whitelist", "file", "-format_whitelist", "wav,mp3", "-i", src,
		"-map", "0:a:0", "-ar", "48000", "-ac", "2", "-f", "s16le",
		"-t", strconv.FormatFloat(maxSeconds, 'f', 3, 64),
		"-fs", strconv.FormatInt(decodeByteCap(maxSeconds), 10),
		"-y", dst}
}

func runDecodeCommand(ctx context.Context, ffmpeg string, args []string) error {
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	stderr := &tailBuffer{limit: ffmpegStderrKeep}
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}
	if msg := strings.TrimSpace(stderr.String()); msg != "" {
		return errors.New(msg)
	}
	return err
}

// decodeToS16 decodes a staged file to raw s16le at 48 kHz stereo. Its error names files by base name only.
func decodeToS16(ctx context.Context, ffmpeg, src, dst string, maxSeconds float64) error {
	err := decodeCommand(ctx, ffmpeg, decodeArgs(src, dst, maxSeconds))
	if err == nil {
		return nil
	}
	msg := strings.ReplaceAll(err.Error(), src, filepath.Base(src))
	msg = strings.ReplaceAll(msg, dst, filepath.Base(dst))
	msg = strings.ReplaceAll(msg, ffmpeg, "FFmpeg")
	return errors.New(clip(msg, 300))
}

// planMixInput computes clip frames and file sizes and copies nothing.
func (run *renderRun) planMixInput(ctx context.Context) (*mixPlan, error) {
	plan := &mixPlan{
		filmFrames:  int64(math.Round(float64(run.report.FrameCount) * MixRate / float64(run.fps))),
		filmSeconds: float64(run.report.FrameCount) / float64(run.fps),
		sources:     map[string]*plannedSource{},
		stems:       run.mixStems(),
		mp3OffsetMS: map[string]int{},
	}
	blocks := map[string]storymaker.AudioBlock{}
	for _, scene := range run.plan.AudioInfo.Narration {
		for _, b := range scene.Blocks {
			blocks[b.ID] = b
		}
	}
	voices := map[string]string{}
	for _, b := range run.plan.Blocks {
		voices[b.ID] = b.Voice.Provider + "|" + b.Voice.Voice
	}
	placed := map[string]bool{}
	for _, ev := range run.report.AudioEvents {
		if ev.Kind == "sfx" {
			entry, ok := run.r.deps.Catalog.Lookup(ev.Ref)
			if !ok || ev.Take < 0 || ev.Take >= len(entry.Takes) {
				return nil, Fail(CodeRenderFailed, "The compile report names an unknown sound %q.", clip(ev.Ref, 80))
			}
			plan.sfx = append(plan.sfx, plannedSFX{take: entry.Takes[ev.Take], hit: entry.Hit, startMS: ev.StartMS})
			continue
		}
		block, ok := blocks[ev.BlockID]
		if !ok || ev.Piece < 0 || ev.Piece >= len(block.Pieces) {
			return nil, Fail(CodeRenderFailed, "The compile report names an unknown piece of block %q.", clip(ev.BlockID, 80))
		}
		piece := block.Pieces[ev.Piece]
		plan.voices = append(plan.voices, plannedVoice{hash: piece.Hash, voice: voices[block.ID], startMS: ev.StartMS, inMS: piece.InMS, outMS: piece.OutMS})
		if !placed[block.ID] {
			placed[block.ID] = true
			base := ev.StartMS - int64(piece.AtMS)
			for _, sp := range block.Spans {
				plan.spans = append(plan.spans, Span{From: msToMixFrames(base + int64(sp[0])), To: msToMixFrames(base + int64(sp[1]))})
			}
		}
		if plan.sources[piece.Hash] != nil {
			continue
		}
		src, err := run.planSource(ctx, plan, "narration", piece.Hash, piece.Ext, float64(piece.OutMS)/1000+1)
		if err != nil {
			return nil, err
		}
		plan.decodedBytes += int64(piece.OutMS+1000) * mixBytesPerMS
		plan.sources[piece.Hash] = src
		plan.order = append(plan.order, piece.Hash)
	}
	if m := run.plan.Music; m != nil {
		maxSeconds := plan.filmSeconds + 2
		src, err := run.planSource(ctx, plan, "music", m.Key, m.Ext, maxSeconds)
		if err != nil {
			return nil, err
		}
		plan.decodedBytes += decodeByteCap(maxSeconds)
		plan.music = src
	}
	return plan, nil
}

func (run *renderRun) planSource(ctx context.Context, plan *mixPlan, folder, name, ext string, maxSeconds float64) (*plannedSource, error) {
	file := name + "." + ext
	r, info, err := run.r.deps.Projects.OpenAudio(ctx, run.plan.ProjectID, folder, file)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, Fail(CodeRenderFailed, "The stored audio file %s is missing. Generate it again.", file)
	}
	_ = r.Close()
	plan.inputBytes += info.Size
	return &plannedSource{folder: folder, name: name, ext: ext, size: info.Size, maxSeconds: maxSeconds}, nil
}

// buildMixInput stages and decodes the planned files below dir/dec and returns the mixer's input.
// dir is the job's staging folder. The disk check runs first, so a refusal leaves no file behind.
func (run *renderRun) buildMixInput(ctx context.Context, plan *mixPlan, dir string) (*MixInput, error) {
	needed := EstimateMixBytes(plan.decodedBytes, plan.inputBytes, plan.filmFrames, plan.stems) + minFreeStagingByte
	if free, err := freeDiskBytes(dir); err == nil && free < needed {
		return nil, Fail(CodeRenderFailed, "The mix needs %d MB of free disk space, %d MB are free.", (needed+(1<<20)-1)>>20, free>>20)
	}
	dec := filepath.Join(dir, "dec")
	if err := os.MkdirAll(dec, 0o700); err != nil {
		return nil, Fail(CodeRenderFailed, "The mix folder could not be created.")
	}
	for _, hash := range plan.order {
		if err := run.decodeSource(ctx, dec, plan.sources[hash], plan); err != nil {
			return nil, err
		}
	}
	if plan.music != nil {
		if err := run.decodeSource(ctx, dec, plan.music, plan); err != nil {
			return nil, err
		}
	}

	in := &MixInput{Frames: plan.filmFrames, Spans: mergeSpans(plan.spans, plan.filmFrames)}
	for _, v := range plan.voices {
		src := plan.sources[v.hash]
		inMS := v.inMS + plan.mp3OffsetMS[v.hash]
		if inMS < 0 {
			inMS = 0
		}
		clipIn := msToMixFrames(int64(inMS))
		clipLen := msToMixFrames(int64(v.outMS - v.inMS))
		if clipIn >= src.rawFrames {
			continue
		}
		if clipIn+clipLen > src.rawFrames {
			clipLen = src.rawFrames - clipIn
		}
		var fade int64
		if clipIn > 0 || clipIn+clipLen < src.rawFrames {
			fade = clipFadeFrames
		}
		in.Narration = append(in.Narration, MixClip{Key: v.hash, Path: src.rawPath, In: clipIn, Len: clipLen, Start: msToMixFrames(v.startMS), Fade: fade, Voice: v.voice})
	}
	pcm := map[string][]int16{}
	for _, e := range plan.sfx {
		samples, ok := pcm[e.take.File]
		if !ok {
			var err error
			if samples, err = run.readSFX(e.take.File); err != nil {
				return nil, err
			}
			pcm[e.take.File] = samples
		}
		over := 0.0
		if e.hit != nil {
			over = e.hit.OverDB
		}
		start := msToMixFrames(e.startMS)
		in.SFX = append(in.SFX, MixClip{Key: e.take.File, PCM: samples, Len: int64(len(samples) / 2), Start: start, LevelOffsetDB: over - e.take.L50LUFS})
		if e.hit != nil && e.hit.DipDB > 0 {
			at := start + int64(math.Round(e.take.ImpactMS*MixRate/1000))
			if at < plan.filmFrames {
				in.Impacts = append(in.Impacts, Impact{At: at, DepthDB: e.hit.DipDB})
			}
		}
	}
	sort.SliceStable(in.Narration, func(i, j int) bool { return in.Narration[i].Start < in.Narration[j].Start })
	sort.SliceStable(in.SFX, func(i, j int) bool { return in.SFX[i].Start < in.SFX[j].Start })
	sort.SliceStable(in.Impacts, func(i, j int) bool { return in.Impacts[i].At < in.Impacts[j].At })
	if plan.music != nil {
		in.Bed = &MixBed{Path: plan.music.rawPath, Frames: plan.music.rawFrames, Volume: storymaker.MusicVolume(run.plan.Story.Music)}
	}
	return in, nil
}

// decodeSource copies one stored file into dec, so FFmpeg sees only the copy, and decodes it.
func (run *renderRun) decodeSource(ctx context.Context, dec string, src *plannedSource, plan *mixPlan) error {
	file := src.name + "." + src.ext
	staged := filepath.Join(dec, file)
	raw := filepath.Join(dec, src.name+".s16")
	fail := func(step string, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		run.r.deps.Logger.Warn("[Storymaker] A mix input failed", "file", file, "step", step, "error", err)
		return Fail(CodeRenderFailed, "The audio file %s could not be %s for the mix.", file, step)
	}
	if err := run.stageSource(ctx, src, staged); err != nil {
		return fail("copied", err)
	}
	if err := decodeToS16(ctx, run.st.FFmpegPath, staged, raw, src.maxSeconds); err != nil {
		return fail("decoded", err)
	}
	info, err := os.Stat(raw)
	if err != nil {
		return fail("decoded", err)
	}
	if info.Size() < 4 {
		return fail("decoded", errors.New("the decode is empty"))
	}
	src.rawPath, src.rawFrames = raw, info.Size()/4
	if src.folder == "narration" && src.ext == "mp3" {
		offset, err := mp3TimelineOffsetMS(staged, raw)
		if err != nil {
			run.r.deps.Logger.Warn("[Storymaker] An MP3 timeline could not be compared", "file", file, "error", err)
		} else if offset > mp3ToleranceMS || offset < -mp3ToleranceMS {
			plan.mp3OffsetMS[src.name] = offset
		}
	}
	return nil
}

func (run *renderRun) stageSource(ctx context.Context, src *plannedSource, dst string) error {
	r, _, err := run.r.deps.Projects.OpenAudio(ctx, run.plan.ProjectID, src.folder, src.name+"."+src.ext)
	if err != nil {
		return err
	}
	defer r.Close()
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, io.LimitReader(r, src.size))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// mp3TimelineOffsetMS compares the first speech of a stored MP3 on go-mp3's timeline, which its
// envelope came from, with the same speech in FFmpeg's decode. go-mp3 has no Xing or LAME handling,
// so the two may differ. The result is FFmpeg minus go-mp3, 0 when either side has no speech.
func mp3TimelineOffsetMS(staged, raw string) (int, error) {
	data, err := os.ReadFile(staged)
	if err != nil {
		return 0, err
	}
	decoded, _, err := storymaker.DecodeAudio(data)
	if err != nil {
		return 0, err
	}
	runs := storymaker.ComputeEnvelope(decoded).SpeechRuns()
	if len(runs) == 0 {
		return 0, nil
	}
	start, found, err := rawFirstSpeechMS(raw, runs[0][0]+mp3SearchMS)
	if err != nil || !found {
		return 0, err
	}
	return start - runs[0][0], nil
}

// rawFirstSpeechMS returns where speech starts in the first upToMS of a raw s16le stereo file, by the envelope rule.
func rawFirstSpeechMS(file string, upToMS int) (int, bool, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, false, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(upToMS)*mixBytesPerMS))
	if err != nil {
		return 0, false, err
	}
	samples := make([]float32, len(data)/2)
	for i := range samples {
		samples[i] = float32(int16(binary.LittleEndian.Uint16(data[2*i:]))) / 32768
	}
	runs := storymaker.ComputeEnvelope(storymaker.DecodedAudio{Rate: MixRate, Channels: mixChannels, Samples: samples}).SpeechRuns()
	if len(runs) == 0 {
		return 0, false, nil
	}
	return runs[0][0], true, nil
}

// readSFX reads one catalog take and expands it to stereo.
func (run *renderRun) readSFX(file string) ([]int16, error) {
	data, err := fs.ReadFile(run.r.deps.UI, path.Join("img/storymaker", file))
	if err != nil {
		return nil, Fail(CodeRenderFailed, "The sound %s could not be read.", file)
	}
	samples, err := parseSFXWAV(data)
	if err != nil {
		return nil, Fail(CodeRenderFailed, "The sound %s is not usable: %v", file, err)
	}
	return samples, nil
}

// parseSFXWAV reads a 48 kHz mono PCM16 WAV and returns interleaved stereo.
func parseSFXWAV(data []byte) ([]int16, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("it is not a WAV file")
	}
	formatOK := false
	for off := 12; off+8 <= len(data); {
		id := string(data[off : off+4])
		size := int(binary.LittleEndian.Uint32(data[off+4:]))
		body := off + 8
		if size < 0 || body+size > len(data) {
			return nil, errors.New("a chunk is cut off")
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return nil, errors.New("the format chunk is too short")
			}
			tag := binary.LittleEndian.Uint16(data[body:])
			channels := binary.LittleEndian.Uint16(data[body+2:])
			rate := binary.LittleEndian.Uint32(data[body+4:])
			bits := binary.LittleEndian.Uint16(data[body+14:])
			if tag != 1 || channels != 1 || rate != MixRate || bits != 16 {
				return nil, fmt.Errorf("it is format %d, %d channels, %d Hz, %d bits, not 48 kHz mono PCM16", tag, channels, rate, bits)
			}
			formatOK = true
		case "data":
			if !formatOK {
				return nil, errors.New("the data chunk comes before the format chunk")
			}
			frames := size / 2
			out := make([]int16, frames*2)
			for i := 0; i < frames; i++ {
				s := int16(binary.LittleEndian.Uint16(data[body+2*i:]))
				out[2*i], out[2*i+1] = s, s
			}
			return out, nil
		}
		off = body + size + size%2
	}
	return nil, errors.New("it has no data chunk")
}

// mergeSpans clips the spans to the film, sorts them and joins the overlapping ones.
func mergeSpans(spans []Span, frames int64) []Span {
	sorted := append([]Span(nil), spans...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].From < sorted[j].From })
	var out []Span
	for _, s := range sorted {
		if s.To > frames {
			s.To = frames
		}
		if s.From >= s.To {
			continue
		}
		if n := len(out); n > 0 && s.From <= out[n-1].To {
			if s.To > out[n-1].To {
				out[n-1].To = s.To
			}
			continue
		}
		out = append(out, s)
	}
	return out
}
