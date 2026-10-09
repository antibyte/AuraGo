package zim

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

var xzMagic = []byte{0xFD, '7', 'z', 'X', 'Z', 0x00}

const (
	xzStreamHeaderLen = 12
	xzFilterLZMA2     = 0x21
)

// newXZReader returns the LZMA2 payload of the first block of an xz stream.
// libzim writes each legacy xz cluster as one single-filter LZMA2 block. The
// dictionary is capped at maxDict: a match can never reach further back than
// the bytes already produced, and readClusterData never produces more than
// maxDict bytes, so a declared multi-GiB dictionary cannot force a huge
// allocation.
func newXZReader(r io.Reader, maxDict int64) (io.Reader, error) {
	var sh [xzStreamHeaderLen]byte
	if _, err := io.ReadFull(r, sh[:]); err != nil {
		return nil, err
	}
	if !bytes.Equal(sh[:6], xzMagic) {
		return nil, errCorrupt("xz cluster has no xz stream header")
	}
	var sizeByte [1]byte
	if _, err := io.ReadFull(r, sizeByte[:]); err != nil {
		return nil, err
	}
	if sizeByte[0] == 0 {
		return nil, errCorrupt("xz cluster stream has no block")
	}
	hlen := (int(sizeByte[0]) + 1) * 4
	bh := make([]byte, hlen)
	bh[0] = sizeByte[0]
	if _, err := io.ReadFull(r, bh[1:]); err != nil {
		return nil, err
	}
	body := bh[:hlen-4]
	if crc32.ChecksumIEEE(body) != binary.LittleEndian.Uint32(bh[hlen-4:]) {
		return nil, errCorrupt("xz block header checksum mismatch")
	}
	flags := body[1]
	if flags&0x3C != 0 {
		return nil, errCorrupt("xz block header has reserved flags set")
	}
	if flags&0x03 != 0 {
		return nil, errUnsupported("xz filter chains are not supported")
	}
	p := 2
	for _, present := range []bool{flags&0x40 != 0, flags&0x80 != 0} {
		if !present {
			continue
		}
		_, n := binary.Uvarint(body[p:])
		if n <= 0 {
			return nil, errCorrupt("xz block header size field is invalid")
		}
		p += n
	}
	id, n := binary.Uvarint(body[p:])
	if n <= 0 {
		return nil, errCorrupt("xz block header filter id is invalid")
	}
	p += n
	if id != xzFilterLZMA2 {
		return nil, errUnsupported("xz filter %#x", id)
	}
	propLen, n := binary.Uvarint(body[p:])
	if n <= 0 || propLen != 1 || p+n >= len(body) {
		return nil, errCorrupt("xz LZMA2 filter properties are invalid")
	}
	dict, err := xzDictSize(body[p+n])
	if err != nil {
		return nil, err
	}
	dict = min(dict, maxDict)
	dict = max(dict, int64(lzma.MinDictCap))
	return lzma.Reader2Config{DictCap: int(dict)}.NewReader2(r)
}

// xzDictSize decodes the LZMA2 dictionary-size property byte.
func xzDictSize(code byte) (int64, error) {
	switch {
	case code > 40:
		return 0, errCorrupt("xz LZMA2 dictionary code %d", code)
	case code == 40:
		return 0xFFFFFFFF, nil
	}
	return int64(2|(code&1)) << (code/2 + 11), nil
}
