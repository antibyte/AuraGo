package bluetooth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// errHeadsetNotInPipeWire marks the short gap between a BlueZ connection and
// PipeWire creating the device; activation waits instead of failing.
var errHeadsetNotInPipeWire = errors.New("PipeWire does not know the headset yet")

const (
	// HeadsetReady means the headset is in an HFP profile and records.
	HeadsetReady = "ready"
	// HeadsetLost means the headset carries no audio right now.
	HeadsetLost = "lost"
	// HeadsetError reports a lasting problem; Code holds the error code.
	HeadsetError = "error"

	HeadsetInputRate  = 16000
	HeadsetOutputRate = 24000
	// HeadsetFrameBytes is one Live Speech VAD frame: 512 s16le samples.
	HeadsetFrameBytes = 1024

	headsetStreamReply    = 0
	headsetStreamProgress = 1
	// headsetQueueLimit caps queued playback per stream at 2 s of s16le mono.
	headsetQueueLimit = HeadsetOutputRate * 2 * 2

	headsetPollInterval  = 250 * time.Millisecond
	headsetReadyTimeout  = 5 * time.Second
	headsetRetryInterval = 2 * time.Second
)

// HeadsetEvent tells the Live Speech bridge whether the headset carries audio.
type HeadsetEvent struct {
	Type string `json:"type"`
	Code string `json:"code,omitempty"`
}

type headsetLinkConfig struct {
	address       string
	runner        headsetRunner
	lookup        func(context.Context) (Device, bool)
	changes       <-chan Change
	release       func()
	logger        *slog.Logger
	pollInterval  time.Duration
	readyTimeout  time.Duration
	retryInterval time.Duration
}

// HeadsetLink carries Live Speech audio through one server-side Bluetooth
// headset. It switches the headset to its HFP profile, records the microphone
// with pw-record and plays browser audio with pw-play. It follows the
// connection: a lost device emits HeadsetLost, a reconnect HeadsetReady.
// Close always restores the profile that was active before.
type HeadsetLink struct {
	cfg          headsetLinkConfig
	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}
	frames       chan []byte
	events       chan HeadsetEvent
	recorderDone chan struct{}
	closeOnce    sync.Once

	mu        sync.Mutex
	recorder  headsetProcess
	sink      string
	players   map[int]*headsetPlayer
	restore   string
	state     string
	errorSent string
}

func startHeadsetLink(parent context.Context, cfg headsetLinkConfig) *HeadsetLink {
	if cfg.pollInterval <= 0 {
		cfg.pollInterval = headsetPollInterval
	}
	if cfg.readyTimeout <= 0 {
		cfg.readyTimeout = headsetReadyTimeout
	}
	if cfg.retryInterval <= 0 {
		cfg.retryInterval = headsetRetryInterval
	}
	if cfg.logger == nil {
		cfg.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	ctx, cancel := context.WithCancel(parent)
	link := &HeadsetLink{
		cfg:          cfg,
		ctx:          ctx,
		cancel:       cancel,
		done:         make(chan struct{}),
		frames:       make(chan []byte, 64),
		events:       make(chan HeadsetEvent, 16),
		recorderDone: make(chan struct{}, 1),
		players:      map[int]*headsetPlayer{},
	}
	go link.run()
	return link
}

// Frames delivers microphone audio: HeadsetFrameBytes of s16le mono PCM at
// HeadsetInputRate per frame. Frames are dropped when the reader falls behind.
func (l *HeadsetLink) Frames() <-chan []byte { return l.frames }

// Events delivers HeadsetReady, HeadsetLost and HeadsetError changes.
func (l *HeadsetLink) Events() <-chan HeadsetEvent { return l.events }

// Close stops audio, restores the previous profile and frees the headset.
func (l *HeadsetLink) Close() error {
	l.closeOnce.Do(func() {
		l.cancel()
		<-l.done
		if l.cfg.release != nil {
			l.cfg.release()
		}
	})
	return nil
}

func (l *HeadsetLink) run() {
	defer close(l.done)
	changes := l.cfg.changes
	l.reconcile()
	ticker := time.NewTicker(l.cfg.retryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-l.ctx.Done():
			l.shutdown()
			return
		case _, ok := <-changes:
			if !ok {
				changes = nil
			}
		case <-l.recorderDone:
		case <-ticker.C:
		}
		l.reconcile()
	}
}

// reconcile compares the BlueZ connection with the running recorder.
func (l *HeadsetLink) reconcile() {
	if l.ctx.Err() != nil {
		return
	}
	device, ok := l.cfg.lookup(l.ctx)
	if !ok || !device.Connected {
		l.deactivate()
		l.mu.Lock()
		l.errorSent = ""
		l.mu.Unlock()
		l.announce(HeadsetLost)
		return
	}
	l.mu.Lock()
	active := l.recorder != nil
	l.mu.Unlock()
	if active {
		return
	}
	// Players of an ended recorder may still target an old sink.
	l.deactivate()
	if err := l.activate(); err != nil {
		l.deactivate()
		l.announce(HeadsetLost)
		if l.ctx.Err() != nil {
			return
		}
		code := ErrorCode(err)
		if code != ErrorHeadsetProfileUnavailable {
			code = ErrorHeadsetAudioUnavailable
		}
		l.announceError(code, err)
		return
	}
	l.announce(HeadsetReady)
}

func (l *HeadsetLink) activate() error {
	deadline := time.Now().Add(l.cfg.readyTimeout)
	graph, err := l.awaitGraph(deadline, func(headsetGraph) bool { return true })
	if err != nil {
		return err
	}
	profile, ok := graph.bestHeadsetProfile()
	if !ok {
		return codedError(ErrorHeadsetProfileUnavailable, "The headset offers no microphone profile.", nil)
	}
	if !isHeadsetProfile(graph.Current.Name) {
		l.mu.Lock()
		if l.restore == "" {
			l.restore = graph.Current.Name
		}
		l.mu.Unlock()
		if _, err := l.cfg.runner.Output(l.ctx, "wpctl", "set-profile", strconv.Itoa(graph.DeviceID), strconv.Itoa(profile.Index)); err != nil {
			return codedError(ErrorHeadsetProfileUnavailable, "The headset did not switch to its microphone profile.", err)
		}
	}
	graph, err = l.awaitGraph(deadline, func(g headsetGraph) bool {
		return isHeadsetProfile(g.Current.Name) && g.Source != "" && g.Sink != ""
	})
	if err != nil {
		return err
	}
	// --raw: without it pw-record/pw-play treat "-" as a sound file (libsndfile).
	recorder, err := l.cfg.runner.Pipe(l.ctx, "pw-record", "--raw", "--target", graph.Source,
		"--rate", strconv.Itoa(HeadsetInputRate), "--channels", "1", "--format", "s16", "-")
	if err != nil {
		return codedError(ErrorHeadsetAudioUnavailable, "The headset microphone could not be opened.", err)
	}
	_ = recorder.Stdin().Close()
	l.mu.Lock()
	l.recorder = recorder
	l.sink = graph.Sink
	l.mu.Unlock()
	go l.readMicrophone(recorder)
	l.cfg.logger.Info("[Bluetooth] Live Speech headset ready", "address", l.cfg.address,
		"profile", graph.Current.Name, "source", graph.Source, "sink", graph.Sink)
	return nil
}

// awaitGraph polls PipeWire until ready accepts the headset's graph or the
// deadline passes. A device PipeWire does not know yet is waited for.
func (l *HeadsetLink) awaitGraph(deadline time.Time, ready func(headsetGraph) bool) (headsetGraph, error) {
	for {
		graph, err := l.graph(l.ctx)
		if err == nil && ready(graph) {
			return graph, nil
		}
		if err != nil && !errors.Is(err, errHeadsetNotInPipeWire) {
			return headsetGraph{}, err
		}
		if time.Now().After(deadline) {
			if err != nil {
				return headsetGraph{}, err
			}
			return headsetGraph{}, codedError(ErrorHeadsetAudioUnavailable, "The headset microphone did not appear in PipeWire.", nil)
		}
		select {
		case <-l.ctx.Done():
			return headsetGraph{}, l.ctx.Err()
		case <-time.After(l.cfg.pollInterval):
		}
	}
}

func (l *HeadsetLink) graph(ctx context.Context) (headsetGraph, error) {
	raw, err := l.cfg.runner.Output(ctx, "pw-dump")
	if err != nil {
		return headsetGraph{}, codedError(ErrorHeadsetAudioUnavailable, "PipeWire is not reachable.", err)
	}
	graph, found, err := parseHeadsetGraph(raw, l.cfg.address)
	if err != nil {
		return headsetGraph{}, codedError(ErrorHeadsetAudioUnavailable, "PipeWire returned unreadable data.", err)
	}
	if !found {
		return headsetGraph{}, codedError(ErrorHeadsetAudioUnavailable, "PipeWire does not know the headset yet.", errHeadsetNotInPipeWire)
	}
	return graph, nil
}

func (l *HeadsetLink) readMicrophone(recorder headsetProcess) {
	reader := recorder.Stdout()
	for {
		frame := make([]byte, HeadsetFrameBytes)
		if _, err := io.ReadFull(reader, frame); err != nil {
			break
		}
		select {
		case l.frames <- frame:
		default: // the browser fell behind; drop instead of delaying speech
		}
	}
	_ = recorder.Wait()
	l.mu.Lock()
	if l.recorder == recorder {
		l.recorder = nil
	}
	l.mu.Unlock()
	select {
	case l.recorderDone <- struct{}{}:
	default:
	}
}

func (l *HeadsetLink) deactivate() {
	l.mu.Lock()
	recorder := l.recorder
	players := l.players
	l.recorder = nil
	l.sink = ""
	l.players = map[int]*headsetPlayer{}
	l.mu.Unlock()
	if recorder != nil {
		_ = recorder.Kill()
	}
	for _, player := range players {
		player.stop()
	}
}

func (l *HeadsetLink) shutdown() {
	l.deactivate()
	l.mu.Lock()
	restore := l.restore
	l.restore = ""
	l.mu.Unlock()
	if restore == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	graph, err := l.graph(ctx)
	if err != nil || graph.Current.Name == restore {
		return
	}
	if profile, ok := graph.profileByName(restore); ok {
		_, _ = l.cfg.runner.Output(ctx, "wpctl", "set-profile", strconv.Itoa(graph.DeviceID), strconv.Itoa(profile.Index))
	}
}

func (l *HeadsetLink) announce(kind string) {
	l.mu.Lock()
	changed := l.state != kind
	l.state = kind
	l.mu.Unlock()
	if changed {
		l.emit(HeadsetEvent{Type: kind})
	}
}

// announceError reports a failure once per connection and logs its cause, so
// a headset that never becomes ready is visible in the panel and server log.
func (l *HeadsetLink) announceError(code string, cause error) {
	l.mu.Lock()
	changed := l.errorSent != code
	l.errorSent = code
	l.mu.Unlock()
	if changed {
		l.cfg.logger.Warn("[Bluetooth] Live Speech headset not ready", "address", l.cfg.address, "code", code, "error", cause)
		l.emit(HeadsetEvent{Type: HeadsetError, Code: code})
	}
}

func (l *HeadsetLink) emit(event HeadsetEvent) {
	select {
	case l.events <- event:
	case <-l.ctx.Done():
	}
}
