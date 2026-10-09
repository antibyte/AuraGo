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
	db     *Database
	slot   uint32
	prefix []byte
	loaded bool
	data   []byte // undecoded rest of the chunk
	did    uint32
	val    []byte
}

func (d *Database) newValueReader(slot uint32) *valueReader {
	prefix := appendUint(append([]byte(nil), keyPrefixValueChunk...), uint64(slot))
	return &valueReader{db: d, slot: slot, prefix: prefix}
}

// load reads the chunk that may contain did.
func (v *valueReader) load(did uint32) (bool, error) {
	v.loaded = false
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
	first, n, err := unpackSortableUint(key[len(v.prefix):])
	if err != nil {
		return false, err
	}
	if len(v.prefix)+n != len(key) || first == 0 || first > 0xffffffff {
		return false, corruptf("bad value chunk key")
	}
	tag, err := c.tag()
	if err != nil {
		return false, err
	}
	val, rest, err := unpackString(tag)
	if err != nil {
		return false, err
	}
	v.did, v.val, v.data, v.loaded = uint32(first), val, rest, true
	return true, nil
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
		if ok, err := v.load(did); err != nil || !ok {
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
			return "", nil // the chunk that would hold did ends before it
		}
		if ok, err := v.load(did); err != nil || !ok {
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
