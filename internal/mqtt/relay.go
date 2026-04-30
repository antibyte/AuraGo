package mqtt

import (
	"sync"
	"sync/atomic"

	"aurago/internal/tools"
)

const defaultRelayQueueSize = 100

var relayWorker = struct {
	mu     sync.Mutex
	queue  chan tools.MQTTMessage
	stopCh chan struct{}
}{}

func startRelayWorker(queueSize int) {
	if queueSize <= 0 {
		queueSize = defaultRelayQueueSize
	}
	relayWorker.mu.Lock()
	defer relayWorker.mu.Unlock()
	stopRelayWorkerLocked()

	relayWorker.queue = make(chan tools.MQTTMessage, queueSize)
	relayWorker.stopCh = make(chan struct{})
	queue := relayWorker.queue
	stopCh := relayWorker.stopCh

	go func() {
		for {
			select {
			case msg := <-queue:
				if RelayCallback != nil {
					RelayCallback(msg.Topic, msg.Payload)
				}
			case <-stopCh:
				return
			}
		}
	}()
}

func stopRelayWorker() {
	relayWorker.mu.Lock()
	defer relayWorker.mu.Unlock()
	stopRelayWorkerLocked()
}

func stopRelayWorkerLocked() {
	if relayWorker.stopCh != nil {
		close(relayWorker.stopCh)
	}
	relayWorker.queue = nil
	relayWorker.stopCh = nil
}

func enqueueRelayMessage(msg tools.MQTTMessage) {
	relayWorker.mu.Lock()
	queue := relayWorker.queue
	relayWorker.mu.Unlock()
	if queue == nil {
		if RelayCallback != nil {
			go RelayCallback(msg.Topic, msg.Payload)
		}
		return
	}
	select {
	case queue <- msg:
	default:
		atomic.AddUint64(&stats.droppedRelayMessages, 1)
		if logger != nil {
			logger.Warn("[MQTT] Relay queue full; dropping message", "topic", msg.Topic)
		}
	}
}
