package zim

import (
	"bytes"
	"io"
	"testing"

	"aurago/internal/zim/zimtest"
)

const fuzzClusterLimit = 1 << 20

func FuzzParseHeader(f *testing.F) {
	data, _, err := sampleBuilder().Build()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data[:headerSize], int64(len(data)))
	f.Add(make([]byte, headerSize), int64(headerSize))
	f.Fuzz(func(t *testing.T, b []byte, size int64) {
		h, err := parseHeader(b, size)
		if err != nil {
			return
		}
		end := uint64(h.dataEnd(size))
		if end > uint64(size) || !tableFits(h.pathPtrPos, uint64(h.entryCount), 8, h.mimeListPos, end) ||
			!tableFits(h.clusterPtrPos, uint64(h.clusterCount), 8, h.mimeListPos, end) {
			t.Fatalf("accepted header with out-of-bounds tables: %+v (size %d)", h, size)
		}
	})
}

func FuzzParseDirent(f *testing.F) {
	f.Add(contentDirent(0, 'C', 1, 2, "Berlin", "Berlin"))
	f.Add(redirectDirent('W', 3, "mainPage", ""))
	f.Add([]byte{0xFE, 0xFF, 0, 'A', 0, 0, 0, 0, 'x', 0, 0})
	mimes := []string{"text/html", "image/png"}
	f.Fuzz(func(t *testing.T, b []byte) {
		e, err := parseDirent(b, mimes)
		if err != nil {
			return
		}
		if len(e.Path)+len(e.Title) > 2*len(b) || (e.kind == kindContent && e.MimeType == "") {
			t.Fatalf("implausible entry %+v from %d bytes", e, len(b))
		}
	})
}

func FuzzDecodeCluster(f *testing.F) {
	body := zimtest.ClusterBody([][]byte{[]byte("one"), []byte("two")}, false)
	f.Add(byte(compNone), body)
	f.Add(byte(compNone|clusterExtendedFlag), zimtest.ClusterBody([][]byte{[]byte("x")}, true))
	data, layout, err := sampleBuilder().Build()
	if err != nil {
		f.Fatal(err)
	}
	for i := 0; i+1 < len(layout.ClusterOffsets); i++ {
		c := data[layout.ClusterOffsets[i]:layout.ClusterOffsets[i+1]]
		f.Add(c[0], c[1:])
	}
	f.Fuzz(func(t *testing.T, info byte, payload []byte) {
		ci := clusterInfo{comp: info & 0x0F, extended: info&clusterExtendedFlag != 0}
		if !ci.compressed() && ci.comp != compNone && ci.comp != compNoneLegacy {
			return
		}
		cd, err := decodeCluster(bytes.NewReader(payload), ci, fuzzClusterLimit, 1000)
		if err != nil {
			return
		}
		if int64(len(cd.data)) > fuzzClusterLimit {
			t.Fatalf("decoded %d bytes past the %d limit", len(cd.data), fuzzClusterLimit)
		}
		for n := uint32(0); uint64(n) < cd.count; n++ {
			if _, err := cd.blob(n); err != nil {
				t.Fatalf("validated cluster has unreadable blob %d: %v", n, err)
			}
		}
	})
}

func FuzzOpenArchive(f *testing.F) {
	data, _, err := sampleBuilder().Build()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	f.Fuzz(func(t *testing.T, b []byte) {
		a, err := newArchive(bytes.NewReader(b), int64(len(b)), Options{ClusterCacheBytes: fuzzClusterLimit}, limits{maxClusterBytes: fuzzClusterLimit})
		if err != nil {
			return
		}
		defer a.Close()
		for i := uint32(0); i < a.EntryCount() && i < 64; i++ {
			e, err := a.EntryAt(i)
			if err != nil {
				continue
			}
			if e, err = a.Resolve(e); err != nil {
				continue
			}
			if r, err := a.Open(e); err == nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(r, fuzzClusterLimit))
			}
		}
		_, _ = a.MainEntry()
		_, _ = a.TitlePrefix("B", 5)
	})
}
