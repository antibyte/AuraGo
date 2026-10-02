package bluetooth

import (
	"io"
	"strconv"
	"sync"
	"sync/atomic"
)

// headsetPlayer feeds one pw-play process from a bounded queue so a slow
// headset never blocks the Live Speech bridge.
type headsetPlayer struct {
	process headsetProcess
	queue   chan []byte
	queued  atomic.Int64
	stopped chan struct{}
	once    sync.Once
}

func newHeadsetPlayer(process headsetProcess) *headsetPlayer {
	player := &headsetPlayer{process: process, queue: make(chan []byte, 256), stopped: make(chan struct{})}
	go func() {
		_, _ = io.Copy(io.Discard, process.Stdout())
		_ = process.Wait()
		player.stop()
	}()
	go player.pump()
	return player
}

func (p *headsetPlayer) pump() {
	stdin := p.process.Stdin()
	defer stdin.Close()
	for {
		select {
		case <-p.stopped:
			return
		case chunk := <-p.queue:
			p.queued.Add(-int64(len(chunk)))
			if _, err := stdin.Write(chunk); err != nil {
				p.stop()
				return
			}
		}
	}
}

// offer queues a chunk unless more than headsetQueueLimit bytes are waiting;
// late audio is dropped instead of delaying the conversation.
func (p *headsetPlayer) offer(chunk []byte) {
	if p.queued.Load()+int64(len(chunk)) > headsetQueueLimit {
		return
	}
	select {
	case p.queue <- chunk:
		p.queued.Add(int64(len(chunk)))
	case <-p.stopped:
	default:
	}
}

func (p *headsetPlayer) alive() bool {
	select {
	case <-p.stopped:
		return false
	default:
		return true
	}
}

func (p *headsetPlayer) stop() {
	p.once.Do(func() {
		close(p.stopped)
		_ = p.process.Kill()
	})
}

// Write queues browser audio (s16le mono at HeadsetOutputRate) for stream 0
// (replies) or 1 (progress narration). While the headset is lost the audio
// is dropped because the browser plays it itself.
func (l *HeadsetLink) Write(stream int, pcm []byte) error {
	if stream != headsetStreamReply && stream != headsetStreamProgress {
		return codedError(ErrorInvalidArgument, "Unknown headset output stream.", nil)
	}
	if len(pcm) == 0 {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.recorder == nil || l.sink == "" {
		return nil
	}
	player := l.players[stream]
	if player == nil || !player.alive() {
		process, err := l.cfg.runner.Pipe(l.ctx, "pw-play", "--target", l.sink,
			"--rate", strconv.Itoa(HeadsetOutputRate), "--channels", "1", "--format", "s16", "-")
		if err != nil {
			return codedError(ErrorHeadsetAudioUnavailable, "The headset speaker could not be opened.", err)
		}
		player = newHeadsetPlayer(process)
		l.players[stream] = player
	}
	player.offer(append([]byte(nil), pcm...))
	return nil
}

// Flush drops queued and playing audio of one stream, e.g. on barge-in.
func (l *HeadsetLink) Flush(stream int) {
	l.mu.Lock()
	player := l.players[stream]
	delete(l.players, stream)
	l.mu.Unlock()
	if player != nil {
		player.stop()
	}
}
