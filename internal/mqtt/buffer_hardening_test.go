package mqtt

import (
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
	"unsafe"

	"aurago/internal/tools"
)

func TestTruncateUTF8BytesOwnsBoundedLinearPrefix(t *testing.T) {
	for _, tc := range []struct {
		name, value, want string
		limit             int
	}{
		{"unicode boundary", "a€b", "a", 3},
		{"complete rune", "a€b", "a€", 4},
		{"invalid middle", strings.Repeat("a", 1024) + "\xff" + strings.Repeat("b", 2048), strings.Repeat("a", 1024), 2048},
		{"invalid first", "\xffabcdef", "", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := TruncateUTF8Bytes(tc.value, tc.limit)
			if got != tc.want || !utf8.ValidString(got) {
				t.Fatalf("bounded prefix = %q, want %q", got, tc.want)
			}
			if got != "" && unsafe.StringData(got) == unsafe.StringData(tc.value) {
				t.Fatal("shortened payload still retains its source allocation")
			}
		})
	}
}

func TestMessageBufferByteBudgetAndExpiredReferences(t *testing.T) {
	b := newMessageBuffer()
	b.Configure(10, 1, 1024, 12)
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	for _, topic := range []string{"one", "two", "tri"} {
		b.Add(tools.MQTTMessage{Topic: topic, Payload: "123", Timestamp: stamp})
	}
	if b.Bytes() != 12 || b.Len() != 2 || b.Get("", 10)[0].Topic != "two" {
		t.Fatalf("byte eviction: bytes=%d messages=%v", b.Bytes(), b.Get("", 10))
	}
	backing := b.messages[:cap(b.messages)]
	for i := range b.messages {
		b.messages[i].Timestamp = time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)
	}
	if b.Len() != 0 || b.Bytes() != 0 {
		t.Fatal("expired payload bytes remain in the buffer")
	}
	for _, msg := range backing {
		if msg.Payload != "" || msg.Topic != "" {
			t.Fatal("expired backing slice still retains message data")
		}
	}
}

func TestMessageBufferFiltersAndBinaryMetadata(t *testing.T) {
	b := newMessageBuffer()
	for _, topic := range []string{"home/a/temp", "home/b/temp", "$SYS/status"} {
		b.Add(tools.MQTTMessage{Topic: topic, Payload: "ok"})
	}
	if len(b.Get("home/+/temp", 10)) != 2 || len(b.Get("home/#", 10)) != 2 || len(b.Get("#", 10)) != 2 || len(b.Get("$SYS/#", 10)) != 1 {
		t.Fatal("buffer filters do not obey MQTT wildcard and system-topic rules")
	}
	m := b.Add(tools.MQTTMessage{Topic: "binary", Payload: "a\xffb"})
	if !m.PayloadBinary || m.Payload != "" || m.PayloadBytes != 3 || m.PayloadTruncated {
		t.Fatalf("binary metadata = %+v", m)
	}
}

func BenchmarkTruncateMQTTInvalidUTF8(b *testing.B) {
	for _, size := range []int{16 << 10, 256 << 10, 1 << 20} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			input := strings.Repeat("a", size/2) + "\xff" + strings.Repeat("b", size)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				TruncateUTF8Bytes(input, size)
			}
		})
	}
}

func TestMQTTStatsSeparateTruncationAndBinaryOmission(t *testing.T) {
	buffer.Configure(10, 0, 4, 0)
	t.Cleanup(func() { buffer.Configure(0, 0, 0, 0) })
	before := RuntimeStats()
	storeMQTTMessage(tools.MQTTMessage{Topic: "stats/text", Payload: "abcdef"})
	after := RuntimeStats()
	if after["truncated_payload_messages"].(uint64) != before["truncated_payload_messages"].(uint64)+1 || after["dropped_payload_messages"] != before["dropped_payload_messages"] {
		t.Fatal("text shortening was counted as a discarded payload")
	}
	storeMQTTMessage(tools.MQTTMessage{Topic: "stats/binary", Payload: "\xff"})
	final := RuntimeStats()
	if final["dropped_payload_messages"].(uint64) != after["dropped_payload_messages"].(uint64)+1 || final["truncated_payload_messages"] != after["truncated_payload_messages"] {
		t.Fatal("binary omission was counted as text truncation")
	}
}
