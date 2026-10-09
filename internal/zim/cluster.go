package zim

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"sync"

	"github.com/klauspost/compress/zstd"
)

const (
	compNoneLegacy = 0 // very old archives: no compression
	compNone       = 1
	compZlib       = 2 // discontinued upstream
	compBzip2      = 3 // discontinued upstream
	compXZ         = 4
	compZstd       = 5

	clusterExtendedFlag = 0x10

	// maxClusterBytes caps one decompressed cluster (libzim writes 2 MiB clusters).
	maxClusterBytes = 32 << 20
	// zstdMaxMemory caps the zstd window (and the decoded size of one frame) of
	// a streaming decoder. A cluster that decompresses to at most maxClusterBytes
	// never needs a larger window, and libzim itself writes level-19 frames with
	// an 8 MiB window.
	zstdMaxMemory     = maxClusterBytes
	clusterReadBuffer = 64 << 10
)

type clusterInfo struct {
	offset   int64 // file offset of the cluster info byte
	comp     byte
	extended bool
}

func (ci clusterInfo) compressed() bool { return ci.comp == compXZ || ci.comp == compZstd }

func (ci clusterInfo) offsetWidth() uint64 {
	if ci.extended {
		return 8
	}
	return 4
}

// clusterData is a decompressed cluster: offset table followed by blob bytes.
// Offsets are validated (ordered, inside data) when the cluster is loaded.
type clusterData struct {
	data  []byte
	width uint64
	count uint64 // number of blobs
}

func (cd *clusterData) blob(n uint32) ([]byte, error) {
	if uint64(n) >= cd.count {
		return nil, errCorrupt("blob %d out of range (cluster has %d)", n, cd.count)
	}
	i := uint64(n) * cd.width
	start := readOffset(cd.data[i:i+cd.width], cd.width)
	end := readOffset(cd.data[i+cd.width:i+2*cd.width], cd.width)
	return cd.data[start:end], nil
}

func readOffset(b []byte, width uint64) uint64 {
	if width == 8 {
		return binary.LittleEndian.Uint64(b)
	}
	return uint64(binary.LittleEndian.Uint32(b))
}

// validateFirstOffset checks the first blob offset, which is also the size of
// the offset table: a positive multiple of the offset width, at most one
// offset more than there are entries in the archive.
func validateFirstOffset(first, width, maxBlobs uint64) error {
	if first < width || first%width != 0 {
		return errCorrupt("invalid first blob offset %d", first)
	}
	if first/width > maxBlobs+1 {
		return errCorrupt("cluster claims %d blobs, archive has %d entries", first/width-1, maxBlobs)
	}
	return nil
}

// readClusterData reads a complete decompressed cluster from r without ever
// allocating more than limit bytes. The limit is clamped to what a Go slice
// length can hold on 32-bit platforms.
func readClusterData(r io.Reader, extended bool, limit int64, maxBlobs uint64) (*clusterData, error) {
	limit = max(0, min(limit, math.MaxInt32))
	width := uint64(4)
	if extended {
		width = 8
	}
	head := make([]byte, width)
	if _, err := io.ReadFull(r, head); err != nil {
		return nil, err
	}
	first := readOffset(head, width)
	if err := validateFirstOffset(first, width, maxBlobs); err != nil {
		return nil, err
	}
	if first > uint64(limit) {
		return nil, errUnsupported("cluster offset table of %d bytes exceeds the %d byte limit", first, limit)
	}
	table := make([]byte, first)
	copy(table, head)
	if _, err := io.ReadFull(r, table[width:]); err != nil {
		return nil, err
	}
	last := first
	for i := width; i < first; i += width {
		v := readOffset(table[i:i+width], width)
		if v < last {
			return nil, errCorrupt("cluster blob offsets are not ordered")
		}
		last = v
	}
	if last > uint64(limit) {
		return nil, errUnsupported("decompressed cluster of %d bytes exceeds the %d byte limit", last, limit)
	}
	data := make([]byte, last)
	copy(data, table)
	if _, err := io.ReadFull(r, data[first:]); err != nil {
		return nil, err
	}
	return &clusterData{data: data, width: width, count: first/width - 1}, nil
}

// zstdDecoders pools stream decoders across clusters and archives. A pooled
// decoder keeps the history buffers of its last stream, which can grow up to
// the zstdMaxMemory (32 MiB) window and live outside the cluster cache budget.
// Real libzim clusters (about 2 MiB, single-segment frames) keep them small.
var zstdDecoders sync.Pool

// getZstdDecoder returns a pooled synchronous stream decoder (concurrency 1
// starts no goroutines) whose window is capped at zstdMaxMemory.
func getZstdDecoder() (*zstd.Decoder, error) {
	if d, ok := zstdDecoders.Get().(*zstd.Decoder); ok {
		return d, nil
	}
	return zstd.NewReader(nil,
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderMaxMemory(zstdMaxMemory),
	)
}

func putZstdDecoder(d *zstd.Decoder) {
	_ = d.Reset(nil) // drop the reference to the archive file
	zstdDecoders.Put(d)
}

// readErrTracker remembers the first genuine I/O error (anything but running
// out of data) that the decoder saw, so a failing disk read is not mistaken
// for corrupt cluster data once a decompressor wraps or masks the error.
type readErrTracker struct {
	r   io.Reader
	err error
}

func (t *readErrTracker) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if err != nil && t.err == nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		t.err = err
	}
	return n, err
}

// decodeCluster decompresses the cluster data that follows the info byte.
// Short or invalid data yields ErrCorrupt, over-limit data ErrUnsupported; an
// I/O error from src is returned wrapped (ErrClosed for a closed file) and is
// never classified as corruption.
func decodeCluster(src io.Reader, ci clusterInfo, limit int64, maxBlobs uint64) (*clusterData, error) {
	tr := &readErrTracker{r: bufio.NewReaderSize(src, clusterReadBuffer)}
	cd, err := decodeClusterFrom(tr, ci, limit, maxBlobs)
	if err != nil && tr.err != nil {
		if errors.Is(tr.err, fs.ErrClosed) {
			return nil, ErrClosed
		}
		return nil, fmt.Errorf("zim: reading cluster: %w", tr.err)
	}
	return cd, err
}

func decodeClusterFrom(br io.Reader, ci clusterInfo, limit int64, maxBlobs uint64) (*clusterData, error) {
	switch ci.comp {
	case compZstd:
		dec, err := getZstdDecoder()
		if err != nil {
			return nil, err
		}
		defer putZstdDecoder(dec)
		if err := dec.Reset(br); err != nil {
			return nil, mapDecodeError(err)
		}
		cd, err := readClusterData(dec, ci.extended, limit, maxBlobs)
		return cd, mapDecodeError(err)
	case compXZ:
		xr, err := newXZReader(br, limit)
		if err != nil {
			return nil, mapDecodeError(err)
		}
		cd, err := readClusterData(xr, ci.extended, limit, maxBlobs)
		return cd, mapDecodeError(err)
	case compNone, compNoneLegacy:
		cd, err := readClusterData(br, ci.extended, limit, maxBlobs)
		return cd, mapDecodeError(err)
	}
	return nil, errCorrupt("unknown cluster compression %d", ci.comp)
}

// mapDecodeError classifies decompression failures; nil stays nil.
func mapDecodeError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrCorrupt), errors.Is(err, ErrUnsupported), errors.Is(err, ErrClosed):
		return err
	case errors.Is(err, fs.ErrClosed):
		return ErrClosed
	case errors.Is(err, zstd.ErrWindowSizeExceeded), errors.Is(err, zstd.ErrDecoderSizeExceeded):
		return errUnsupported("zstd cluster needs more than %d bytes of decoder memory", zstdMaxMemory)
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return errCorrupt("cluster data truncated")
	}
	return errCorrupt("cluster decompression failed: %v", err)
}
