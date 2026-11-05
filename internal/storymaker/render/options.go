package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"aurago/internal/videostudio"
)

const (
	qualityStandard = "standard"
	qualityHigh     = "high"
	// highQualityCRF is the encoder quality of "high". "standard" keeps the selected encoder's value.
	highQualityCRF = 18
)

// RenderOptions are the export options of POST /projects/{id}/render.
type RenderOptions struct {
	BurnInCaptions *bool  `json:"burn_in_captions,omitempty"` // nil means style.captions
	SRT            bool   `json:"srt"`
	VTT            bool   `json:"vtt"`
	Stems          bool   `json:"stems"`
	Quality        string `json:"quality"` // standard | high
}

// ParseRenderOptions reads the request body. An empty body gives the defaults, and an unknown field,
// trailing data or an unknown quality is an error.
func ParseRenderOptions(body []byte) (RenderOptions, error) {
	opts := RenderOptions{Quality: qualityStandard}
	if len(bytes.TrimSpace(body)) == 0 {
		return opts, nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&opts); err != nil {
		return RenderOptions{}, fmt.Errorf("the options are not valid: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return RenderOptions{}, errors.New("the options are followed by more data")
	}
	if opts.Quality != qualityStandard && opts.Quality != qualityHigh {
		return RenderOptions{}, fmt.Errorf("quality must be %s or %s", qualityStandard, qualityHigh)
	}
	return opts, nil
}

// Fingerprint is the canonical hash of the options, for the idempotency check.
func (o RenderOptions) Fingerprint() string {
	burn := "null"
	if o.BurnInCaptions != nil {
		burn = strconv.FormatBool(*o.BurnInCaptions)
	}
	text := "burn=" + burn + ";srt=" + strconv.FormatBool(o.SRT) + ";vtt=" + strconv.FormatBool(o.VTT) +
		";stems=" + strconv.FormatBool(o.Stems) + ";quality=" + o.Quality
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// encoder returns the encoder the options ask for.
func (o RenderOptions) encoder(base videostudio.Encoder) videostudio.Encoder {
	if o.Quality == qualityHigh {
		base.Quality = highQualityCRF
	}
	return base
}
