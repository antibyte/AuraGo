package zim

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"aurago/internal/zim/zimtest"
)

func TestVerifyChecksum(t *testing.T) {
	a := openSample(t)
	if err := a.VerifyChecksum(context.Background()); err != nil {
		t.Fatalf("VerifyChecksum() = %v", err)
	}
}

func TestVerifyChecksumDetectsMismatch(t *testing.T) {
	data, layout, err := sampleBuilder().Build()
	if err != nil {
		t.Fatal(err)
	}
	// Flip a byte inside the uncompressed cluster's blob data: the archive
	// still opens, only the checksum fails.
	data[layout.ClusterOffsets[2]+40] ^= 0xFF
	a := openPath(t, zimtest.WriteBytes(t, data))
	err = a.VerifyChecksum(context.Background())
	wantErr(t, err, ErrCorrupt)
}

func TestVerifyChecksumHonoursCancellation(t *testing.T) {
	a := openSample(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.VerifyChecksum(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("VerifyChecksum(cancelled) = %v, want context.Canceled", err)
	}
}

func TestVerifyChecksumAfterCloseReportsErrClosed(t *testing.T) {
	path, _ := sampleBuilder().WriteFile(t)
	a, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	wantErr(t, a.VerifyChecksum(context.Background()), ErrClosed)
}

func TestVerifyChecksumWithoutStoredChecksumIsUnsupported(t *testing.T) {
	// Archives with a 72-byte legacy header carry no checksum position.
	a := &Archive{hdr: header{mimeListPos: legacyHeaderSize}}
	wantErr(t, a.VerifyChecksum(context.Background()), ErrUnsupported)
}

// hookReaderAt reports every read to onRead after it has completed.
type hookReaderAt struct {
	r      io.ReaderAt
	onRead func(off int64, n int)
}

func (h *hookReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := h.r.ReadAt(p, off)
	if h.onRead != nil {
		h.onRead(off, n)
	}
	return n, err
}

// multiChunkArchive is an archive of more than three checksum chunks, backed
// by an in-memory reader whose reads the test can observe.
func multiChunkArchive(t *testing.T) (*Archive, *hookReaderAt) {
	t.Helper()
	b := zimtest.New()
	c := b.AddCluster(zimtest.CompressionNone, false)
	b.AddArticle(c, 'C', "Page", "Page", "<p>x</p>")
	b.Add(zimtest.Entry{Namespace: 'C', Path: "blob.bin", MimeType: "application/octet-stream",
		Data: bytes.Repeat([]byte{0x5A}, 3*checksumChunkBytes+123), Cluster: c})
	data, _, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	h := &hookReaderAt{r: bytes.NewReader(data)}
	a, err := newArchive(h, int64(len(data)), Options{}, defaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a, h
}

func TestVerifyChecksumReadsInChunks(t *testing.T) {
	a, h := multiChunkArchive(t)
	var reads []int
	h.onRead = func(_ int64, n int) { reads = append(reads, n) }
	if err := a.VerifyChecksum(context.Background()); err != nil {
		t.Fatalf("VerifyChecksum() = %v", err)
	}
	// Three full chunks, the short remainder, then the 16 stored checksum bytes.
	if len(reads) != 5 || reads[0] != checksumChunkBytes || reads[1] != checksumChunkBytes ||
		reads[2] != checksumChunkBytes || reads[4] != checksumSize || int64(reads[3]) != a.dataEnd-3*checksumChunkBytes {
		t.Fatalf("read sizes = %v for a %d byte archive", reads, a.size)
	}
}

func TestVerifyChecksumStopsMidStreamWhenCancelled(t *testing.T) {
	a, h := multiChunkArchive(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var reads []int64
	h.onRead = func(off int64, n int) {
		reads = append(reads, off)
		if off+int64(n) >= checksumChunkBytes {
			cancel() // deterministic: cancel right after the first chunk has been read
		}
	}
	err := a.VerifyChecksum(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("VerifyChecksum(cancelled after the first chunk) = %v, want context.Canceled", err)
	}
	if len(reads) != 1 || reads[0] != 0 {
		t.Fatalf("reads after cancellation = %v, want only the first chunk at offset 0", reads)
	}
}
