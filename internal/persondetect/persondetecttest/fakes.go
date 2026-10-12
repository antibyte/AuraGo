// Package persondetecttest provides fakes for tests of code that uses persondetect:
// a scripted detector and factory, a frame source, a clock and an alert dispatcher.
package persondetecttest

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"sync"
	"testing"
	"time"

	"aurago/internal/persondetect"
)

// Timings every fake detection reports.
const (
	FakeWallMS = 12.0
	FakeCPUMS  = 30.0
)

// JPEG returns a valid w×h JPEG frame; different sizes give different bytes.
func JPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// CPUInfo is the DetectorInfo of a healthy CPU detector with the standard model.
func CPUInfo() persondetect.DetectorInfo {
	return persondetect.DetectorInfo{
		Model: persondetect.ModelStandard, ModelName: "D-FINE-N",
		Requested: persondetect.BackendAuto, Active: persondetect.BackendCPU,
		Runtime: "onnxruntime 1.26.0", Providers: []string{"CPUExecutionProvider"},
		SelfTest: persondetect.SelfTest{Passed: true, ReferencePersons: 4, CPUWallMS: 60, CPUCPUMS: 120},
	}
}

// Result is one scripted Detect outcome.
type Result struct {
	Boxes []persondetect.Box
	Err   error
}

// Call records one Detect request.
type Call struct {
	JPEG     []byte
	MinScore float64
}

// Script is the behavior shared by every detector a Factory creates: queued
// results first, then the current boxes. Boxes below the request's minScore are
// dropped, like the real worker does.
type Script struct {
	mu    sync.Mutex
	boxes []persondetect.Box
	queue []Result
	calls []Call
}

// SetBoxes sets the boxes returned once the queue is empty.
func (s *Script) SetBoxes(boxes ...persondetect.Box) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.boxes = append([]persondetect.Box(nil), boxes...)
}

// Push queues results that are returned before the current boxes.
func (s *Script) Push(results ...Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = append(s.queue, results...)
}

// Calls returns the Detect requests seen so far.
func (s *Script) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Call(nil), s.calls...)
}

func (s *Script) next(data []byte, minScore float64) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, Call{JPEG: append([]byte(nil), data...), MinScore: minScore})
	if len(s.queue) > 0 {
		r := s.queue[0]
		s.queue = s.queue[1:]
		return r
	}
	return Result{Boxes: append([]persondetect.Box(nil), s.boxes...)}
}

// Detector implements persondetect.Detector with a Script.
type Detector struct {
	script *Script
	info   persondetect.DetectorInfo
	mu     sync.Mutex
	closed bool
}

// Detect returns the next scripted result for a decodable JPEG.
func (d *Detector) Detect(ctx context.Context, data []byte, minScore float64) (persondetect.DetectResult, error) {
	d.mu.Lock()
	closed := d.closed
	d.mu.Unlock()
	if closed {
		return persondetect.DetectResult{}, persondetect.ErrDetectorClosed
	}
	if err := ctx.Err(); err != nil {
		return persondetect.DetectResult{}, err
	}
	r := d.script.next(data, minScore)
	if r.Err != nil {
		return persondetect.DetectResult{}, r.Err
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return persondetect.DetectResult{}, fmt.Errorf("decode frame: %w", err)
	}
	boxes := make([]persondetect.Box, 0, len(r.Boxes))
	for _, b := range r.Boxes {
		if b.Score >= minScore {
			boxes = append(boxes, b)
		}
	}
	return persondetect.DetectResult{Boxes: boxes, Width: cfg.Width, Height: cfg.Height, WallMS: FakeWallMS, CPUMS: FakeCPUMS}, nil
}

// Info returns the configured detector info.
func (d *Detector) Info() persondetect.DetectorInfo { return d.info }

// Close marks the detector closed.
func (d *Detector) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	return nil
}

// Closed reports whether Close was called.
func (d *Detector) Closed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closed
}

// Factory replaces Deps.NewDetector. Each call reports the configured setup phases,
// waits for the gate (when set), then fails with the configured error or returns a
// new Detector that shares Script.
type Factory struct {
	Script *Script

	mu      sync.Mutex
	info    persondetect.DetectorInfo
	err     error
	phases  []persondetect.SetupProgress
	gate    chan struct{}
	created []*Detector
	options []persondetect.DetectorOptions
}

// NewFactory returns a factory whose detectors report info.
func NewFactory(info persondetect.DetectorInfo) *Factory {
	return &Factory{Script: &Script{}, info: info}
}

// SetInfo changes the info of detectors created from now on.
func (f *Factory) SetInfo(info persondetect.DetectorInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.info = info
}

// SetError makes the following setups fail with err (nil: succeed).
func (f *Factory) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// SetPhases sets the progress reported by every following setup.
func (f *Factory) SetPhases(phases ...persondetect.SetupProgress) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.phases = append([]persondetect.SetupProgress(nil), phases...)
}

// SetGate makes following setups wait until gate is closed (nil: no wait).
func (f *Factory) SetGate(gate chan struct{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gate = gate
}

// New implements Deps.NewDetector.
func (f *Factory) New(ctx context.Context, opts persondetect.DetectorOptions) (persondetect.Detector, error) {
	f.mu.Lock()
	f.options = append(f.options, opts)
	phases := append([]persondetect.SetupProgress(nil), f.phases...)
	gate := f.gate
	f.mu.Unlock()
	for _, phase := range phases {
		if opts.OnProgress != nil {
			opts.OnProgress(phase)
		}
	}
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	det := &Detector{script: f.Script, info: f.info}
	f.created = append(f.created, det)
	return det, nil
}

// Calls is the number of setups started.
func (f *Factory) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.options)
}

// Created returns the detectors handed out, oldest first.
func (f *Factory) Created() []*Detector {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*Detector(nil), f.created...)
}

// LastOptions returns the options of the latest setup.
func (f *Factory) LastOptions() persondetect.DetectorOptions {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.options) == 0 {
		return persondetect.DetectorOptions{}
	}
	return f.options[len(f.options)-1]
}

// Frames is a scripted persondetect.FrameSource.
type Frames struct {
	mu    sync.Mutex
	data  map[string][]byte
	errs  map[string]error
	calls map[string][]time.Time
}

// NewFrames returns an empty frame source.
func NewFrames() *Frames {
	return &Frames{data: map[string][]byte{}, errs: map[string]error{}, calls: map[string][]time.Time{}}
}

// Set sets the frame a camera returns.
func (f *Frames) Set(cameraID string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[cameraID] = append([]byte(nil), data...)
}

// Fail makes a camera's requests fail with err (nil: succeed again).
func (f *Frames) Fail(cameraID string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err == nil {
		delete(f.errs, cameraID)
		return
	}
	f.errs[cameraID] = err
}

// Frame implements persondetect.FrameSource.
func (f *Frames) Frame(_ context.Context, cameraID string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[cameraID] = append(f.calls[cameraID], time.Now())
	if err := f.errs[cameraID]; err != nil {
		return nil, err
	}
	data, ok := f.data[cameraID]
	if !ok {
		return nil, fmt.Errorf("camera %q has no frame", cameraID)
	}
	return append([]byte(nil), data...), nil
}

// Calls is the number of frame requests for a camera.
func (f *Frames) Calls(cameraID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls[cameraID])
}

// CallTimes returns the wall-clock times of a camera's frame requests, oldest first.
func (f *Frames) CallTimes(cameraID string) []time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]time.Time(nil), f.calls[cameraID]...)
}

// Clock is a settable time source for Deps.Now.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// NewClock starts the clock at start.
func NewClock(start time.Time) *Clock { return &Clock{now: start} }

// Now returns the current fake time.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward and returns the new time.
func (c *Clock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	return c.now
}

// Set sets the clock.
func (c *Clock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// Dispatcher records AlertEvents and reports a configurable result per action
// (default sent), synchronously.
type Dispatcher struct {
	mu      sync.Mutex
	events  []persondetect.AlertEvent
	results map[string]persondetect.AlertResult
}

// NewDispatcher returns a dispatcher that reports every fired action as sent.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{results: map[string]persondetect.AlertResult{}}
}

// SetResult sets the result reported for an action.
func (d *Dispatcher) SetResult(action string, result persondetect.AlertResult) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.results[action] = result
}

// Dispatch implements persondetect.AlertDispatcher.
func (d *Dispatcher) Dispatch(ev persondetect.AlertEvent, report func(string, persondetect.AlertResult)) {
	d.mu.Lock()
	d.events = append(d.events, ev)
	results := make(map[string]persondetect.AlertResult, len(d.results))
	for action, result := range d.results {
		results[action] = result
	}
	d.mu.Unlock()
	for _, action := range ev.Fire.Fired() {
		result, ok := results[action]
		if !ok {
			result = persondetect.AlertResult{Status: persondetect.AlertSent}
		}
		report(action, result)
	}
}

// Events returns the dispatched events, oldest first.
func (d *Dispatcher) Events() []persondetect.AlertEvent {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]persondetect.AlertEvent(nil), d.events...)
}

// StopOnCleanup shuts svc down when the test ends, so no background loop of the
// service outlives its test: the package keeps test seams in package-level
// variables, and a loop that still runs while the next test replaces one is a data
// race. Register it after the service's data directory (t.TempDir), so the
// database is closed before the directory is removed. A Shutdown that does not
// finish within 5 s fails the test.
func StopOnCleanup(t testing.TB, svc *persondetect.Service) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := svc.Shutdown(ctx); err != nil {
			t.Errorf("service shutdown: %v", err)
		}
	})
}

// WaitFor polls cond every 5 ms until it holds; it fails the test after timeout.
func WaitFor(t testing.TB, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// WaitDetector waits until the service's detector reaches state.
func WaitDetector(t testing.TB, svc *persondetect.Service, state string) {
	t.Helper()
	WaitFor(t, 5*time.Second, "detector state "+state, func() bool {
		return svc.Status().Detector.State == state
	})
}
