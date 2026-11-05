// Package storymakertest writes generated audio into a project, for tests that need a narrated or
// scored story without a provider. No production code imports it.
package storymakertest

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"aurago/internal/storymaker"
)

const (
	seedRate     = 24000
	seedMarginMS = 100
	// seedSpeechHz has 4 whole cycles in a 10 ms envelope window, so every full window has the same power.
	seedSpeechHz = 400
	seedMusicHz  = 220
	// seedPeak is the peak of a sine with an RMS of -20 dBFS: 10^(-20/20) * sqrt(2).
	seedPeak = 0.1 * math.Sqrt2
)

// SeedNarration writes a generated 24 kHz mono s16 WAV and its envelope for every missing piece,
// with speech at -20 dBFS from 100 ms to the end minus 100 ms. Used by the fixtures that the
// narration_incomplete gate would otherwise break.
func SeedNarration(ctx context.Context, p *storymaker.ProjectStore, id string, s *storymaker.Story, seconds float64) error {
	if seconds*1000 <= 2*seedMarginMS {
		return fmt.Errorf("storymakertest: a piece of %v s has no room for speech", seconds)
	}
	frames := int(math.Round(seconds * seedRate))
	margin := seedMarginMS * seedRate / 1000
	data := toneWAV(frames, margin, frames-margin, seedSpeechHz)
	decoded, ext, err := storymaker.DecodeAudio(data)
	if err != nil {
		return fmt.Errorf("storymakertest: decode the seeded piece: %w", err)
	}
	env := storymaker.ComputeEnvelope(decoded)
	files, err := p.PieceFiles(ctx, id)
	if err != nil {
		return err
	}
	if files == nil {
		files = map[string]storymaker.PieceFile{}
	}
	for _, block := range storymaker.Blocks(s) {
		for _, piece := range storymaker.PiecesOf(block) {
			hash := storymaker.PieceKey(block.Voice, piece.Text)
			if f, ok := files[hash]; ok && f.HasEnv {
				continue
			}
			if err := p.WritePiece(ctx, id, hash, ext, data, env); err != nil {
				return err
			}
			files[hash] = storymaker.PieceFile{Ext: ext, HasEnv: true}
		}
	}
	return nil
}

// SeedMusic writes a generated WAV and its meta through WriteMusic and WriteMusicMeta.
func SeedMusic(ctx context.Context, p *storymaker.ProjectStore, id string, key string, seconds float64) error {
	if seconds < 1 {
		return fmt.Errorf("storymakertest: music needs at least 1 s, got %v", seconds)
	}
	frames := int(math.Round(seconds * seedRate))
	data := toneWAV(frames, 0, frames, seedMusicHz)
	ext, durationMS, err := storymaker.ProbeAudio(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("storymakertest: probe the seeded music: %w", err)
	}
	if err := p.WriteMusic(ctx, id, key, ext, data); err != nil && !errors.Is(err, storymaker.ErrConflict) {
		return err
	}
	return p.WriteMusicMeta(ctx, id, key, storymaker.MusicMeta{V: 1, Ext: ext, DurationMS: durationMS})
}

// toneWAV is a 24 kHz mono PCM16 WAV of frames samples with a sine in [from, to) and silence around it.
func toneWAV(frames, from, to int, hz float64) []byte {
	const header = 44
	size := frames * 2
	data := make([]byte, header+size)
	copy(data[0:], "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+size))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1) // PCM
	binary.LittleEndian.PutUint16(data[22:], 1) // mono
	binary.LittleEndian.PutUint32(data[24:], seedRate)
	binary.LittleEndian.PutUint32(data[28:], seedRate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(size))
	for i := from; i < to; i++ {
		s := int16(math.Round(seedPeak * 32767 * math.Sin(2*math.Pi*hz*float64(i)/seedRate)))
		binary.LittleEndian.PutUint16(data[header+2*i:], uint16(s))
	}
	return data
}
