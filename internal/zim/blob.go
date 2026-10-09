package zim

import (
	"bytes"
	"fmt"
	"io"
)

const maxMetadataBytes = 1 << 20

// Open returns the content of a non-redirect entry. Blobs in uncompressed
// clusters (images, X/ indexes) are sections directly on the file; blobs in
// compressed clusters are sections over the cached decompressed cluster.
// Reading from either kind of section after Close fails with ErrClosed. The
// zero Entry (one that no Archive returned) is rejected with ErrNotFound.
func (a *Archive) Open(e Entry) (*io.SectionReader, error) {
	switch e.kind {
	case kindContent:
	case kindRedirect:
		return nil, fmt.Errorf("%w: %c/%s", ErrIsRedirect, e.Namespace, e.Path)
	case kindDeprecated:
		return nil, fmt.Errorf("%w: %c/%s has no content", ErrNotFound, e.Namespace, e.Path)
	default:
		return nil, fmt.Errorf("%w: entry %c/%s was not returned by an archive", ErrNotFound, e.Namespace, e.Path)
	}
	r, err := a.openBlob(e.cluster, e.blob)
	if err != nil {
		return nil, fmt.Errorf("open %c/%s: %w", e.Namespace, e.Path, err)
	}
	return r, nil
}

func (a *Archive) openBlob(cluster, blob uint32) (*io.SectionReader, error) {
	ci, err := a.clusterInfo(cluster)
	if err != nil {
		return nil, err
	}
	if !ci.compressed() {
		off, n, err := a.uncompressedBlob(ci, blob)
		if err != nil {
			return nil, err
		}
		return io.NewSectionReader(a.r, off, n), nil
	}
	cd, err := a.cache.get(cluster, func() (*clusterData, error) { return a.loadCluster(cluster, ci) })
	if err != nil {
		return nil, err
	}
	b, err := cd.blob(blob)
	if err != nil {
		return nil, err
	}
	return io.NewSectionReader(a.guard(bytes.NewReader(b)), 0, int64(len(b))), nil
}

func (a *Archive) clusterInfo(idx uint32) (clusterInfo, error) {
	if idx >= a.hdr.clusterCount {
		return clusterInfo{}, errCorrupt("cluster %d out of range (archive has %d)", idx, a.hdr.clusterCount)
	}
	var p [8]byte
	if err := a.readAt(p[:], int64(a.hdr.clusterPtrPos)+8*int64(idx)); err != nil {
		return clusterInfo{}, err
	}
	off := readOffset(p[:], 8)
	if off < a.hdr.mimeListPos || off >= uint64(a.dataEnd) {
		return clusterInfo{}, errCorrupt("cluster %d points to offset %d outside the archive", idx, off)
	}
	var info [1]byte
	if err := a.readAt(info[:], int64(off)); err != nil {
		return clusterInfo{}, err
	}
	ci := clusterInfo{offset: int64(off), comp: info[0] & 0x0F, extended: info[0]&clusterExtendedFlag != 0}
	if ci.extended && a.hdr.major < 6 {
		return clusterInfo{}, errCorrupt("cluster %d is extended in a major version %d archive", idx, a.hdr.major)
	}
	switch ci.comp {
	case compNoneLegacy, compNone, compXZ, compZstd:
		return ci, nil
	case compZlib, compBzip2:
		return clusterInfo{}, errUnsupported("cluster %d uses discontinued compression %d", idx, ci.comp)
	}
	return clusterInfo{}, errCorrupt("cluster %d has unknown compression %d", idx, ci.comp)
}

// uncompressedBlob locates blob n of an uncompressed cluster on the file.
func (a *Archive) uncompressedBlob(ci clusterInfo, n uint32) (int64, int64, error) {
	width := ci.offsetWidth()
	dataStart := ci.offset + 1
	avail := uint64(a.dataEnd - dataStart)
	head := make([]byte, width)
	if err := a.readAt(head, dataStart); err != nil {
		return 0, 0, err
	}
	first := readOffset(head, width)
	if err := validateFirstOffset(first, width, uint64(a.hdr.entryCount)); err != nil {
		return 0, 0, err
	}
	if first > avail {
		return 0, 0, errCorrupt("cluster offset table at %d exceeds the archive", ci.offset)
	}
	if uint64(n) >= first/width-1 {
		return 0, 0, errCorrupt("blob %d out of range (cluster has %d)", n, first/width-1)
	}
	pair := make([]byte, 2*width)
	if err := a.readAt(pair, dataStart+int64(uint64(n)*width)); err != nil {
		return 0, 0, err
	}
	start, end := readOffset(pair[:width], width), readOffset(pair[width:], width)
	if start < first || end < start || end > avail {
		return 0, 0, errCorrupt("blob %d of cluster at %d has invalid bounds [%d, %d)", n, ci.offset, start, end)
	}
	return dataStart + int64(start), int64(end - start), nil
}

func (a *Archive) loadCluster(idx uint32, ci clusterInfo) (*clusterData, error) {
	a.loads.Add(1)
	src := io.NewSectionReader(a.r, ci.offset+1, a.dataEnd-ci.offset-1)
	cd, err := decodeCluster(src, ci, a.lim.maxClusterBytes, uint64(a.hdr.entryCount))
	if err != nil {
		if a.closed.Load() {
			return nil, ErrClosed
		}
		return nil, fmt.Errorf("cluster %d: %w", idx, err)
	}
	return cd, nil
}

// Metadata returns the value of M/<name>; ErrNotFound if absent.
func (a *Archive) Metadata(name string) (string, error) {
	e, err := a.EntryByPath('M', name)
	if err != nil {
		return "", err
	}
	if e, err = a.Resolve(e); err != nil {
		return "", err
	}
	r, err := a.Open(e)
	if err != nil {
		return "", err
	}
	if r.Size() > maxMetadataBytes {
		return "", errUnsupported("metadata %q is %d bytes (limit %d)", name, r.Size(), maxMetadataBytes)
	}
	b := make([]byte, r.Size())
	if _, err := io.ReadFull(r, b); err != nil {
		return "", fmt.Errorf("read metadata %q: %w", name, err)
	}
	return string(b), nil
}
