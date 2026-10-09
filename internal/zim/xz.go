package zim

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"math"

	"github.com/ulikunitz/xz/lzma"
)

var xzMagic = []byte{0xFD, '7', 'z', 'X', 'Z', 0x00}

const (
	xzStreamHeaderLen = 12
	xzFilterLZMA2     = 0x21
	// xzMaxVarintLen is the longest multibyte integer the xz format allows.
	xzMaxVarintLen = 9
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
	if sh[6] != 0 || sh[7]&0xF0 != 0 {
		return nil, errCorrupt("xz stream header has reserved flags set")
	}
	if crc32.ChecksumIEEE(sh[6:8]) != binary.LittleEndian.Uint32(sh[8:12]) {
		return nil, errCorrupt("xz stream header checksum mismatch")
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
		_, n, err := xzVarint(body[p:], "size field")
		if err != nil {
			return nil, err
		}
		p += n
	}
	id, n, err := xzVarint(body[p:], "filter id")
	if err != nil {
		return nil, err
	}
	p += n
	if id != xzFilterLZMA2 {
		return nil, errUnsupported("xz filter %#x", id)
	}
	propLen, n, err := xzVarint(body[p:], "filter property size")
	if err != nil || propLen != 1 || p+n >= len(body) {
		return nil, errCorrupt("xz LZMA2 filter properties are invalid")
	}
	declared, err := xzDictSize(body[p+n])
	if err != nil {
		return nil, err
	}
	for _, pad := range body[p+n+1:] {
		if pad != 0 {
			return nil, errCorrupt("xz block header padding is not zero")
		}
	}
	return lzma.Reader2Config{DictCap: xzReaderDictCap(declared, maxDict)}.NewReader2(r)
}

// xzVarint decodes one xz multibyte integer (at most xzMaxVarintLen bytes).
func xzVarint(b []byte, what string) (uint64, int, error) {
	v, n := binary.Uvarint(b)
	if n <= 0 || n > xzMaxVarintLen {
		return 0, 0, errCorrupt("xz block header %s is invalid", what)
	}
	return v, n, nil
}

// xzReaderDictCap picks the LZMA2 dictionary capacity: the declared size
// capped at maxDict and at what an int holds on 32-bit platforms, but at
// least the smallest dictionary the decoder accepts.
func xzReaderDictCap(declared, maxDict int64) int {
	dict := min(declared, maxDict, math.MaxInt32)
	return int(max(dict, int64(lzma.MinDictCap)))
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
