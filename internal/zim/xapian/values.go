package xapian

import "bytes"

// Value returns the value in slot for did ("" when unset). libzim stores the
// title in slot 0; slot 1 is the word count (full-text) or target path (title).
func (d *Database) Value(did, slot uint32) (string, error) {
	if did == 0 || did > d.info.lastDocID {
		return "", ErrDocNotFound
	}
	r := d.newValueReader(slot)
	return r.get(did)
}

// valueReader reads one slot's value stream; lookups in ascending docid order
// reuse the decoded chunk.
type valueReader struct {
	db         *Database
	slot       uint32
	prefix     []byte
	loaded     bool
	c          *cursor // on the current chunk until checkNextChunk steps past it
	chunkFirst uint32  // first docid of the current chunk
	data       []byte  // undecoded rest of the chunk (may alias a cached block)
	did        uint32
	val        []byte
}

func (d *Database) newValueReader(slot uint32) *valueReader {
	prefix := appendUint(append([]byte(nil), keyPrefixValueChunk...), uint64(slot))
	return &valueReader{db: d, slot: slot, prefix: prefix}
}

// load reads the chunk that may contain did: the last chunk starting at or
// before did. A forward load, after the reader ran off the end of its chunk,
// passes that chunk's first docid as notBefore and so can never land earlier
// than the chunk it just left. Seeks trust the branch separators, and a tree
// whose separators steer them left keeps every walk in key order, so the
// cursor cannot notice; without this check such a tree would send a reader
// that moves forward back to the same early chunk on every lookup.
func (v *valueReader) load(did, notBefore uint32) (bool, error) {
	v.loaded, v.c = false, nil
	c := v.db.postlist.cursor()
	found, _, err := c.seekLE(valueChunkKey(v.slot, did))
	if err != nil || !found {
		return false, err
	}
	key, err := c.key()
	if err != nil {
		return false, err
	}
	if !bytes.HasPrefix(key, v.prefix) {
		return false, nil
	}
	first, err := v.chunkStart(key)
	if err != nil {
		return false, err
	}
	if first < notBefore {
		return false, corruptf("value slot %d: seek for docid %d landed on the chunk at %d, before the current chunk at %d", v.slot, did, first, notBefore)
	}
	tag, err := c.tagView() // read in place: forward readers load chunk after chunk
	if err != nil {
		return false, err
	}
	val, rest, err := unpackString(tag)
	if err != nil {
		return false, err
	}
	v.did, v.val, v.data, v.loaded = first, val, rest, true
	v.c, v.chunkFirst = c, first
	return true, nil
}

// chunkStart returns the first docid of the value chunk stored under key,
// which has the reader's slot prefix.
func (v *valueReader) chunkStart(key []byte) (uint32, error) {
	first, n, err := unpackSortableUint(key[len(v.prefix):])
	if err != nil {
		return 0, err
	}
	if len(v.prefix)+n != len(key) || first == 0 || first > 0xffffffff {
		return 0, corruptf("bad value chunk key")
	}
	return uint32(first), nil
}

// checkNextChunk is called when a freshly loaded chunk ended before did. A
// valid tree landed on the last chunk starting at or before did, so the chunk
// after it, if it belongs to this slot, starts past did. Anything else is a
// seek that was steered to the wrong chunk (the lookup would answer "unset"
// for a stored value and, repeated, walk the same chunk again each time).
func (v *valueReader) checkNextChunk(did uint32) error {
	c := v.c
	v.c = nil
	if c == nil {
		return nil
	}
	ok, err := c.next()
	if err != nil || !ok {
		return err
	}
	key, err := c.key()
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(key, v.prefix) {
		return nil
	}
	next, err := v.chunkStart(key)
	if err != nil {
		return err
	}
	if next <= did {
		return corruptf("value slot %d: seek for docid %d stopped at the chunk at %d, but the chunk at %d follows", v.slot, did, v.chunkFirst, next)
	}
	return nil
}

// step decodes the next (docid, value) pair of the chunk.
func (v *valueReader) step() (bool, error) {
	if len(v.data) == 0 {
		return false, nil
	}
	delta, n, err := unpackUint32(v.data)
	if err != nil {
		return false, err
	}
	next := uint64(v.did) + uint64(delta) + 1
	if next > 0xffffffff {
		return false, corruptf("value docid overflows")
	}
	val, rest, err := unpackString(v.data[n:])
	if err != nil {
		return false, err
	}
	v.did, v.val, v.data = uint32(next), val, rest
	return true, nil
}

func (v *valueReader) get(did uint32) (string, error) {
	reloaded := false
	if !v.loaded || did < v.did {
		if ok, err := v.load(did, 0); err != nil || !ok {
			return "", err
		}
		reloaded = true
	}
	for v.did < did {
		ok, err := v.step()
		if err != nil {
			return "", err
		}
		if ok {
			continue
		}
		if reloaded {
			// The chunk that would hold did ends before it.
			return "", v.checkNextChunk(did)
		}
		if ok, err := v.load(did, v.chunkFirst); err != nil || !ok {
			return "", err
		}
		reloaded = true
	}
	if v.did != did {
		return "", nil
	}
	return string(v.val), nil
}

// unpackString reads a varint length followed by that many bytes.
func unpackString(b []byte) ([]byte, []byte, error) {
	l, n, err := unpackUint(b)
	if err != nil {
		return nil, nil, err
	}
	if l > uint64(len(b)-n) {
		return nil, nil, corruptf("string overruns data")
	}
	return b[n : n+int(l)], b[n+int(l):], nil
}
