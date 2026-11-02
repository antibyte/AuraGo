package videostudio

// Exported views of the encoder pieces that frame-input pipelines outside this package (Storymaker's
// render job) need. They return exactly what the Video Studio segment and preview graphs use, so one
// encoder choice (SelectEncoder) produces the same stream settings everywhere.

// GlobalArgs returns the arguments that initialise the encoder's hardware device; they precede all inputs.
func (e Encoder) GlobalArgs() []string { return e.globalArgs() }

// FilterTail returns the filter text appended after the format=yuv420p conversion ("" for software encoders,
// ",format=nv12,hwupload" for VAAPI and QSV).
func (e Encoder) FilterTail() string { return e.filterTail() }

// VideoArgs returns the video codec arguments for the output (codec, quality, GOP and the BT.709 tags).
func (e Encoder) VideoArgs(rate Rate) []string { return e.videoArgs(rate) }

// ResolveFFmpeg resolves the FFmpeg binary with the shared policy: a configured path must be an existing
// regular file, a configured bare name is looked up on PATH, an empty path searches PATH.
func ResolveFFmpeg(configuredPath string) (string, error) {
	return resolveBinary(configuredPath, "ffmpeg")
}
