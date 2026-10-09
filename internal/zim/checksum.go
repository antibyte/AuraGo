package zim

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // MD5 is mandated by the ZIM checksum format, not a security choice
)

const checksumChunkBytes = 1 << 20

// VerifyChecksum compares the MD5 of [0, checksumPos) with the stored
// checksum. It stops with ctx.Err() when ctx is cancelled.
func (a *Archive) VerifyChecksum(ctx context.Context) error {
	if !a.hdr.hasChecksum() {
		return errUnsupported("archive has no checksum")
	}
	h := md5.New() //nolint:gosec // see import
	buf := make([]byte, checksumChunkBytes)
	end := a.dataEnd
	for off := int64(0); off < end; {
		if err := ctx.Err(); err != nil {
			return err
		}
		n := min(int64(len(buf)), end-off)
		if err := a.readAt(buf[:n], off); err != nil {
			return err
		}
		h.Write(buf[:n])
		off += n
	}
	var want [checksumSize]byte
	if err := a.readAt(want[:], end); err != nil {
		return err
	}
	if !bytes.Equal(h.Sum(nil), want[:]) {
		return errCorrupt("MD5 checksum mismatch")
	}
	return nil
}
