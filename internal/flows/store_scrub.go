package flows

// Scrubber redacts registered secrets in run data before the store writes it (S7). The server
// supplies it (internal/server flowStoreScrubber): this package cannot reach the global scrubber.
type Scrubber struct {
	// JSON returns the scrubbed encoding of one encoded JSON object.
	JSON func(data []byte) ([]byte, error)
	// Text returns text with every secret replaced.
	Text func(text string) string
}

// SetScrubber makes the store scrub what it writes: the JSON of trigger data, step params and
// step outputs, and the error messages of steps and runs. The API scrubs every read as well;
// that stays, because a secret registered after the write is only caught there. Call it before
// the first run starts. A nil field leaves that kind of data unscrubbed.
func (s *Store) SetScrubber(sc Scrubber) { s.scrubber.Store(&sc) }

// scrubJSON runs the JSON scrubber over encoded text; without one the text is returned as it is.
func (s *Store) scrubJSON(text string) (string, error) {
	sc := s.scrubber.Load()
	if sc == nil || sc.JSON == nil {
		return text, nil
	}
	out, err := sc.JSON([]byte(text))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// scrubText runs the text scrubber; without one the text is returned as it is.
func (s *Store) scrubText(text string) string {
	sc := s.scrubber.Load()
	if sc == nil || sc.Text == nil || text == "" {
		return text
	}
	return sc.Text(text)
}
