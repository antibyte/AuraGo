package render

import (
	"fmt"
	"mime"
	"net/http"
	"sync"
	"sync/atomic"
)

// IngestError is a refused frame. Status is the HTTP status the listener answers with; the render
// job treats every IngestError as fatal.
type IngestError struct {
	Code    string
	Message string
	Status  int
}

func (e *IngestError) Error() string { return e.Code + ": " + e.Message }

func refuse(status int, code, format string, args ...any) *IngestError {
	return &IngestError{Code: code, Message: fmt.Sprintf(format, args...), Status: status}
}

// FrameSink consumes the accepted frames, normally the stdin of FFmpeg.
type FrameSink interface {
	// WriteFrame receives the validated bytes of one frame.
	WriteFrame(data []byte) error
	// Finish ends the stream after the last frame.
	Finish() error
}

// FrameOutcome is what the listener tells the page after a frame.
type FrameOutcome int

const (
	// FrameOK: the frame was taken, send the next one.
	FrameOK FrameOutcome = iota
	// FrameDone: the last frame was taken.
	FrameDone
	// FrameRestart: the encoder was replaced; the page renders from frame 0 again.
	FrameRestart
)

type ingestState int

const (
	ingestIdle ingestState = iota
	ingestRunning
	ingestFinished
	ingestStopped
)

// IngestConfig fixes what a frame must look like and how many are allowed.
type IngestConfig struct {
	Width, Height int
	Transport     string
	// FrameLimit is the most frames the job may ever take: Go computes it from
	// render.max_duration_seconds times fps, never from the page.
	FrameLimit int
}

// Ingest is the strict state machine behind POST <base>/frame: running state only, frames in order
// from 0, a Go-computed frame limit, Content-Type and header or size checks against the canvas.
type Ingest struct {
	cfg IngestConfig

	mu      sync.Mutex
	state   ingestState
	next    int
	total   int
	sink    FrameSink
	recover func(err error) (FrameSink, bool)
	onFrame func(n int)
	bytes   atomic.Int64
}

// NewIngest returns an idle ingest: every frame is refused until Start.
func NewIngest(cfg IngestConfig) *Ingest { return &Ingest{cfg: cfg} }

// MaxFrameBytes bounds encoded frames by canvas size. PNG may contain incompressible illustrated pixels.
func MaxFrameBytes(cfg IngestConfig) int64 {
	if cfg.Transport == TransportRaw {
		return int64(cfg.Width) * int64(cfg.Height) * 4
	}
	const base = 4 << 20
	pixels := int64(cfg.Width) * int64(cfg.Height)
	limit := max(int64(base), base*pixels/(1920*1080))
	if cfg.Transport == TransportPNG || cfg.Transport == TransportScreenshot {
		limit = max(limit, pixels*4+pixels*4/100+(64<<10))
	}
	return limit
}

// Start moves to the running state with the number of frames the validated report announced. recoverFn
// is called when the sink fails: it may return a replacement sink (the one-shot software restart),
// in which case the page is told to start over.
func (in *Ingest) Start(total int, sink FrameSink, recoverFn func(err error) (FrameSink, bool), onFrame func(n int)) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.state, in.next, in.total, in.sink, in.recover, in.onFrame = ingestRunning, 0, total, sink, recoverFn, onFrame
}

// Stop refuses every further frame (the job left the running state).
func (in *Ingest) Stop() {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.state = ingestStopped
}

// Next returns the index of the next expected frame.
func (in *Ingest) Next() int {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.next
}

// Bytes is the total of accepted frame bytes.
func (in *Ingest) Bytes() int64 { return in.bytes.Load() }

// Accept validates and forwards one frame. n is the X-SM-Frame value.
func (in *Ingest) Accept(n int, contentType string, data []byte) (FrameOutcome, error) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.state != ingestRunning {
		return FrameOK, refuse(http.StatusConflict, "not_running", "The render is not accepting frames.")
	}
	if n < 0 || n != in.next {
		return FrameOK, refuse(http.StatusConflict, "frame_order", "Frame %d arrived, frame %d was expected.", n, in.next)
	}
	if n >= in.cfg.FrameLimit || n >= in.total {
		return FrameOK, refuse(http.StatusRequestEntityTooLarge, "frame_limit", "Frame %d is past the limit of %d frames.", n, min(in.cfg.FrameLimit, in.total))
	}
	if err := checkFrame(in.cfg, contentType, data); err != nil {
		return FrameOK, err
	}
	if err := in.sink.WriteFrame(data); err != nil {
		if in.recover != nil {
			if replacement, ok := in.recover(err); ok {
				in.sink, in.next = replacement, 0
				return FrameRestart, nil
			}
		}
		return FrameOK, refuse(http.StatusInternalServerError, "encoder_failed", "The encoder stopped: %v", err)
	}
	in.bytes.Add(int64(len(data)))
	in.next++
	if in.onFrame != nil {
		in.onFrame(n)
	}
	if in.next >= in.total {
		in.state = ingestFinished
		if err := in.sink.Finish(); err != nil {
			return FrameDone, refuse(http.StatusInternalServerError, "encoder_failed", "The encoder could not finish: %v", err)
		}
		return FrameDone, nil
	}
	return FrameOK, nil
}

// checkFrame verifies content type and payload against the canvas before any byte reaches FFmpeg.
func checkFrame(cfg IngestConfig, contentType string, data []byte) *IngestError {
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return refuse(http.StatusUnsupportedMediaType, "content_type", "The frame has no usable Content-Type.")
	}
	want := map[string]string{TransportPNG: "image/png", TransportWebP: "image/webp", TransportRaw: "application/octet-stream", TransportScreenshot: "image/png"}[cfg.Transport]
	// text/plain is the CORS-safelisted type a page uses to send frames without a preflight request;
	// the bytes are checked against the transport below either way.
	if media != want && media != "text/plain" {
		return refuse(http.StatusUnsupportedMediaType, "content_type", "The transport %s expects %s frames, not %s.", cfg.Transport, want, media)
	}
	switch cfg.Transport {
	case TransportRaw:
		if want := cfg.Width * cfg.Height * 4; len(data) != want {
			return refuse(http.StatusBadRequest, "frame_size", "A raw frame has %d bytes, the canvas needs %d.", len(data), want)
		}
	case TransportPNG, TransportScreenshot:
		w, h, err := pngSize(data)
		if err != nil {
			return refuse(http.StatusBadRequest, "frame_format", "%v", err)
		}
		if w != cfg.Width || h != cfg.Height {
			return refuse(http.StatusBadRequest, "frame_size", "The frame is %dx%d, the canvas is %dx%d.", w, h, cfg.Width, cfg.Height)
		}
	case TransportWebP:
		w, h, err := webpSize(data)
		if err != nil {
			return refuse(http.StatusBadRequest, "frame_format", "%v", err)
		}
		if w != cfg.Width || h != cfg.Height {
			return refuse(http.StatusBadRequest, "frame_size", "The frame is %dx%d, the canvas is %dx%d.", w, h, cfg.Width, cfg.Height)
		}
	default:
		return refuse(http.StatusBadRequest, "transport", "Unknown transport %q.", cfg.Transport)
	}
	return nil
}
