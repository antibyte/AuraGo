package xapian

import (
	"bytes"
	"compress/flate"
	"errors"
	"fmt"
	"io"
)

// maxTagSize bounds one reassembled (and inflated) tag; real tags are a few KiB.
const maxTagSize = 16 << 20

// table is one glass B-tree inside the single-file database.
type table struct {
	name      string
	r         io.ReaderAt
	blockSize int
	nblocks   uint32
	root      uint32
	level     int
	empty     bool // root_is_fake: the table holds no items
	cache     *blockCache
}

func (t *table) readBlock(n uint32, wantLevel int) (*block, error) {
	if n == 0 || n >= t.nblocks {
		return nil, corruptf("%s: block %d outside database (%d blocks)", t.name, n, t.nblocks)
	}
	if b := t.cache.get(n); b != nil {
		if b.level != wantLevel {
			return nil, corruptf("%s: block %d has level %d, want %d", t.name, n, b.level, wantLevel)
		}
		return b, nil
	}
	buf := make([]byte, t.blockSize)
	if _, err := t.r.ReadAt(buf, int64(n)*int64(t.blockSize)); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, corruptf("%s: block %d truncated", t.name, n)
		}
		return nil, fmt.Errorf("xapian: read %s block %d: %w", t.name, n, err)
	}
	b, err := parseBlock(n, buf)
	if err != nil {
		return nil, err
	}
	if b.level != wantLevel {
		return nil, corruptf("%s: block %d has level %d, want %d", t.name, n, b.level, wantLevel)
	}
	t.cache.put(b)
	return b, nil
}

// cursor walks the leaf items of a table in key order. path[0] is the leaf.
type cursor struct {
	t     *table
	path  []cursorLevel
	valid bool
}

type cursorLevel struct {
	b *block
	i int
}

func (t *table) cursor() *cursor {
	return &cursor{t: t, path: make([]cursorLevel, t.level+1)}
}

func (c *cursor) clone() *cursor {
	cp := &cursor{t: c.t, valid: c.valid, path: make([]cursorLevel, len(c.path))}
	copy(cp.path, c.path)
	return cp
}

func (c *cursor) item() (leafItem, error) {
	return c.path[0].b.leaf(c.path[0].i)
}

// descend positions the path below level lvl on child items, leftmost when
// first is true, rightmost otherwise.
func (c *cursor) descend(lvl int, first bool) error {
	for l := lvl; l > 0; l-- {
		it, err := c.path[l].b.branch(c.path[l].i)
		if err != nil {
			return err
		}
		b, err := c.t.readBlock(it.child, l-1)
		if err != nil {
			return err
		}
		i := 0
		if !first {
			i = b.count - 1
		}
		c.path[l-1] = cursorLevel{b: b, i: i}
	}
	return nil
}

// nextItem moves to the following leaf item; false at the end of the table.
func (c *cursor) nextItem() (bool, error) {
	if c.path[0].i+1 < c.path[0].b.count {
		c.path[0].i++
		return true, nil
	}
	lvl := 1
	for lvl <= c.t.level && c.path[lvl].i+1 >= c.path[lvl].b.count {
		lvl++
	}
	if lvl > c.t.level {
		return false, nil
	}
	c.path[lvl].i++
	return true, c.descend(lvl, true)
}

// prevItem moves to the preceding leaf item; false at the start of the table.
func (c *cursor) prevItem() (bool, error) {
	if c.path[0].i > 0 {
		c.path[0].i--
		return true, nil
	}
	lvl := 1
	for lvl <= c.t.level && c.path[lvl].i == 0 {
		lvl++
	}
	if lvl > c.t.level {
		return false, nil
	}
	c.path[lvl].i--
	return true, c.descend(lvl, false)
}

// seekLE positions the cursor on the entry (first component of a tag) with the
// greatest key <= key. found is false when no such entry exists; exact reports
// an entry whose key equals key.
func (c *cursor) seekLE(key []byte) (found, exact bool, err error) {
	c.valid = false
	if c.t.empty {
		return false, false, nil
	}
	n := c.t.root
	for lvl := c.t.level; lvl > 0; lvl-- {
		b, err := c.t.readBlock(n, lvl)
		if err != nil {
			return false, false, err
		}
		i, err := b.findBranch(key)
		if err != nil {
			return false, false, err
		}
		c.path[lvl] = cursorLevel{b: b, i: i}
		it, err := b.branch(i)
		if err != nil {
			return false, false, err
		}
		n = it.child
	}
	leaf, err := c.t.readBlock(n, 0)
	if err != nil {
		return false, false, err
	}
	i, exact, err := leaf.findLeaf(key)
	if err != nil {
		return false, false, err
	}
	if i < 0 {
		c.path[0] = cursorLevel{b: leaf, i: 0}
		ok, err := c.prevItem()
		if err != nil || !ok {
			return false, false, err
		}
	} else {
		c.path[0] = cursorLevel{b: leaf, i: i}
	}
	for {
		it, err := c.item()
		if err != nil {
			return false, false, err
		}
		if it.first {
			break
		}
		ok, err := c.prevItem()
		if err != nil {
			return false, false, err
		}
		if !ok {
			return false, false, corruptf("%s: continuation item without first component", c.t.name)
		}
	}
	c.valid = true
	return true, exact, nil
}

// next moves to the following entry, skipping continuation components.
func (c *cursor) next() (bool, error) {
	if !c.valid {
		return false, nil
	}
	for {
		ok, err := c.nextItem()
		if err != nil || !ok {
			c.valid = false
			return false, err
		}
		it, err := c.item()
		if err != nil {
			c.valid = false
			return false, err
		}
		if it.first {
			return true, nil
		}
	}
}

// seekGE positions the cursor on the first entry with key >= key.
func (c *cursor) seekGE(key []byte) (bool, error) {
	found, exact, err := c.seekLE(key)
	if err != nil {
		return false, err
	}
	if exact {
		return true, nil
	}
	if !found {
		// key sorts before every entry: start at the first one.
		if c.t.empty {
			return false, nil
		}
		return c.first()
	}
	return c.next()
}

// first positions the cursor on the first entry of the table.
func (c *cursor) first() (bool, error) {
	b, err := c.t.readBlock(c.t.root, c.t.level)
	if err != nil {
		return false, err
	}
	c.path[c.t.level] = cursorLevel{b: b, i: 0}
	if err := c.descend(c.t.level, true); err != nil {
		return false, err
	}
	it, err := c.item()
	if err != nil {
		return false, err
	}
	c.valid = true
	if !it.first {
		return c.next()
	}
	return true, nil
}

// key returns the current entry key (valid until the cursor moves).
func (c *cursor) key() ([]byte, error) {
	it, err := c.item()
	if err != nil {
		return nil, err
	}
	return it.key, nil
}

// tag reassembles the current entry's tag from all its components and
// inflates it when the first component carries the compressed flag. The
// cursor itself does not move.
func (c *cursor) tag() ([]byte, error) {
	w := c.clone()
	it, err := w.item()
	if err != nil {
		return nil, err
	}
	if !it.first {
		return nil, corruptf("%s: tag read not at first component", c.t.name)
	}
	key := append([]byte(nil), it.key...)
	compressed := it.compressed
	buf := append([]byte(nil), it.chunk...)
	want := 2
	for !it.last {
		ok, err := w.nextItem()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, corruptf("%s: tag of key %q ends early", c.t.name, key)
		}
		if it, err = w.item(); err != nil {
			return nil, err
		}
		if it.first || it.component != want || !bytes.Equal(it.key, key) {
			return nil, corruptf("%s: broken component sequence for key %q", c.t.name, key)
		}
		want++
		if len(buf)+len(it.chunk) > maxTagSize {
			return nil, corruptf("%s: tag of key %q too large", c.t.name, key)
		}
		buf = append(buf, it.chunk...)
	}
	if !compressed {
		return buf, nil
	}
	zr := flate.NewReader(bytes.NewReader(buf))
	defer zr.Close()
	out, err := io.ReadAll(io.LimitReader(zr, maxTagSize+1))
	if err != nil {
		return nil, corruptf("%s: inflate tag of key %q: %v", c.t.name, key, err)
	}
	if len(out) > maxTagSize {
		return nil, corruptf("%s: inflated tag of key %q too large", c.t.name, key)
	}
	return out, nil
}

// get returns the tag stored under exactly key.
func (t *table) get(key []byte) ([]byte, bool, error) {
	c := t.cursor()
	_, exact, err := c.seekLE(key)
	if err != nil || !exact {
		return nil, false, err
	}
	tag, err := c.tag()
	if err != nil {
		return nil, false, err
	}
	return tag, true, nil
}
