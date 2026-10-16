package bookshelf

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type contextReaderAt struct {
	ctx context.Context
	r   io.ReaderAt
}

func (r contextReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.ReadAt(p, off)
}

const maxArchiveEntries = 10_000
const maxCentralDirectoryBytes = 16 << 20

// boundedZipReader checks the fixed-size directory footer before handing an
// archive to archive/zip, so malicious directory counts cannot allocate an
// unbounded file table.
func boundedZipReader(reader io.ReaderAt, size int64) (*zip.Reader, error) {
	if size < 22 {
		return nil, fmt.Errorf("invalid bookshelf archive")
	}
	tailSize := int64(22 + 65_535)
	if size < tailSize {
		tailSize = size
	}
	tail := make([]byte, int(tailSize))
	n, err := reader.ReadAt(tail, size-tailSize)
	if n != len(tail) {
		return nil, fmt.Errorf("read bookshelf archive footer: %w", io.ErrUnexpectedEOF)
	}
	if err != nil && !(errors.Is(err, io.EOF) && n == len(tail)) {
		return nil, fmt.Errorf("read bookshelf archive footer: %w", err)
	}
	const eocdSignature = uint32(0x06054b50)
	eocd := -1
	for i := len(tail) - 22; i >= 0; i-- {
		if binary.LittleEndian.Uint32(tail[i:i+4]) != eocdSignature {
			continue
		}
		commentSize := int(binary.LittleEndian.Uint16(tail[i+20 : i+22]))
		if i+22+commentSize == len(tail) {
			eocd = i
			break
		}
	}
	if eocd < 0 {
		return nil, fmt.Errorf("invalid bookshelf archive directory")
	}
	e := tail[eocd:]
	disk := binary.LittleEndian.Uint16(e[4:6])
	directoryDisk := binary.LittleEndian.Uint16(e[6:8])
	entriesOnDisk := binary.LittleEndian.Uint16(e[8:10])
	entries := binary.LittleEndian.Uint16(e[10:12])
	directorySize := binary.LittleEndian.Uint32(e[12:16])
	directoryOffset := binary.LittleEndian.Uint32(e[16:20])
	if disk != 0 || directoryDisk != 0 || entriesOnDisk != entries {
		return nil, fmt.Errorf("multi-disk bookshelf archives are unsupported")
	}
	if entries == 0xffff || directorySize == 0xffffffff || directoryOffset == 0xffffffff || eocd >= 20 && binary.LittleEndian.Uint32(tail[eocd-20:eocd-16]) == 0x07064b50 {
		return nil, ErrLimitExceeded // Zip64 metadata is deliberately not indexed.
	}
	if entries > maxArchiveEntries || directorySize > maxCentralDirectoryBytes || int64(directoryOffset)+int64(directorySize) > size-tailSize+int64(eocd) {
		return nil, ErrLimitExceeded
	}
	archive, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, fmt.Errorf("invalid bookshelf archive: %w", err)
	}
	if len(archive.File) > maxArchiveEntries {
		return nil, ErrLimitExceeded
	}
	return archive, nil
}
