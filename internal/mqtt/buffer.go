package mqtt

import (
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/tools"
)

const (
	defaultMaxBufferMessages = 500
	defaultMaxPayloadBytes   = 256 << 10
)

type messageBuffer struct {
	mu              sync.RWMutex
	messages        []tools.MQTTMessage
	maxMessages     int
	maxAge          time.Duration
	maxPayloadBytes int
}

func newMessageBuffer() *messageBuffer {
	return &messageBuffer{
		maxMessages:     defaultMaxBufferMessages,
		maxPayloadBytes: defaultMaxPayloadBytes,
	}
}

func (b *messageBuffer) Configure(maxMessages, maxAgeHours, maxPayloadBytes int) {
	if maxMessages <= 0 {
		maxMessages = defaultMaxBufferMessages
	}
	if maxPayloadBytes <= 0 {
		maxPayloadBytes = defaultMaxPayloadBytes
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.maxMessages = maxMessages
	b.maxPayloadBytes = maxPayloadBytes
	if maxAgeHours > 0 {
		b.maxAge = time.Duration(maxAgeHours) * time.Hour
	} else {
		b.maxAge = 0
	}
	b.pruneLocked(time.Now().UTC())
}

func (b *messageBuffer) Add(msg tools.MQTTMessage) tools.MQTTMessage {
	now := time.Now().UTC()
	msg = b.normalizeMessage(msg)

	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked(now)
	b.messages = append(b.messages, msg)
	if len(b.messages) > b.maxMessages {
		b.messages = b.messages[len(b.messages)-b.maxMessages:]
	}
	return msg
}

func (b *messageBuffer) Get(topic string, limit int) []tools.MQTTMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked(time.Now().UTC())

	if limit <= 0 {
		limit = 50
	}

	result := make([]tools.MQTTMessage, 0, min(limit, len(b.messages)))
	for i := len(b.messages) - 1; i >= 0 && len(result) < limit; i-- {
		if topic == "" || topic == "#" || b.messages[i].Topic == topic {
			result = append(result, b.messages[i])
		}
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func (b *messageBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked(time.Now().UTC())
	return len(b.messages)
}

func (b *messageBuffer) normalizeMessage(msg tools.MQTTMessage) tools.MQTTMessage {
	msg.PayloadBytes = len([]byte(msg.Payload))
	maxPayloadBytes := b.currentMaxPayloadBytes()
	if maxPayloadBytes > 0 && msg.PayloadBytes > maxPayloadBytes {
		msg.Payload = truncateUTF8Bytes(msg.Payload, maxPayloadBytes)
		msg.PayloadTruncated = true
	}
	return msg
}

func (b *messageBuffer) currentMaxPayloadBytes() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.maxPayloadBytes
}

func (b *messageBuffer) pruneLocked(now time.Time) {
	if b.maxAge > 0 {
		cutoff := now.Add(-b.maxAge)
		kept := b.messages[:0]
		for _, msg := range b.messages {
			stamp, err := time.Parse(time.RFC3339, msg.Timestamp)
			if err != nil {
				stamp, err = time.Parse(time.RFC3339Nano, msg.Timestamp)
			}
			if err != nil || !stamp.Before(cutoff) {
				kept = append(kept, msg)
			}
		}
		b.messages = kept
	}
	if b.maxMessages <= 0 {
		b.maxMessages = defaultMaxBufferMessages
	}
	if len(b.messages) > b.maxMessages {
		b.messages = b.messages[len(b.messages)-b.maxMessages:]
	}
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	truncated := value[:maxBytes]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}
