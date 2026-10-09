package xapian

import (
	"bytes"
	"encoding/binary"
)

const (
	blockHeaderSize = 11 // revision u32, level u8, max_free u16, total_free u16, dir_end u16
	minBlockSize    = 2048
	maxBlockSize    = 65536
	maxLevel        = 9   // Xapian keeps at most 10 cursor levels (0..9)
	levelFreelist   = 254 // freelist blocks; never part of a B-tree

	itemCompressed = 0x80 // flags live in the top bits of the item size field
	itemLast       = 0x40
	itemFirst      = 0x20
	itemSizeMask   = 0x1fff
)

// block is one parsed B-tree block. data is immutable and shared via the cache.
type block struct {
	n     uint32
	level int
	data  []byte
	count int // directory entries
}

type leafItem struct {
	key        []byte
	component  int
	first      bool
	last       bool
	compressed bool
	chunk      []byte
}

type branchItem struct {
	child     uint32
	key       []byte
	component int
}

func parseBlock(n uint32, data []byte) (*block, error) {
	if len(data) < blockHeaderSize {
		return nil, corruptf("block %d shorter than its header", n)
	}
	level := int(data[4])
	if level == levelFreelist {
		return nil, corruptf("block %d is a freelist block", n)
	}
	if level > maxLevel {
		return nil, corruptf("block %d has level %d", n, level)
	}
	dirEnd := int(binary.BigEndian.Uint16(data[9:11]))
	if dirEnd < blockHeaderSize || dirEnd > len(data) || (dirEnd-blockHeaderSize)%2 != 0 {
		return nil, corruptf("block %d has invalid directory end %d", n, dirEnd)
	}
	count := (dirEnd - blockHeaderSize) / 2
	if count == 0 {
		return nil, corruptf("block %d is empty", n)
	}
	return &block{n: n, level: level, data: data, count: count}, nil
}

func (b *block) itemOffset(i int) (int, error) {
	if i < 0 || i >= b.count {
		return 0, corruptf("block %d: item %d out of range", b.n, i)
	}
	o := int(binary.BigEndian.Uint16(b.data[blockHeaderSize+2*i:]))
	if o < blockHeaderSize+2*b.count || o >= len(b.data) {
		return 0, corruptf("block %d: item %d offset %d out of range", b.n, i, o)
	}
	return o, nil
}

// leaf decodes leaf item i: [I u16][K u8][key K bytes][X u16 if not first][tag chunk].
func (b *block) leaf(i int) (leafItem, error) {
	o, err := b.itemOffset(i)
	if err != nil {
		return leafItem{}, err
	}
	d := b.data
	if o+3 > len(d) {
		return leafItem{}, corruptf("block %d: item %d header truncated", b.n, i)
	}
	flags := d[o]
	size := int(binary.BigEndian.Uint16(d[o:])&itemSizeMask) + 3
	if o+size > len(d) {
		return leafItem{}, corruptf("block %d: item %d overruns block", b.n, i)
	}
	klen := int(d[o+2])
	cd := 3 + klen
	it := leafItem{
		first:      flags&itemFirst != 0,
		last:       flags&itemLast != 0,
		compressed: flags&itemCompressed != 0,
		component:  1,
	}
	if !it.first {
		if cd+2 > size {
			return leafItem{}, corruptf("block %d: item %d component truncated", b.n, i)
		}
		it.component = int(binary.BigEndian.Uint16(d[o+cd:]))
		cd += 2
		if it.component < 2 {
			return leafItem{}, corruptf("block %d: item %d has component %d", b.n, i, it.component)
		}
	}
	if cd > size {
		return leafItem{}, corruptf("block %d: item %d key overruns item", b.n, i)
	}
	it.key = d[o+3 : o+3+klen]
	it.chunk = d[o+cd : o+size]
	return it, nil
}

// branch decodes branch item i: [child u32][K u8][key K bytes][X u16].
func (b *block) branch(i int) (branchItem, error) {
	o, err := b.itemOffset(i)
	if err != nil {
		return branchItem{}, err
	}
	d := b.data
	if o+5 > len(d) {
		return branchItem{}, corruptf("block %d: branch item %d truncated", b.n, i)
	}
	klen := int(d[o+4])
	if o+5+klen+2 > len(d) {
		return branchItem{}, corruptf("block %d: branch item %d overruns block", b.n, i)
	}
	return branchItem{
		child:     binary.BigEndian.Uint32(d[o:]),
		key:       d[o+5 : o+5+klen],
		component: int(binary.BigEndian.Uint16(d[o+5+klen:])),
	}, nil
}

// compareItem orders (key, component) pairs the way glass does: bytewise key
// order (a proper prefix sorts first), then component number.
func compareItem(ak []byte, ac int, bk []byte, bc int) int {
	if c := bytes.Compare(ak, bk); c != 0 {
		return c
	}
	switch {
	case ac < bc:
		return -1
	case ac > bc:
		return 1
	}
	return 0
}

// findBranch returns the index of the child to descend into for key: the last
// item <= (key, 1). Item 0 is the leftmost child and is never compared.
func (b *block) findBranch(key []byte) (int, error) {
	i, j := 0, b.count
	for j-i > 1 {
		k := i + (j-i)/2
		it, err := b.branch(k)
		if err != nil {
			return 0, err
		}
		c := compareItem(key, 1, it.key, it.component)
		if c < 0 {
			j = k
		} else {
			i = k
			if c == 0 {
				break
			}
		}
	}
	return i, nil
}

// findLeaf returns the last item <= (key, 1), or -1 when key sorts before
// every item of the block. exact reports a key match with component 1.
func (b *block) findLeaf(key []byte) (int, bool, error) {
	i, j := -1, b.count
	for j-i > 1 {
		k := i + (j-i)/2
		it, err := b.leaf(k)
		if err != nil {
			return 0, false, err
		}
		c := compareItem(key, 1, it.key, it.component)
		if c < 0 {
			j = k
		} else {
			i = k
			if c == 0 {
				return k, true, nil
			}
		}
	}
	return i, false, nil
}
