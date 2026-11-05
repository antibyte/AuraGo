package storymaker

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

const (
	envelopeRate     = 100 // windows per second
	envelopeWindowMS = 10
	speechLevel      = 85  // level byte of -40 dBFS
	gapBridgeMS      = 100 // runs separated by less are bridged
	inMarginMS       = 40
	outMarginMS      = 100
	sentencePauseMS  = 350
	splitPauseMS     = 150
)

// Levels marshals as a JSON array of integers 0..255. A plain []uint8 would become base64.
type Levels []uint8

// MarshalJSON writes an array, also for nil.
func (l Levels) MarshalJSON() ([]byte, error) {
	out := make([]byte, 0, len(l)*4+2)
	out = append(out, '[')
	for i, v := range l {
		if i > 0 {
			out = append(out, ',')
		}
		out = strconv.AppendUint(out, uint64(v), 10)
	}
	return append(out, ']'), nil
}

// UnmarshalJSON reads an array of integers and refuses any value outside 0..255.
func (l *Levels) UnmarshalJSON(b []byte) error {
	var raw []int
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(Levels, len(raw))
	for i, v := range raw {
		if v < 0 || v > 255 {
			return fmt.Errorf("storymaker: level %d is outside 0..255", v)
		}
		out[i] = uint8(v)
	}
	*l = out
	return nil
}

// Envelope is the stored 10 ms loudness curve of one audio file.
type Envelope struct {
	V          int    `json:"v"` // 1
	DurationMS int    `json:"duration_ms"`
	Rate       int    `json:"rate"` // 100
	Levels     Levels `json:"levels"`
}

// levelByte is round((clamp(dBFS,-60,0)+60)*255/60), silence 0.
func levelByte(power float64) uint8 {
	if power <= 0 {
		return 0
	}
	db := 10 * math.Log10(power)
	if db < -60 {
		db = -60
	}
	if db > 0 {
		db = 0
	}
	return uint8(math.Round((db + 60) * 255 / 60))
}

// levelPower is the inverse of levelByte: 0 when b is 0, else 10^((b*60/255-60)/10).
func levelPower(b uint8) float64 {
	if b == 0 {
		return 0
	}
	return math.Pow(10, (float64(b)*60/255-60)/10)
}

// ComputeEnvelope measures mean power across channels per window. Window i covers the frames floor(i*rate/100) to
// floor((i+1)*rate/100)-1, and a final partial window counts. A fixed window would drift at 22050 Hz.
func ComputeEnvelope(a DecodedAudio) Envelope {
	env := Envelope{V: 1, Rate: envelopeRate, Levels: Levels{}}
	if a.Rate <= 0 || a.Channels <= 0 || len(a.Samples) < a.Channels {
		return env
	}
	channels := int64(a.Channels)
	frames := int64(len(a.Samples)) / channels
	rate := int64(a.Rate)
	env.DurationMS = roundedMS(frames, a.Rate)
	windows := (frames*envelopeRate + rate - 1) / rate
	env.Levels = make(Levels, windows)
	for i := int64(0); i < windows; i++ {
		first := i * rate / envelopeRate
		end := (i + 1) * rate / envelopeRate
		if end > frames {
			end = frames
		}
		if end <= first {
			continue
		}
		sum := 0.0
		samples := a.Samples[first*channels : end*channels]
		for _, s := range samples {
			sum += float64(s) * float64(s)
		}
		env.Levels[i] = levelByte(sum / float64(len(samples)))
	}
	return env
}

// SpeechRuns lists the runs at or above -40 dBFS, in ms of the file, ascending, with gaps under 100 ms bridged.
func (e Envelope) SpeechRuns() [][2]int {
	var runs [][2]int
	for i := 0; i < len(e.Levels); {
		if e.Levels[i] < speechLevel {
			i++
			continue
		}
		j := i
		for j < len(e.Levels) && e.Levels[j] >= speechLevel {
			j++
		}
		start, end := i*envelopeWindowMS, j*envelopeWindowMS
		if end > e.DurationMS {
			end = e.DurationMS
		}
		i = j
		if start >= end {
			continue
		}
		if n := len(runs); n > 0 && start-runs[n-1][1] < gapBridgeMS {
			runs[n-1][1] = end
		} else {
			runs = append(runs, [2]int{start, end})
		}
	}
	return runs
}

// ParseEnvelope reads a stored env.json. These are ordinary workspace files, so nothing in them is trusted:
// v == 1, rate == 100, 0 < duration_ms <= 600000, and len(levels) equals ceil(duration_ms/10) within 1.
func ParseEnvelope(data []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return Envelope{}, fmt.Errorf("storymaker: envelope: %w", err)
	}
	if e.V != 1 {
		return Envelope{}, fmt.Errorf("storymaker: envelope version %d", e.V)
	}
	if e.Rate != envelopeRate {
		return Envelope{}, fmt.Errorf("storymaker: envelope rate %d", e.Rate)
	}
	if e.DurationMS <= 0 || e.DurationMS > maxDecodedSeconds*1000 {
		return Envelope{}, fmt.Errorf("storymaker: envelope duration %d ms", e.DurationMS)
	}
	want := (e.DurationMS + envelopeWindowMS - 1) / envelopeWindowMS
	if n := len(e.Levels); n < want-1 || n > want+1 {
		return Envelope{}, fmt.Errorf("storymaker: envelope has %d levels, want %d", n, want)
	}
	return e, nil
}

// AudioPieceInput is one stored piece of a block.
type AudioPieceInput struct {
	Hash, Ext string
	Chars     int
	Join      string
	Env       Envelope
}

// AudioPiece says where a piece plays inside its block and which range of its file is used.
type AudioPiece struct {
	Hash  string `json:"hash"`
	Ext   string `json:"ext"`
	AtMS  int    `json:"at_ms"`
	InMS  int    `json:"in_ms"`
	OutMS int    `json:"out_ms"`
	Chars int    `json:"chars"`
}

// AudioBlock is a measured block as the /narration answer, Plan.AudioInfo and the mix see it.
type AudioBlock struct {
	ID         string       `json:"id"`
	Kind       string       `json:"kind"`
	Beat       *int         `json:"beat,omitempty"`
	Speaker    string       `json:"speaker"`
	DurationMS int          `json:"duration_ms"`
	Spans      [][2]int     `json:"spans"`
	Levels     Levels       `json:"levels"`
	Pieces     []AudioPiece `json:"pieces"`
	SpeechOK   bool         `json:"speech_ok"`
}

func joinPauseMS(join string) int {
	switch join {
	case JoinSentence:
		return sentencePauseMS
	case JoinSplit:
		return splitPauseMS
	}
	return 0
}

// AssembleBlock is the one function behind /narration, Plan.AudioInfo and the mix. Each piece plays only its used range
// and an explicit pause separates pieces. A piece with no speech run plays whole and clears SpeechOK.
func AssembleBlock(b NarrationBlock, pieces []AudioPieceInput, fps int) AudioBlock {
	out := AudioBlock{
		ID: b.ID, Kind: b.Kind, Speaker: b.Speaker,
		Spans: [][2]int{}, Levels: Levels{}, Pieces: make([]AudioPiece, 0, len(pieces)), SpeechOK: true,
	}
	if b.Kind == BlockKindDialogue {
		beat := b.Beat
		out.Beat = &beat
	}
	at, pause := 0, 0
	for _, p := range pieces {
		at += pause
		in, end := 0, p.Env.DurationMS
		runs := p.Env.SpeechRuns()
		if len(runs) == 0 {
			out.SpeechOK = false
		} else {
			in = max(0, runs[0][0]-inMarginMS)
			end = min(p.Env.DurationMS, runs[len(runs)-1][1]+outMarginMS)
		}
		out.Pieces = append(out.Pieces, AudioPiece{Hash: p.Hash, Ext: p.Ext, AtMS: at, InMS: in, OutMS: end, Chars: p.Chars})
		for _, r := range runs {
			first, last := max(r[0], in), min(r[1], end)
			if first < last {
				out.Spans = append(out.Spans, [2]int{at + first - in, at + last - in})
			}
		}
		at += end - in
		pause = joinPauseMS(p.Join)
	}
	out.DurationMS = at
	out.Levels = blockLevels(out, pieces, fps)
	return out
}

// blockLevels gives one level per frame. The block is a 10 ms grid: a bin holds the piece's level byte at the matching
// file position, or 0 in a pause. Frame k averages power over bins k*100/fps to (k+1)*100/fps-1.
func blockLevels(block AudioBlock, pieces []AudioPieceInput, fps int) Levels {
	if fps <= 0 || block.DurationMS <= 0 || len(block.Pieces) == 0 {
		return Levels{}
	}
	frames := (block.DurationMS*fps + 999) / 1000
	bins := (block.DurationMS + envelopeWindowMS - 1) / envelopeWindowMS
	out := make(Levels, frames)
	piece := 0
	for k := 0; k < frames; k++ {
		lo := k * 100 / fps
		hi := (k+1)*100/fps - 1
		if hi < lo {
			hi = lo
		}
		if hi > bins-1 {
			hi = bins - 1
		}
		sum := 0.0
		for j := lo; j <= hi; j++ {
			t := j * envelopeWindowMS
			for piece+1 < len(block.Pieces) && t >= block.Pieces[piece+1].AtMS {
				piece++
			}
			sum += levelPower(binLevel(block.Pieces[piece], pieces[piece].Env.Levels, t))
		}
		if hi >= lo {
			out[k] = levelByte(sum / float64(hi-lo+1))
		}
	}
	return out
}

// binLevel reads the level at block time t, or 0 outside the used range of the piece.
func binLevel(p AudioPiece, levels Levels, t int) uint8 {
	if t < p.AtMS || t >= p.AtMS+p.OutMS-p.InMS {
		return 0
	}
	i := (p.InMS + t - p.AtMS) / envelopeWindowMS
	if i >= len(levels) {
		return 0
	}
	return levels[i]
}
