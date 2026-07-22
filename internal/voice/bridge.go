package voice

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

var ErrBridgeClosed = errors.New("audio bridge is closed")

// Bridge is a bounded PCM bus. When a producer outruns a consumer, the oldest
// frame is discarded so call latency remains bounded.
type Bridge struct {
	receive chan PCMFrame
	send    chan PCMFrame
	events  chan VoiceEvent
	closed  atomic.Bool
}

func NewBridge(queueFrames int) *Bridge {
	if queueFrames < 1 {
		queueFrames = 1
	}
	return &Bridge{
		receive: make(chan PCMFrame, queueFrames),
		send:    make(chan PCMFrame, queueFrames),
		events:  make(chan VoiceEvent, queueFrames),
	}
}

func (b *Bridge) Receive() <-chan PCMFrame { return b.receive }

func (b *Bridge) Events() <-chan VoiceEvent { return b.events }

func (b *Bridge) Send(ctx context.Context, frame PCMFrame) error {
	if b.closed.Load() {
		return ErrBridgeClosed
	}
	frame.Samples = append([]int16(nil), frame.Samples...)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case b.send <- frame:
		return nil
	default:
		b.dropOldest(b.send, "output_queue_overrun")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case b.send <- frame:
			return nil
		default:
			return nil
		}
	}
}

// PushReceive publishes input captured by the attached media peer.
func (b *Bridge) PushReceive(frame PCMFrame) error {
	if b.closed.Load() {
		return ErrBridgeClosed
	}
	frame.Samples = append([]int16(nil), frame.Samples...)
	select {
	case b.receive <- frame:
		return nil
	default:
		b.dropOldest(b.receive, "input_queue_overrun")
		select {
		case b.receive <- frame:
		default:
		}
		return nil
	}
}

// NextSend returns one frame queued for the attached media peer.
func (b *Bridge) NextSend(ctx context.Context) (PCMFrame, error) {
	if b.closed.Load() {
		return PCMFrame{}, ErrBridgeClosed
	}
	select {
	case <-ctx.Done():
		return PCMFrame{}, ctx.Err()
	case frame := <-b.send:
		return frame, nil
	}
}

func (b *Bridge) FlushOutput() {
	for {
		select {
		case <-b.send:
		default:
			return
		}
	}
}

func (b *Bridge) Close() {
	b.closed.Store(true)
	b.FlushOutput()
}

func (b *Bridge) dropOldest(queue chan PCMFrame, eventType string) {
	select {
	case <-queue:
	default:
	}
	event := VoiceEvent{Type: eventType, Timestamp: time.Now().UTC()}
	select {
	case b.events <- event:
	default:
	}
}
