package chibisensei

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadGGUFInfo(t *testing.T) {
	put := func(b *bytes.Buffer, v any) { _ = binary.Write(b, binary.LittleEndian, v) }
	str := func(b *bytes.Buffer, s string) { put(b, uint64(len(s))); b.WriteString(s) }
	head := func(version uint32, tensors, kvs uint64) *bytes.Buffer {
		b := &bytes.Buffer{}
		b.WriteString("GGUF")
		put(b, version)
		put(b, tensors)
		put(b, kvs)
		return b
	}
	patch := func(off int, v any) []byte {
		b := bytes.NewBuffer(append([]byte(nil), fakeGGUF("gemma3")...))
		out := b.Bytes()
		var p bytes.Buffer
		put(&p, v)
		copy(out[off:], p.Bytes())
		return out
	}
	skipped := head(3, 2, 3)
	str(skipped, "general.alignment")
	put(skipped, uint32(4))
	put(skipped, uint32(32))
	str(skipped, "tokenizer.ggml.tokens")
	put(skipped, uint32(9))
	put(skipped, uint32(8))
	put(skipped, uint64(2))
	str(skipped, "a")
	str(skipped, "bc")
	str(skipped, "general.architecture")
	put(skipped, uint32(8))
	str(skipped, "qwen35")
	oversized := head(3, 1, 1)
	put(oversized, uint64(1)<<40)
	notString := head(3, 1, 1)
	str(notString, "general.architecture")
	put(notString, uint32(4))
	put(notString, uint32(7))
	nested := head(3, 1, 1)
	str(nested, "x")
	put(nested, uint32(9))
	put(nested, uint32(9))
	put(nested, uint64(1))
	cases := []struct {
		name string
		body []byte
		want GGUFInfo
		err  string
	}{
		{"fake writer", fakeGGUF("gemma3"), GGUFInfo{3, 1, "gemma3"}, ""},
		{"skips other keys", skipped.Bytes(), GGUFInfo{3, 2, "qwen35"}, ""},
		{"wrong magic", append([]byte("GGML"), fakeGGUF("gemma3")[4:]...), GGUFInfo{}, "wrong magic"},
		{"version 2", patch(4, uint32(2)), GGUFInfo{}, "version 2"},
		{"zero tensors", patch(8, uint64(0)), GGUFInfo{}, "tensor count 0"},
		{"oversized string", oversized.Bytes(), GGUFInfo{}, "exceeds the limit"},
		{"truncated", fakeGGUF("gemma3")[:30], GGUFInfo{}, "truncated"},
		{"architecture missing", head(3, 1, 0).Bytes(), GGUFInfo{}, "is missing"},
		{"architecture not a string", notString.Bytes(), GGUFInfo{}, "is not a string"},
		{"nested array", nested.Bytes(), GGUFInfo{}, "nested arrays"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := readGGUF(bytes.NewReader(c.body))
			if c.err == "" {
				if err != nil || got != c.want {
					t.Fatalf("got %+v %v, want %+v", got, err, c.want)
				}
				return
			}
			if !errors.Is(err, ErrGGUFInvalid) || !strings.Contains(err.Error(), c.err) {
				t.Fatalf("err = %v, want %q", err, c.err)
			}
		})
	}
	p := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(p, fakeGGUF("lfm2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadGGUFInfo(p); err != nil || got.Architecture != "lfm2" {
		t.Fatalf("file: %+v %v", got, err)
	}
}
