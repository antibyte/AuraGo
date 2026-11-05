package render

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode"

	"aurago/internal/storymaker"
)

const (
	maxReportEvents   = 32768 // absolute ceiling of audio_events
	maxReportCaptions = 8000
	maxCaptionUnits   = 256 // UTF-16 units per caption line
)

// compileReport is the part of the page's compile report Go uses (spec "Compile report and job
// order"); unknown fields are ignored.
type compileReport struct {
	FrameCount     int                `json:"frame_count"`
	FPS            int                `json:"fps"`
	Width          int                `json:"width"`
	Height         int                `json:"height"`
	DurationMS     float64            `json:"duration_ms"`
	CatalogVersion string             `json:"catalog_version"`
	Scenes         []reportScene      `json:"scenes"`
	Issues         []storymaker.Issue `json:"issues"`
	AudioEvents    []reportAudioEvent `json:"audio_events"`
	Captions       []reportCaption    `json:"captions"`
	CuePrecision   *float64           `json:"cue_precision"`
}

// reportAudioEvent is one sound of the film. Kind is narration, dialogue or sfx, and Ref is a piece hash or
// an sfx id. A JS gain field does not exist here, so any gain in a report is ignored.
type reportAudioEvent struct {
	Kind    string `json:"kind"`
	Ref     string `json:"ref"`
	StartMS int64  `json:"start_ms"`
	SceneID string `json:"scene_id"`
	BlockID string `json:"block_id,omitempty"`
	Piece   int    `json:"piece,omitempty"`
	Take    int    `json:"take,omitempty"`
}

// reportCaption is one caption of the film, for burned captions and subtitle files.
type reportCaption struct {
	StartMS int64    `json:"start_ms"`
	EndMS   int64    `json:"end_ms"`
	Lines   []string `json:"lines"`
	SceneID string   `json:"scene_id"`
	BlockID string   `json:"block_id"`
}

// reportBlock is what Go knows about a measured block: the hash of each piece and the captionKey of its text.
type reportBlock struct {
	SceneID string
	Kind    string
	Hashes  []string
	Text    string
}

// captionKey removes JS whitespace and control characters. Both sides of the caption check use it, so a
// narration with U+0001 passes Validate and passes here.
func captionKey(s string) string {
	return strings.Map(func(r rune) rune {
		if storymaker.IsJSSpace(r) || unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// planReportBlocks derives the report expectations from what Go itself computed when the job was queued.
// A block in AudioInfo is measured and needs events. Any other block of the plan is estimated by the page,
// which still writes captions for it.
func planReportBlocks(plan *Plan) (measured map[string]reportBlock, captionText map[string]string, maxEvents int) {
	captionText = make(map[string]string, len(plan.Blocks))
	for _, b := range plan.Blocks {
		captionText[b.ID] = captionKey(b.Text)
	}
	measured = map[string]reportBlock{}
	for sceneID, audio := range plan.AudioInfo.Narration {
		for _, block := range audio.Blocks {
			hashes := make([]string, len(block.Pieces))
			for i, piece := range block.Pieces {
				hashes[i] = piece.Hash
			}
			measured[block.ID] = reportBlock{SceneID: sceneID, Kind: block.Kind, Hashes: hashes, Text: captionText[block.ID]}
			maxEvents += len(hashes)
		}
	}
	return measured, captionText, maxEvents
}

// checkAudioEvents returns why the events are refused, or "".
func checkAudioEvents(events []reportAudioEvent, filmMS int64, want reportExpect) string {
	if limit := min(want.MaxEvents, maxReportEvents); len(events) > limit {
		return fmt.Sprintf("it lists too many audio events: %d, the limit is %d", len(events), limit)
	}
	seen := make(map[string]map[int]bool, len(want.Blocks))
	for i, ev := range events {
		if ev.StartMS < 0 || ev.StartMS >= filmMS {
			return fmt.Sprintf("audio event %d starts at %d ms, outside the film of %d ms", i, ev.StartMS, filmMS)
		}
		switch ev.Kind {
		case storymaker.BlockKindNarration, storymaker.BlockKindDialogue:
			b, ok := want.Blocks[ev.BlockID]
			if !ok {
				return fmt.Sprintf("audio event %d names the unknown block %q", i, want.clean(ev.BlockID, 60))
			}
			if b.Kind != ev.Kind || b.SceneID != ev.SceneID {
				return fmt.Sprintf("audio event %d has the wrong kind or scene for block %q", i, ev.BlockID)
			}
			if ev.Piece < 0 || ev.Piece >= len(b.Hashes) {
				return fmt.Sprintf("audio event %d names piece %d of block %q, which has %d", i, ev.Piece, ev.BlockID, len(b.Hashes))
			}
			if b.Hashes[ev.Piece] != ev.Ref {
				return fmt.Sprintf("the hash of audio event %d does not match piece %d of block %q", i, ev.Piece, ev.BlockID)
			}
			if seen[ev.BlockID] == nil {
				seen[ev.BlockID] = map[int]bool{}
			}
			if seen[ev.BlockID][ev.Piece] {
				return fmt.Sprintf("block %q lists piece %d twice", ev.BlockID, ev.Piece)
			}
			seen[ev.BlockID][ev.Piece] = true
		case "sfx":
			// checkSFXEvents owns the ref and the take.
		default:
			return fmt.Sprintf("audio event %d has the unknown kind %q", i, want.clean(ev.Kind, 40))
		}
	}
	// The pieces of a block form a prefix: compile may drop the events past the film end, never one in the
	// middle. A scene always starts inside the film, so a narration block has at least piece 0. A far delay can
	// push a whole dialogue block past the film end, so a dialogue block may have none.
	for id, b := range want.Blocks {
		pieces := seen[id]
		if len(pieces) == 0 {
			if b.Kind == storymaker.BlockKindNarration {
				return fmt.Sprintf("the measured block %q has no audio event", id)
			}
			continue
		}
		for p := 0; p < len(pieces); p++ {
			if !pieces[p] {
				return fmt.Sprintf("the events of block %q have a gap at piece %d", id, p)
			}
		}
	}
	return ""
}

// checkCaptions returns why the captions are refused, or "".
func checkCaptions(captions []reportCaption, filmMS int64, want reportExpect) string {
	if len(captions) > maxReportCaptions {
		return fmt.Sprintf("it lists %d captions, the limit is %d", len(captions), maxReportCaptions)
	}
	for i, c := range captions {
		text, ok := want.captionText(c.BlockID)
		if !ok {
			return fmt.Sprintf("caption %d names the unknown block %q", i, want.clean(c.BlockID, 60))
		}
		if c.StartMS < 0 || c.EndMS <= c.StartMS || c.EndMS > filmMS {
			return fmt.Sprintf("caption %d runs from %d to %d ms, outside the film of %d ms", i, c.StartMS, c.EndMS, filmMS)
		}
		if i > 0 && c.StartMS < captions[i-1].StartMS {
			return fmt.Sprintf("the captions are not in time order at caption %d", i)
		}
		if len(c.Lines) < 1 || len(c.Lines) > 2 {
			return fmt.Sprintf("caption %d has %d lines, it must have 1 or 2 lines", i, len(c.Lines))
		}
		for _, line := range c.Lines {
			if captionKey(line) == "" {
				return fmt.Sprintf("caption %d has an empty line", i)
			}
			if storymaker.UTF16Len(line) > maxCaptionUnits {
				return fmt.Sprintf("caption %d has a line over %d UTF-16 units", i, maxCaptionUnits)
			}
		}
		if !strings.Contains(text, captionKey(strings.Join(c.Lines, ""))) {
			return fmt.Sprintf("caption %d is not in the text of block %q", i, c.BlockID)
		}
	}
	return ""
}

type reportScene struct {
	ID         string `json:"id"`
	StartFrame int    `json:"start_frame"`
	EndFrame   int    `json:"end_frame"`
	Kind       string `json:"kind"`
}

// reportRefusal is a report Go will not render: the code is a job error code, the message is shown
// to the user and to the model.
type reportRefusal struct {
	Code    string
	Message string
}

func (r *reportRefusal) Error() string { return r.Code + ": " + r.Message }

// reportExpect is what Go knows independently of the page.
type reportExpect struct {
	FPS            int
	Width, Height  int
	MaxFrames      int
	CatalogVersion string
	SceneIDs       []string
	Catalog        *storymaker.Catalog // sfx refs
	SFXEvents      int                 // sfx share of MaxEvents, from sfxEventBound
	// Blocks holds the measured blocks by id. Only they may have audio events.
	Blocks map[string]reportBlock
	// CaptionText holds the captionKey of every other block the plan knows. The page also writes captions for
	// estimated blocks, and those blocks have no audio events.
	CaptionText map[string]string
	// MaxEvents bounds audio_events: the pieces of all measured blocks.
	MaxEvents int
	// Prefix and Token are the render secrets. A text the page wrote is cleaned of both before it reaches a
	// refusal message, and a refusal message reaches the job record and the model.
	Prefix, Token string
}

// clean removes the secrets from a page text and then clips it. The order matters: a clip that cuts through the
// prefix leaves a part that withoutSecrets no longer matches.
func (e reportExpect) clean(s string, n int) string {
	return clip(withoutSecrets(s, e.Prefix, e.Token), n)
}

// captionText returns the captionKey of a block's text, measured or estimated.
func (e reportExpect) captionText(id string) (string, bool) {
	if b, ok := e.Blocks[id]; ok {
		return b.Text, true
	}
	text, ok := e.CaptionText[id]
	return text, ok
}

// parseReport decodes and validates a compile report: the numbers must agree with the job (fps,
// canvas, catalog version), frame_count must not exceed the Go-computed limit, the scene ranges must
// be monotonic and cover frame_count exactly, and an error issue refuses the render.
func parseReport(body []byte, want reportExpect) (compileReport, *reportRefusal) {
	want.MaxEvents = min(want.MaxEvents+want.SFXEvents, maxReportEvents)
	var rep compileReport
	if err := json.Unmarshal(body, &rep); err != nil {
		return rep, &reportRefusal{CodeRenderFailed, "The compile report is not valid JSON."}
	}
	bad := func(format string, args ...any) (compileReport, *reportRefusal) {
		// The arguments can carry page text (a catalog version, a scene id), so the whole reason is cleaned and bounded.
		return rep, &reportRefusal{CodeRenderFailed, "The compile report was refused: " + want.clean(fmt.Sprintf(format, args...), 400)}
	}
	if rep.FPS != want.FPS || rep.Width != want.Width || rep.Height != want.Height {
		return bad("it describes %dx%d at %d fps, the job is %dx%d at %d fps", rep.Width, rep.Height, rep.FPS, want.Width, want.Height, want.FPS)
	}
	if rep.CatalogVersion != want.CatalogVersion {
		return bad("the catalog version %q differs from %q", rep.CatalogVersion, want.CatalogVersion)
	}
	for _, is := range rep.Issues {
		if is.Severity == storymaker.SeverityError {
			code := CodeInvalidStory
			if is.Code == storymaker.CodeCodeError || is.Code == storymaker.CodeDeadline {
				code = CodeCodeError
			}
			// Both fields are page text: scrub each one before the join, then clip the result.
			msg := withoutSecrets(is.Message, want.Prefix, want.Token)
			if is.SceneID != "" {
				msg = "Scene " + withoutSecrets(is.SceneID, want.Prefix, want.Token) + ": " + msg
			}
			return rep, &reportRefusal{code, clip(strings.TrimSpace(msg), 400)}
		}
	}
	if rep.FrameCount < 1 {
		return bad("the story has no frames")
	}
	if rep.FrameCount > want.MaxFrames {
		return rep, &reportRefusal{CodeInvalidStory, fmt.Sprintf("The video would have %d frames; the limit is %d (render.max_duration_seconds).", rep.FrameCount, want.MaxFrames)}
	}
	if len(rep.Scenes) != len(want.SceneIDs) {
		return bad("it lists %d scenes, the story has %d", len(rep.Scenes), len(want.SceneIDs))
	}
	maxEnd := 0
	for i, sc := range rep.Scenes {
		if sc.ID != want.SceneIDs[i] {
			return bad("scene %d is %q, the story has %q", i, sc.ID, want.SceneIDs[i])
		}
		if sc.StartFrame < 0 || sc.EndFrame <= sc.StartFrame || sc.EndFrame > rep.FrameCount {
			return bad("scene %q has the invalid range %d to %d", sc.ID, sc.StartFrame, sc.EndFrame)
		}
		if i > 0 {
			prev := rep.Scenes[i-1]
			// Transitions overlap scenes, so a start may lie before the previous end, but the ranges
			// still move forward.
			if sc.StartFrame <= prev.StartFrame || sc.EndFrame < prev.EndFrame {
				return bad("the scene ranges are not monotonic at %q", sc.ID)
			}
		}
		if sc.EndFrame > maxEnd {
			maxEnd = sc.EndFrame
		}
	}
	if maxEnd != rep.FrameCount {
		return bad("frame_count %d is not the last scene end %d", rep.FrameCount, maxEnd)
	}
	// Every bound below uses Go's own film length, never the report's value.
	filmMS := int64(math.Round(float64(rep.FrameCount) * 1000 / float64(rep.FPS)))
	if math.Abs(rep.DurationMS-float64(filmMS)) > 1 {
		return bad("duration_ms %v is not the film length of %d ms", rep.DurationMS, filmMS)
	}
	if reason := checkAudioEvents(rep.AudioEvents, filmMS, want); reason != "" {
		return bad("%s", reason)
	}
	if reason := checkCaptions(rep.Captions, filmMS, want); reason != "" {
		return bad("%s", reason)
	}
	if msg := checkSFXEvents(rep.AudioEvents, want.Catalog); msg != "" {
		return bad("%s", msg)
	}
	return rep, nil
}

// sfxEventBound is the sfx share of MaxEvents: one event per sfx beat, for a pattern
// floor(max(duration, default duration at the largest rhythm)/smallest spacing)+1, and one per show
// with entry pop when auto_sfx is on. The beats of a code scene never run.
func sfxEventBound(s *storymaker.Story, cat *storymaker.Catalog) int {
	n := 0
	for i := range s.Scenes {
		if s.Scenes[i].Kind == "code" {
			continue
		}
		for j := range s.Scenes[i].Beats {
			b := &s.Scenes[i].Beats[j]
			switch {
			case b.Do == "sfx":
				n += sfxBeatBound(b, cat)
			case b.Do == "show" && b.Entry == "pop" && s.Style.AutoSFX:
				n++
			}
		}
	}
	return n
}

func sfxBeatBound(b *storymaker.Beat, cat *storymaker.Catalog) int {
	entry, ok := cat.Lookup(b.Sound)
	if !ok || entry.Pattern == nil || entry.Pattern.SpacingMS[0] <= 0 {
		return 1
	}
	span := max(b.Duration*1000, entry.Pattern.DefaultDurationMS*maxRhythmFactor)
	return int(span/entry.Pattern.SpacingMS[0]) + 1 // a positive float truncates to its floor
}

// checkSFXEvents says why an sfx event is refused, or returns "". Its ref must be a catalog sfx and
// its take an index into that entry's takes. A nil catalog refuses every sfx event.
func checkSFXEvents(events []reportAudioEvent, cat *storymaker.Catalog) string {
	for i := range events {
		ev := &events[i]
		if ev.Kind != "sfx" {
			continue
		}
		entry, ok := cat.Lookup(ev.Ref)
		if !ok || entry.Kind != "sfx" {
			return fmt.Sprintf("the sfx event %d names the unknown sound %q", i, clip(ev.Ref, 80))
		}
		if ev.Take < 0 || ev.Take >= len(entry.Takes) {
			return fmt.Sprintf("the sfx event %d of %q has take %d of %d", i, ev.Ref, ev.Take, len(entry.Takes))
		}
	}
	return ""
}

const maxRhythmFactor = 1.25 // pinned against RHYTHM in motion.js by a test
