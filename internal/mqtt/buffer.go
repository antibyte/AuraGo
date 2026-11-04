package mqtt

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/tools"
)

const (
	defaultMaxBufferMessages = 500
	defaultMaxPayloadBytes   = 256 << 10
	defaultMaxBufferBytes    = 16 << 20
)

type messageBuffer struct {
	mu              sync.RWMutex
	messages        []tools.MQTTMessage
	maxMessages     int
	maxAge          time.Duration
	maxPayloadBytes int
	maxBytes        int
	totalBytes      int
}

func newMessageBuffer() *messageBuffer {
	return &messageBuffer{
		maxMessages:     defaultMaxBufferMessages,
		maxPayloadBytes: defaultMaxPayloadBytes,
		maxBytes:        defaultMaxBufferBytes,
	}
}

func (b *messageBuffer) Configure(maxMessages, maxAgeHours, maxPayloadBytes, maxBytes int) {
	if maxMessages <= 0 {
		maxMessages = defaultMaxBufferMessages
	}
	if maxPayloadBytes <= 0 {
		maxPayloadBytes = defaultMaxPayloadBytes
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxBufferBytes
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.maxMessages = maxMessages
	b.maxPayloadBytes = maxPayloadBytes
	b.maxBytes = maxBytes
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
	b.totalBytes += len(msg.Topic) + len(msg.Payload)
	b.trimLocked()
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
		if topic == "" || topicMatches(topic, b.messages[i].Topic) {
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
	if msg.PayloadBytes == 0 {
		msg.PayloadBytes = len(msg.Payload)
	}
	if msg.PayloadBinary || !utf8.ValidString(msg.Payload) || strings.IndexByte(msg.Payload, 0) >= 0 {
		msg.PayloadBinary = true
		msg.Payload = ""
		return msg
	}
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
		b.totalBytes = 0
		for _, msg := range b.messages {
			stamp, err := time.Parse(time.RFC3339, msg.Timestamp)
			if err != nil {
				stamp, err = time.Parse(time.RFC3339Nano, msg.Timestamp)
			}
			if err != nil || !stamp.Before(cutoff) {
				kept = append(kept, msg)
				b.totalBytes += len(msg.Topic) + len(msg.Payload)
			}
		}
		clear(b.messages[len(kept):])
		b.messages = kept
	}
	if b.maxMessages <= 0 {
		b.maxMessages = defaultMaxBufferMessages
	}
	b.trimLocked()
}

func (b *messageBuffer) trimLocked() {
	if b.maxBytes <= 0 {
		b.maxBytes = defaultMaxBufferBytes
	}
	removed := 0
	for len(b.messages)-removed > 0 && (len(b.messages)-removed > b.maxMessages || b.totalBytes > b.maxBytes) {
		msg := b.messages[removed]
		b.totalBytes -= len(msg.Topic) + len(msg.Payload)
		removed++
	}
	clear(b.messages[:removed])
	b.messages = b.messages[removed:]
	if len(b.messages) == 0 {
		b.messages = nil
	}
}

// Bytes reports retained topic and payload bytes after applying expiry.
func (b *messageBuffer) Bytes() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked(time.Now().UTC())
	return b.totalBytes
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	end := 0
	for end < maxBytes {
		r, size := utf8.DecodeRuneInString(value[end:])
		if r == utf8.RuneError && size == 1 || end+size > maxBytes {
			break
		}
		end += size
	}
	// Own the shortened allocation instead of retaining the original payload.
	return strings.Clone(value[:end])
}

// TruncateUTF8Bytes shortens value to at most maxBytes without splitting a UTF-8 sequence.
func TruncateUTF8Bytes(value string, maxBytes int) string {
	return truncateUTF8Bytes(value, maxBytes)
}
