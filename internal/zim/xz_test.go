package zim

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"math"
	"math/rand/v2"
	"runtime"
	"testing"

	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"

	"aurago/internal/zim/zimtest"
)

// The synthetic writer (ulikunitz) only ever declares an 8 MiB dictionary, so
// these tests craft streams that declare what liblzma presets do (preset 9 is
// 64 MiB) up to the 4 GiB maximum, to exercise the dictionary cap.

const (
	xzDictCode32MiB = 26
	xzDictCode64MiB = 28 // liblzma preset 9
	xzDictCodeMax   = 40 // 4 GiB - 1
)

// xzStream encodes payload as a single-block xz stream.
func xzStream(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := xz.WriterConfig{DictCap: 1 << 20}.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// rewriteXZBlockHeader applies mutate to the first block header (without its
// CRC32) of an xz stream and re-seals the header with a fresh CRC32. mutate
// must not change the header length.
func rewriteXZBlockHeader(t *testing.T, stream []byte, mutate func(body []byte)) []byte {
	t.Helper()
	out := append([]byte(nil), stream...)
	hlen := (int(out[xzStreamHeaderLen]) + 1) * 4
	body := out[xzStreamHeaderLen : xzStreamHeaderLen+hlen-4]
	mutate(body)
	binary.LittleEndian.PutUint32(out[xzStreamHeaderLen+hlen-4:], crc32.ChecksumIEEE(body))
	return out
}

// xzWithDictCode returns stream with its declared LZMA2 dictionary replaced.
// The LZMA2 chunks do not repeat the dictionary size, so the stream stays valid.
func xzWithDictCode(t *testing.T, stream []byte, code byte) []byte {
	t.Helper()
	return rewriteXZBlockHeader(t, stream, func(body []byte) {
		if body[1] != 0 || body[2] != xzFilterLZMA2 || body[3] != 1 {
			t.Fatalf("unexpected xz block header layout % x", body)
		}
		body[4] = code
	})
}

// allocatedBy reports how many heap bytes fn allocates in total.
func allocatedBy(fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// longRangeBlobs returns blobs where the third repeats the first, so the
// encoder emits a match reaching back more than 64 KiB.
func longRangeBlobs() [][]byte {
	rng := rand.New(rand.NewPCG(1, 2))
	chunk := make([]byte, 64<<10)
	for i := range chunk {
		chunk[i] = byte(rng.Uint32())
	}
	return [][]byte{chunk, []byte("separator"), chunk}
}

func TestXZDeclaredHugeDictionaryStillDecodesValidData(t *testing.T) {
	blobs := longRangeBlobs()
	body := zimtest.ClusterBody(blobs, false)
	base := xzStream(t, body)
	for _, tc := range []struct {
		name  string
		code  byte
		limit int64
	}{
		{"32 MiB declared, 1 MiB cap", xzDictCode32MiB, 1 << 20},
		{"64 MiB declared, 1 MiB cap", xzDictCode64MiB, 1 << 20},
		{"4 GiB declared, 1 MiB cap", xzDictCodeMax, 1 << 20},
		{"64 MiB declared, cap exactly the cluster size", xzDictCode64MiB, int64(len(body))},
		{"64 MiB declared, production cap", xzDictCode64MiB, maxClusterBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := xzWithDictCode(t, base, tc.code)
			cd, err := decodeCluster(bytes.NewReader(stream), clusterInfo{comp: compXZ}, tc.limit, 10)
			if err != nil {
				t.Fatal(err)
			}
			if cd.count != uint64(len(blobs)) {
				t.Fatalf("count = %d, want %d", cd.count, len(blobs))
			}
			for i, want := range blobs {
				got, err := cd.blob(uint32(i))
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("blob %d differs (len %d, want %d, err %v)", i, len(got), len(want), err)
				}
			}
		})
	}
}

func TestXZDeclaredHugeDictionaryDoesNotAllocateIt(t *testing.T) {
	body := zimtest.ClusterBody(longRangeBlobs(), false)
	stream := xzWithDictCode(t, xzStream(t, body), xzDictCode64MiB)
	var err error
	got := allocatedBy(func() {
		_, err = decodeCluster(bytes.NewReader(stream), clusterInfo{comp: compXZ}, 1<<20, 10)
	})
	if err != nil {
		t.Fatal(err)
	}
	// Dictionary (1 MiB cap) + cluster bytes + read buffer; far below the 64 MiB declared.
	if got > 8<<20 {
		t.Fatalf("decoding allocated %d bytes, want at most 8 MiB for a 1 MiB cap (64 MiB dictionary declared)", got)
	}
}

func TestXZContentLargerThanCapIsRejectedWithinCap(t *testing.T) {
	t.Run("real content above the limit", func(t *testing.T) {
		// 4 MiB of zeros compresses to a few KiB; the cluster is announced and
		// really is 4 MiB, but the limit is 1 MiB.
		body := zimtest.ClusterBody([][]byte{make([]byte, 4<<20)}, false)
		stream := xzWithDictCode(t, xzStream(t, body), xzDictCode64MiB)
		var err error
		got := allocatedBy(func() {
			_, err = decodeCluster(bytes.NewReader(stream), clusterInfo{comp: compXZ}, 1<<20, 10)
		})
		wantErr(t, err, ErrUnsupported)
		if got > 8<<20 {
			t.Fatalf("decoding allocated %d bytes, want at most 8 MiB for a 1 MiB cap", got)
		}
	})
	t.Run("announced size above the production cap", func(t *testing.T) {
		// Only the offset table is present: validation fails before the 40 MiB
		// announced by it is allocated, and the dictionary stops at the 32 MiB cap.
		table := make([]byte, 8)
		binary.LittleEndian.PutUint32(table[0:], 8)
		binary.LittleEndian.PutUint32(table[4:], 40<<20)
		stream := xzWithDictCode(t, xzStream(t, table), xzDictCode64MiB)
		var err error
		got := allocatedBy(func() {
			_, err = decodeCluster(bytes.NewReader(stream), clusterInfo{comp: compXZ}, maxClusterBytes, 10)
		})
		wantErr(t, err, ErrUnsupported)
		// 32 MiB dictionary at most, not the declared 64 MiB or the announced 40 MiB on top.
		if got > maxClusterBytes+8<<20 {
			t.Fatalf("decoding allocated %d bytes, want at most %d (32 MiB cap plus slack)", got, maxClusterBytes+8<<20)
		}
	})
}

// withStreamHeader returns stream with its 2 stream-flag bytes replaced and
// the stream header checksum recomputed.
func withStreamHeader(stream []byte, flags [2]byte) []byte {
	out := append([]byte(nil), stream...)
	out[6], out[7] = flags[0], flags[1]
	binary.LittleEndian.PutUint32(out[8:], crc32.ChecksumIEEE(out[6:8]))
	return out
}

// withBlockHeader replaces the first block header of stream with one built
// from fields (everything after the size byte), zero-padded and sealed.
func withBlockHeader(stream, fields []byte) []byte {
	oldLen := (int(stream[xzStreamHeaderLen]) + 1) * 4
	hdr := make([]byte, 1+len(fields))
	copy(hdr[1:], fields)
	for (len(hdr)+4)%4 != 0 || len(hdr)+4 < 8 {
		hdr = append(hdr, 0)
	}
	hdr[0] = byte((len(hdr)+4)/4 - 1)
	hdr = binary.LittleEndian.AppendUint32(hdr, crc32.ChecksumIEEE(hdr))
	out := append([]byte(nil), stream[:xzStreamHeaderLen]...)
	out = append(out, hdr...)
	return append(out, stream[xzStreamHeaderLen+oldLen:]...)
}

func TestNewXZReaderRejectsMalformedHeaders(t *testing.T) {
	valid := xzStream(t, zimtest.ClusterBody([][]byte{[]byte("x")}, false))
	mutate := func(fn func(body []byte)) []byte { return rewriteXZBlockHeader(t, valid, fn) }
	// A 10-byte multibyte integer is one byte longer than the xz format allows.
	tooLong := []byte{0xA1, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x00}
	cases := []struct {
		name   string
		stream []byte
		want   error
	}{
		{"empty input", nil, io.EOF},
		{"truncated stream header", valid[:8], io.ErrUnexpectedEOF},
		{"wrong magic", append([]byte("NOTXZ-NOTXZ-"), valid[xzStreamHeaderLen:]...), ErrCorrupt},
		{"stream flags byte 0", withStreamHeader(valid, [2]byte{1, valid[7]}), ErrCorrupt},
		{"stream flags reserved check bits", withStreamHeader(valid, [2]byte{0, valid[7] | 0x10}), ErrCorrupt},
		{"stream header checksum", func() []byte {
			s := append([]byte(nil), valid...)
			s[8] ^= 0xFF
			return s
		}(), ErrCorrupt},
		{"index instead of block", append(append([]byte(nil), valid[:xzStreamHeaderLen]...), 0), ErrCorrupt},
		{"block header checksum", func() []byte {
			s := append([]byte(nil), valid...)
			hlen := (int(s[xzStreamHeaderLen]) + 1) * 4
			s[xzStreamHeaderLen+hlen-1] ^= 0xFF
			return s
		}(), ErrCorrupt},
		{"reserved flags", mutate(func(b []byte) { b[1] |= 0x04 }), ErrCorrupt},
		{"filter chain", mutate(func(b []byte) { b[1] |= 0x01 }), ErrUnsupported},
		{"non-LZMA2 filter", mutate(func(b []byte) { b[2] = 0x03 }), ErrUnsupported},
		{"invalid dictionary code", mutate(func(b []byte) { b[4] = 41 }), ErrCorrupt},
		{"wrong property length", mutate(func(b []byte) { b[3] = 2 }), ErrCorrupt},
		{"non-zero header padding", mutate(func(b []byte) { b[len(b)-1] = 1 }), ErrCorrupt},
		{"filter id integer too long", withBlockHeader(valid, append(append([]byte{0x00}, tooLong...), 0x01, 22)), ErrCorrupt},
		{"size field integer too long", withBlockHeader(valid, append(append([]byte{0x40}, tooLong...), 0x21, 0x01, 22)), ErrCorrupt},
		{"unterminated integer", withBlockHeader(valid, []byte{0x00, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80}), ErrCorrupt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newXZReader(bytes.NewReader(tc.stream), maxClusterBytes)
			wantErr(t, err, tc.want)
		})
	}
}

func TestNewXZReaderAcceptsRebuiltHeaders(t *testing.T) {
	// Controls for the crafted-header cases above: the same constructions with
	// well-formed integers are accepted, so those cases fail for their reason.
	valid := xzStream(t, zimtest.ClusterBody([][]byte{[]byte("x")}, false))
	if len(valid) < xzStreamHeaderLen+8 || (int(valid[xzStreamHeaderLen])+1)*4 <= 8 {
		t.Fatalf("unexpected block header size in %x", valid[:24])
	}
	for name, stream := range map[string][]byte{
		"untouched":            valid,
		"rebuilt":              withBlockHeader(valid, []byte{0x00, 0x21, 0x01, 22}),
		"with compressed size": withBlockHeader(valid, []byte{0x40, 0x05, 0x21, 0x01, 22}),
		"intact stream header": withStreamHeader(valid, [2]byte{valid[6], valid[7]}),
	} {
		if _, err := newXZReader(bytes.NewReader(stream), maxClusterBytes); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestXZReaderDictCap(t *testing.T) {
	cases := []struct {
		name             string
		declared, maxDic int64
		want             int
	}{
		{"declared below cap", 8 << 20, 32 << 20, 8 << 20},
		{"declared above cap", 64 << 20, 32 << 20, 32 << 20},
		{"4 GiB declared, huge cap", 0xFFFFFFFF, math.MaxInt64, math.MaxInt32},
		{"tiny cap rises to the decoder minimum", 8 << 20, 100, int(lzma.MinDictCap)},
		{"tiny declared rises to the decoder minimum", 100, 32 << 20, int(lzma.MinDictCap)},
	}
	for _, tc := range cases {
		if got := xzReaderDictCap(tc.declared, tc.maxDic); got != tc.want {
			t.Errorf("%s: xzReaderDictCap(%d, %d) = %d, want %d", tc.name, tc.declared, tc.maxDic, got, tc.want)
		}
	}
}
