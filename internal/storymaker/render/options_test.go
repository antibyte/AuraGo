package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"aurago/internal/videostudio"
)

func TestParseRenderOptions(t *testing.T) {
	for _, empty := range [][]byte{nil, {}, []byte("  \n")} {
		opts, err := ParseRenderOptions(empty)
		if err != nil || opts.Quality != "standard" || opts.SRT || opts.VTT || opts.Stems || opts.BurnInCaptions != nil {
			t.Errorf("empty body %q = %+v, %v", empty, opts, err)
		}
	}
	opts, err := ParseRenderOptions([]byte(`{"burn_in_captions":false,"srt":true,"vtt":true,"stems":true,"quality":"high"}`))
	if err != nil || opts.BurnInCaptions == nil || *opts.BurnInCaptions || !opts.SRT || !opts.VTT || !opts.Stems || opts.Quality != "high" {
		t.Errorf("full body = %+v, %v", opts, err)
	}
	if opts, err := ParseRenderOptions([]byte(`{"srt":true}`)); err != nil || opts.Quality != "standard" {
		t.Errorf("an absent quality = %+v, %v", opts, err)
	}
	for _, bad := range []string{`{"handoff":true}`, `{"quality":"ultra"}`, `{"quality":""}`, `[]`, `{"srt":"yes"}`, `{"srt":true} {"srt":false}`} {
		if _, err := ParseRenderOptions([]byte(bad)); err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
}

func TestRenderOptionsFingerprint(t *testing.T) {
	hash := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	yes := true
	if got, want := (RenderOptions{BurnInCaptions: &yes, SRT: true, Stems: true, Quality: "high"}).Fingerprint(), hash("burn=true;srt=true;vtt=false;stems=true;quality=high"); got != want {
		t.Errorf("fingerprint = %s, want %s", got, want)
	}
	if got, want := (RenderOptions{Quality: "standard"}).Fingerprint(), hash("burn=null;srt=false;vtt=false;stems=false;quality=standard"); got != want {
		t.Errorf("fingerprint of the defaults = %s, want %s", got, want)
	}
}

func TestRenderOptionsEncoderQuality(t *testing.T) {
	base := videostudio.Encoder{Codec: videostudio.CodecNVENC, Quality: 30}
	if got := (RenderOptions{Quality: "standard"}).encoder(base); got != base {
		t.Errorf("standard changed the encoder: %+v", got)
	}
	if got := (RenderOptions{}).encoder(base); got != base {
		t.Errorf("zero options changed the encoder: %+v", got)
	}
	if got := (RenderOptions{Quality: "high"}).encoder(base); got.Quality != 18 || got.Codec != base.Codec {
		t.Errorf("high = %+v, want quality 18 on the same codec", got)
	}
}

// The stems count in the disk check of buildMixInput, which reads plan.stems.
func TestRenderOptionsStemsReachTheMixPlan(t *testing.T) {
	for _, stems := range []bool{false, true} {
		run := &renderRun{plan: &Plan{Options: RenderOptions{Stems: stems}}, fps: 30, report: compileReport{FrameCount: 30}}
		plan, err := run.planMixInput(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if plan.stems != stems {
			t.Errorf("plan.stems = %v with the option %v", plan.stems, stems)
		}
	}
}
