package zim

import (
	"context"
	"errors"
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
