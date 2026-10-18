package chunking

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkRecursiveOversize(b *testing.B) {
	for _, size := range []int{1 << 20, 8 << 20} {
		b.Run(fmt.Sprintf("%dMiB", size>>20), func(b *testing.B) {
			text := strings.Repeat("A long paragraph with Unicode ä and no newlines. ", size/49+1)[:size]
			b.SetBytes(int64(len(text)))
			b.ReportAllocs()
			for b.Loop() {
				chunks, err := ChunkText(text, DefaultOptions())
				if err != nil || len(chunks) == 0 {
					b.Fatalf("chunks=%d err=%v", len(chunks), err)
				}
			}
		})
	}
}
