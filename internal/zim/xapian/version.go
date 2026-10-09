package xapian

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	glassMagic = "\x0f\x0dXapian Glass"
	// glassFormatVersion is DATE_TO_VERSION(2016,03,14): (2016-2014)<<9 | 3<<5 | 14.
	glassFormatVersion = 0x046e
	versionHeaderSize  = 14 + 2 + 16 // magic, format, UUID
)

// Table order inside the version block.
const (
	tablePostlist = iota
	tableDocdata
	tableTermlist
	tablePosition
	tableSpelling
	tableSynonym
	tableCount
)

var tableNames = [tableCount]string{"postlist", "docdata", "termlist", "position", "spelling", "synonym"}

type rootInfo struct {
	root        uint32
	level       int
	numEntries  uint64
	sequential  bool
	rootIsFake  bool
	blockSize   int
	compressMin uint32
	freelist    []byte
}

type versionInfo struct {
	uuid        [16]byte
	revision    uint32
	roots       [tableCount]rootInfo
	docCount    uint32
	lastDocID   uint32
	doclenLower uint32
	wdfUpper    uint32
	doclenUpper uint32
	totalLength uint64
}

// parseVersion decodes block 0 of a single-file glass database.
func parseVersion(b []byte) (versionInfo, error) {
	var v versionInfo
	if len(b) < versionHeaderSize+1 {
		return v, unsupportedf("too short for a glass version block")
	}
	if string(b[:14]) != glassMagic {
		if bytes.HasPrefix(b, []byte("\x0f\x0dXapian ")) {
			return v, unsupportedf("Xapian backend %q is not glass", bytes.TrimRight(b[9:14], "\x00"))
		}
		return v, unsupportedf("not a Xapian glass database")
	}
	if f := binary.BigEndian.Uint16(b[14:16]); f != glassFormatVersion {
		return v, unsupportedf("glass format %04d-%02d-%02d, want 2016-03-14", (f>>9)+2014, (f>>5)&0x0f, f&0x1f)
	}
	copy(v.uuid[:], b[16:32])
	p := b[versionHeaderSize:]
	next := func(what string) (uint64, error) {
		x, n, err := unpackUint(p)
		if err != nil {
			return 0, fmt.Errorf("version block %s: %w", what, err)
		}
		p = p[n:]
		return x, nil
	}
	rev, err := next("revision")
	if err != nil {
		return v, err
	}
	if rev > 0xffffffff {
		return v, corruptf("version block revision out of range")
	}
	v.revision = uint32(rev)
	for t := 0; t < tableCount; t++ {
		ri := &v.roots[t]
		root, err := next("root")
		if err != nil {
			return v, err
		}
		flags, err := next("flags")
		if err != nil {
			return v, err
		}
		entries, err := next("entries")
		if err != nil {
			return v, err
		}
		bs, err := next("block size")
		if err != nil {
			return v, err
		}
		cmin, err := next("compress_min")
		if err != nil {
			return v, err
		}
		flLen, err := next("freelist length")
		if err != nil {
			return v, err
		}
		if flLen > uint64(len(p)) {
			return v, corruptf("version block freelist overruns data")
		}
		// Range-check every field on its uint64 before narrowing: on 32-bit
		// platforms int(flags>>2) of a huge value wraps negative and would
		// pass a later level check.
		if root > 0xffffffff || cmin > 0xffffffff {
			return v, corruptf("version block root info out of range for %s", tableNames[t])
		}
		if bs > maxBlockSize>>11 {
			return v, unsupportedf("%s block size field %d", tableNames[t], bs)
		}
		if flags>>2 > maxLevel {
			return v, corruptf("%s B-tree level %d", tableNames[t], flags>>2)
		}
		ri.root = uint32(root)
		ri.level = int(flags >> 2)
		ri.sequential = flags&2 != 0
		ri.rootIsFake = flags&1 != 0
		ri.numEntries = entries
		ri.blockSize = int(bs) << 11
		ri.compressMin = uint32(cmin)
		ri.freelist = append([]byte(nil), p[:flLen]...)
		p = p[flLen:]
		if ri.blockSize < minBlockSize || ri.blockSize&(ri.blockSize-1) != 0 {
			return v, unsupportedf("%s block size %d", tableNames[t], ri.blockSize)
		}
	}
	// Statistics (all zero for an empty database).
	var st [8]uint64
	for i := range st {
		if len(p) == 0 {
			if i == 0 {
				break
			}
			return v, corruptf("version block statistics truncated")
		}
		x, err := next("statistics")
		if err != nil {
			return v, err
		}
		st[i] = x
	}
	// Order: doccount, lastdocid-doccount, doclen lower bound, wdf upper bound,
	// doclen upper bound - wdf upper bound, oldest changeset, total length,
	// spelling wordfreq upper bound. Xapian 1.4 reads every statistic except
	// the total length into a 32-bit type and rejects the file when one does
	// not fit; checking them first also keeps the sums below from wrapping.
	for i, x := range st {
		if i != 6 && x > 0xffffffff {
			return v, corruptf("version block statistic %d out of range", i)
		}
	}
	last := st[1] + st[0]
	if last > 0xffffffff || st[4]+st[3] > 0xffffffff {
		return v, corruptf("version block statistics out of range")
	}
	v.docCount = uint32(st[0])
	v.lastDocID = uint32(last)
	v.doclenLower = uint32(st[2])
	v.wdfUpper = uint32(st[3])
	v.doclenUpper = uint32(st[4] + st[3])
	v.totalLength = st[6]
	return v, nil
}
