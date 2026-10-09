package zim

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"math"
	"math/rand/v2"
	"testing"
	"testing/iotest"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"

	"aurago/internal/zim/zimtest"
)

func TestReadClusterDataNormalAndExtended(t *testing.T) {
	for _, extended := range []bool{false, true} {
		body := zimtest.ClusterBody([][]byte{[]byte("alpha"), {}, []byte("gamma")}, extended)
		cd, err := readClusterData(bytes.NewReader(body), extended, 1<<20, 10)
		if err != nil {
			t.Fatalf("extended=%v: %v", extended, err)
		}
		if cd.count != 3 {
			t.Fatalf("extended=%v: count = %d, want 3", extended, cd.count)
		}
		for i, want := range []string{"alpha", "", "gamma"} {
			got, err := cd.blob(uint32(i))
			if err != nil || string(got) != want {
				t.Fatalf("extended=%v blob %d = %q, %v; want %q", extended, i, got, err, want)
			}
		}
		_, err = cd.blob(3)
		wantErr(t, err, ErrCorrupt)
	}
}

func TestReadClusterDataRejectsBadTables(t *testing.T) {
	le := binary.LittleEndian
	table := func(offsets ...uint32) []byte {
		b := make([]byte, 4*len(offsets))
		for i, o := range offsets {
			le.PutUint32(b[4*i:], o)
		}
		return b
	}
	cases := []struct {
		name string
		body []byte
		want error
	}{
		{"zero first offset", table(0, 0), ErrCorrupt},
		{"unaligned first offset", table(6, 6), ErrCorrupt},
		{"more blobs than entries", table(4 * 20), ErrCorrupt},
		{"unordered offsets", append(table(12, 14, 13), 'a', 'b'), ErrCorrupt},
		{"oversized cluster", table(8, 33<<20), ErrUnsupported},
		{"truncated data", append(table(8, 20), 'a'), ErrCorrupt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeCluster(bytes.NewReader(tc.body), clusterInfo{comp: compNone}, 32<<20, 10)
			wantErr(t, err, tc.want)
		})
	}
}

func TestDecodeClusterZstdAndXZ(t *testing.T) {
	body := zimtest.ClusterBody([][]byte{[]byte("hello"), bytes.Repeat([]byte("z"), 5000)}, false)
	enc, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	zstdData := enc.EncodeAll(body, nil)
	_ = enc.Close()
	var xzData bytes.Buffer
	w, err := xz.NewWriter(&xzData)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write(body)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	// Trailing bytes stand for the next cluster; decoding must stop at the cluster end.
	trailer := []byte("NEXT-CLUSTER-BYTES")
	for name, tc := range map[string]struct {
		comp byte
		data []byte
	}{"zstd": {compZstd, zstdData}, "xz": {compXZ, xzData.Bytes()}} {
		src := append(append([]byte(nil), tc.data...), trailer...)
		cd, err := decodeCluster(bytes.NewReader(src), clusterInfo{comp: tc.comp}, 1<<20, 10)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got, _ := cd.blob(1)
		if len(got) != 5000 || got[0] != 'z' {
			t.Fatalf("%s: blob 1 has %d bytes", name, len(got))
		}
	}
}

func TestDecodeClusterRejectsGarbage(t *testing.T) {
	for _, comp := range []byte{compZstd, compXZ} {
		_, err := decodeCluster(bytes.NewReader([]byte("definitely not compressed data")), clusterInfo{comp: comp}, 1<<20, 10)
		wantErr(t, err, ErrCorrupt)
	}
}

func TestXZDictionaryIsCappedByClusterLimit(t *testing.T) {
	body := zimtest.ClusterBody([][]byte{[]byte("small")}, false)
	var buf bytes.Buffer
	w, err := xz.WriterConfig{DictCap: 8 << 20}.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write(body)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	// A 4 KiB limit caps the 8 MiB declared dictionary; small data still decodes.
	cd, err := decodeCluster(bytes.NewReader(buf.Bytes()), clusterInfo{comp: compXZ}, 4096, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := cd.blob(0); string(got) != "small" {
		t.Fatalf("blob = %q", got)
	}
}

func TestXZDictSize(t *testing.T) {
	cases := map[byte]int64{0: 4096, 1: 6144, 18: 2 << 20, 22: 8 << 20, 26: 32 << 20, 27: 48 << 20, 28: 64 << 20, 40: 0xFFFFFFFF}
	for code, want := range cases {
		got, err := xzDictSize(code)
		if err != nil || got != want {
			t.Fatalf("xzDictSize(%d) = %d, %v; want %d", code, got, err, want)
		}
	}
	_, err := xzDictSize(41)
	wantErr(t, err, ErrCorrupt)
}

func TestReadClusterDataClampsHugeLimits(t *testing.T) {
	// Extended table announcing a 1 TiB cluster: even an unbounded limit must
	// not reach make([]byte, n) with it, which could not be sized on 32-bit.
	table := make([]byte, 16)
	binary.LittleEndian.PutUint64(table[0:], 16)
	binary.LittleEndian.PutUint64(table[8:], 1<<40)
	_, err := readClusterData(bytes.NewReader(table), true, math.MaxInt64, 10)
	wantErr(t, err, ErrUnsupported)

	small := zimtest.ClusterBody([][]byte{[]byte("ok")}, false)
	cd, err := readClusterData(bytes.NewReader(small), false, math.MaxInt64, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := cd.blob(0); string(got) != "ok" {
		t.Fatalf("blob = %q", got)
	}

	_, err = readClusterData(bytes.NewReader(small), false, -1, 10)
	wantErr(t, err, ErrUnsupported)
}

// encodedClusters returns one encoded cluster per supported compression,
// large enough that half of it still holds a partial stream.
func encodedClusters(t *testing.T) map[string]struct {
	comp byte
	data []byte
} {
	t.Helper()
	rng := rand.New(rand.NewPCG(3, 4))
	noise := make([]byte, 24<<10)
	for i := range noise {
		noise[i] = byte(rng.Uint32())
	}
	body := zimtest.ClusterBody([][]byte{noise, []byte("tail")}, false)
	enc, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	zstdData := enc.EncodeAll(body, nil)
	_ = enc.Close()
	return map[string]struct {
		comp byte
		data []byte
	}{
		"none": {compNone, body},
		"zstd": {compZstd, zstdData},
		"xz":   {compXZ, xzStream(t, body)},
	}
}

func TestDecodeClusterIOErrorsAreNotCorruption(t *testing.T) {
	errDisk := errors.New("disk on fire")
	for name, tc := range encodedClusters(t) {
		for _, cut := range []int{0, len(tc.data) / 2} {
			src := io.MultiReader(bytes.NewReader(tc.data[:cut]), iotest.ErrReader(errDisk))
			_, err := decodeCluster(src, clusterInfo{comp: tc.comp}, 1<<20, 10)
			if !errors.Is(err, errDisk) {
				t.Fatalf("%s, I/O error after %d bytes: error = %v, want it to wrap the read error", name, cut, err)
			}
			if errors.Is(err, ErrCorrupt) {
				t.Fatalf("%s, I/O error after %d bytes: %v must not be classified as corruption", name, cut, err)
			}
		}
	}
}

func TestDecodeClusterClosedFileIsErrClosed(t *testing.T) {
	closed := &fs.PathError{Op: "read", Path: "archive.zim", Err: fs.ErrClosed}
	for name, tc := range encodedClusters(t) {
		src := io.MultiReader(bytes.NewReader(tc.data[:len(tc.data)/2]), iotest.ErrReader(closed))
		_, err := decodeCluster(src, clusterInfo{comp: tc.comp}, 1<<20, 10)
		if !errors.Is(err, ErrClosed) || errors.Is(err, ErrCorrupt) {
			t.Fatalf("%s: error = %v, want ErrClosed", name, err)
		}
	}
}

func TestDecodeClusterTruncatedDataIsCorrupt(t *testing.T) {
	for name, tc := range encodedClusters(t) {
		for _, cut := range []int{0, 3, len(tc.data) / 2} {
			_, err := decodeCluster(bytes.NewReader(tc.data[:cut]), clusterInfo{comp: tc.comp}, 1<<20, 10)
			if !errors.Is(err, ErrCorrupt) {
				t.Fatalf("%s truncated to %d of %d bytes: error = %v, want ErrCorrupt", name, cut, len(tc.data), err)
			}
		}
	}
}
