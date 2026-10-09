package zim

import (
	"encoding/binary"
	"strings"
	"testing"

	"aurago/internal/zim/zimtest"
)

// mutated builds the sample archive, applies mutate and writes it to disk.
func mutated(t *testing.T, mutate func(data []byte, l *zimtest.Layout) []byte) string {
	t.Helper()
	data, layout, err := sampleBuilder().Build()
	if err != nil {
		t.Fatal(err)
	}
	return zimtest.WriteBytes(t, mutate(data, layout))
}

func TestOpenRejectsInvalidArchives(t *testing.T) {
	le := binary.LittleEndian
	cases := []struct {
		name   string
		mutate func([]byte, *zimtest.Layout) []byte
		want   error
	}{
		{"bad magic", func(d []byte, _ *zimtest.Layout) []byte { d[0] = 'X'; return d }, ErrNotZIM},
		{"truncated file", func(d []byte, _ *zimtest.Layout) []byte { return d[:len(d)-100] }, ErrCorrupt},
		{"appended bytes", func(d []byte, _ *zimtest.Layout) []byte { return append(d, 0) }, ErrCorrupt},
		{"unsupported major version", func(d []byte, _ *zimtest.Layout) []byte { le.PutUint16(d[4:], 4); return d }, ErrUnsupported},
		{"path pointer list out of bounds", func(d []byte, l *zimtest.Layout) []byte {
			le.PutUint64(d[32:], uint64(l.ChecksumPos))
			return d
		}, ErrCorrupt},
		{"unterminated MIME list", func(d []byte, l *zimtest.Layout) []byte {
			for i := 80; i < int(l.ClusterOffsets[0]); i++ {
				d[i] = 'a'
			}
			for i := int(l.ClusterOffsets[0]); i < len(d); i++ {
				if d[i] == 0 {
					d[i] = 'b'
				}
			}
			return d
		}, ErrCorrupt},
		{"dirent pointer out of bounds", func(d []byte, l *zimtest.Layout) []byte {
			for i := range l.EntryIndex {
				le.PutUint64(d[l.PathPtrPos+8*int64(l.EntryIndex[i]):], uint64(len(d)+10))
			}
			return d
		}, ErrCorrupt},
		{"title list blob out of bounds", func(d []byte, l *zimtest.Layout) []byte {
			// The v1 listing lives alone in the last (uncompressed) cluster: make its end offset huge.
			listing := l.ClusterOffsets[len(l.ClusterOffsets)-1]
			le.PutUint32(d[listing+1+4:], 0x7FFFFFFF)
			return d
		}, ErrCorrupt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Open(mutated(t, tc.mutate), Options{})
			wantErr(t, err, tc.want)
		})
	}
}

func TestReadsFailOnCorruptStructures(t *testing.T) {
	le := binary.LittleEndian
	clusterOf := func(l *zimtest.Layout, i int) int64 { return l.ClusterOffsets[i] }
	cases := []struct {
		name   string
		mutate func([]byte, *zimtest.Layout) []byte
		path   string
		want   error
	}{
		{"cluster pointer out of bounds", func(d []byte, l *zimtest.Layout) []byte {
			le.PutUint64(d[l.ClusterPtrPos:], uint64(len(d)))
			return d
		}, "Berlin", ErrCorrupt},
		{"unknown compression byte", func(d []byte, l *zimtest.Layout) []byte { d[clusterOf(l, 0)] = 0x07; return d }, "Berlin", ErrCorrupt},
		{"discontinued zlib compression", func(d []byte, l *zimtest.Layout) []byte { d[clusterOf(l, 0)] = 0x02; return d }, "Berlin", ErrUnsupported},
		{"garbage zstd data", func(d []byte, l *zimtest.Layout) []byte {
			for i := clusterOf(l, 0) + 1; i < clusterOf(l, 1); i++ {
				d[i] = 0x5A
			}
			return d
		}, "Berlin", ErrCorrupt},
		{"garbage xz data", func(d []byte, l *zimtest.Layout) []byte {
			for i := clusterOf(l, 1) + 13; i < clusterOf(l, 2); i++ {
				d[i] = 0x5A
			}
			return d
		}, "Hamburg", ErrCorrupt},
		{"blob number out of range", func(d []byte, l *zimtest.Layout) []byte {
			le.PutUint32(d[l.DirentOffset["C/Berlin"]+12:], 99)
			return d
		}, "Berlin", ErrCorrupt},
		{"cluster number out of range", func(d []byte, l *zimtest.Layout) []byte {
			le.PutUint32(d[l.DirentOffset["C/Berlin"]+8:], 99)
			return d
		}, "Berlin", ErrCorrupt},
		{"uncompressed blob beyond archive", func(d []byte, l *zimtest.Layout) []byte {
			// Cluster 2 (uncompressed) holds C/map.webp as blob 0: stretch its end offset.
			le.PutUint32(d[clusterOf(l, 2)+1+4:], 0x7FFFFFF0)
			return d
		}, "map.webp", ErrCorrupt},
		{"extended cluster in major 5", func(d []byte, _ *zimtest.Layout) []byte { le.PutUint16(d[4:], 5); return d }, "Köln", ErrCorrupt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := openPath(t, mutated(t, tc.mutate))
			e, err := a.EntryByPath('C', tc.path)
			if err == nil {
				_, err = a.Open(e)
			}
			wantErr(t, err, tc.want)
		})
	}
}

func TestOversizedClusterIsRejectedBeforeAllocation(t *testing.T) {
	raw := make([]byte, 8)
	binary.LittleEndian.PutUint32(raw[0:], 8)
	binary.LittleEndian.PutUint32(raw[4:], maxClusterBytes+1) // one blob claiming 32 MiB + 1
	b := zimtest.New()
	c := b.AddRawCluster(zimtest.CompressionZstd, false, raw)
	b.Add(zimtest.Entry{Namespace: 'C', Path: "huge", MimeType: "text/html", Cluster: c, Blob: 0})
	path, _ := b.WriteFile(t)
	a := openPath(t, path)
	_, err := a.Open(mustEntry(t, a, 'C', "huge"))
	wantErr(t, err, ErrUnsupported)
}

func TestOverlongDirentIsCorrupt(t *testing.T) {
	b := zimtest.New()
	b.TitleListV1 = false
	c := b.AddCluster(zimtest.CompressionNone, false)
	b.Add(zimtest.Entry{Namespace: 'C', Path: strings.Repeat("a", maxDirentSize), MimeType: "text/html", Data: []byte("x"), Cluster: c})
	path, _ := b.WriteFile(t)
	_, err := Open(path, Options{})
	wantErr(t, err, ErrCorrupt)
}
