// Package zimtest builds synthetic ZIM archives for tests and provides the
// optional real-world fixture. It must not import internal/zim, so that the
// zim package's own tests can use it.
package zimtest

import (
	"bytes"
	"crypto/md5" //nolint:gosec // MD5 is mandated by the ZIM checksum format, not a security choice
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// Compression is the low nibble of a cluster info byte.
type Compression byte

const (
	CompressionNone Compression = 1
	CompressionXZ   Compression = 4
	CompressionZstd Compression = 5
)

// ClusterID identifies a cluster added to a Builder.
type ClusterID int

// Entry describes one directory entry. Redirect entries set Redirect to
// "<ns>/<path>"; content entries set MimeType, Data and Cluster.
type Entry struct {
	Namespace byte
	Path      string
	Title     string
	MimeType  string
	Data      []byte
	Cluster   ClusterID
	Blob      int  // blob number inside a raw cluster (ignored otherwise)
	Front     bool // listed in X/listing/titleOrdered/v1
	Redirect  string
	// RawRedirectIndex, when non-nil, is written verbatim as the redirect
	// target index (for corrupt-archive tests).
	RawRedirectIndex *uint32
}

type clusterSpec struct {
	comp     Compression
	extended bool
	raw      []byte // non-nil: verbatim uncompressed cluster data
	blobs    [][]byte
}

// Builder assembles a ZIM archive in memory.
type Builder struct {
	Major, Minor uint16
	UUID         [16]byte
	// TitleListV1 writes X/listing/titleOrdered/v1 for Front entries.
	TitleListV1 bool
	// CompressTitleListV1 stores that listing in a zstd cluster (spec violation).
	CompressTitleListV1 bool
	// TitleListV0 writes the header's title pointer list (all entries).
	TitleListV0 bool

	clusters   []*clusterSpec
	entries    []Entry
	headerMain string // "<ns>/<path>" used when there is no W/mainPage
}

// New returns a builder for a new-namespace (6.2) archive with a v1 title list.
func New() *Builder {
	b := &Builder{Major: 6, Minor: 2, TitleListV1: true}
	copy(b.UUID[:], "aurago-zimtest-1")
	return b
}

// AddCluster adds an empty cluster that collects Data of the entries added to it.
func (b *Builder) AddCluster(c Compression, extended bool) ClusterID {
	b.clusters = append(b.clusters, &clusterSpec{comp: c, extended: extended})
	return ClusterID(len(b.clusters) - 1)
}

// AddRawCluster adds a cluster whose uncompressed data (offset table and
// blobs) is written verbatim and then compressed with c.
func (b *Builder) AddRawCluster(c Compression, extended bool, raw []byte) ClusterID {
	b.clusters = append(b.clusters, &clusterSpec{comp: c, extended: extended, raw: raw})
	return ClusterID(len(b.clusters) - 1)
}

// Add appends a directory entry.
func (b *Builder) Add(e Entry) { b.entries = append(b.entries, e) }

// AddArticle adds a front HTML article.
func (b *Builder) AddArticle(c ClusterID, ns byte, path, title, html string) {
	b.Add(Entry{Namespace: ns, Path: path, Title: title, MimeType: "text/html", Data: []byte(html), Cluster: c, Front: true})
}

// AddMetadata adds M/<name>.
func (b *Builder) AddMetadata(c ClusterID, name, value string) {
	b.Add(Entry{Namespace: 'M', Path: name, MimeType: "text/plain", Data: []byte(value), Cluster: c})
}

// AddRedirect adds a redirect from ns/path to target ("<ns>/<path>").
func (b *Builder) AddRedirect(ns byte, path, title, target string) {
	b.Add(Entry{Namespace: ns, Path: path, Title: title, Redirect: target})
}

// SetHeaderMainPage points the header mainPage at target ("<ns>/<path>")
// when the archive has no W/mainPage entry.
func (b *Builder) SetHeaderMainPage(target string) { b.headerMain = target }

// Layout reports where the structures of a built archive are.
type Layout struct {
	EntryIndex     map[string]uint32 // "<ns>/<path>" → path-order index
	DirentOffset   map[string]int64
	ClusterOffsets []int64
	MimeTypes      []string
	PathPtrPos     int64
	TitlePtrPos    int64 // -1 without a v0 list
	ClusterPtrPos  int64
	ChecksumPos    int64
}

type builtEntry struct {
	Entry
	key     string
	mime    uint16
	cluster uint32
	blob    uint32
}

func key(ns byte, path string) string { return string(ns) + "/" + path }

func titleOf(e Entry) string {
	if e.Title != "" {
		return e.Title
	}
	return e.Path
}

// validateEntry rejects entries that would silently corrupt the archive: the
// MIME list and the directory entries are NUL-terminated strings, and an empty
// MIME type would end the MIME list early. Corrupt-archive tests that need such
// bytes mutate the built archive using Layout, or use RawRedirectIndex and
// AddRawCluster, which are not affected by these checks.
func validateEntry(e Entry) error {
	k := key(e.Namespace, e.Path)
	if strings.IndexByte(e.Path, 0) >= 0 {
		return fmt.Errorf("zimtest: entry %q has a NUL byte in its path", k)
	}
	if strings.IndexByte(e.Title, 0) >= 0 {
		return fmt.Errorf("zimtest: entry %q has a NUL byte in its title", k)
	}
	if e.Redirect != "" || e.RawRedirectIndex != nil {
		return nil
	}
	if e.MimeType == "" {
		return fmt.Errorf("zimtest: content entry %q has an empty MIME type", k)
	}
	if strings.IndexByte(e.MimeType, 0) >= 0 {
		return fmt.Errorf("zimtest: content entry %q has a NUL byte in its MIME type", k)
	}
	return nil
}

// Build serialises the archive.
func (b *Builder) Build() ([]byte, *Layout, error) {
	for _, e := range b.entries {
		if err := validateEntry(e); err != nil {
			return nil, nil, err
		}
	}
	entries := append([]Entry(nil), b.entries...)
	clusters := append([]*clusterSpec(nil), b.clusters...)
	for i, c := range clusters {
		cc := *c
		cc.blobs = nil
		clusters[i] = &cc
	}
	var front []Entry
	for _, e := range entries {
		if e.Front {
			front = append(front, e)
		}
	}
	if b.TitleListV1 && len(front) > 0 {
		comp := CompressionNone
		if b.CompressTitleListV1 {
			comp = CompressionZstd
		}
		clusters = append(clusters, &clusterSpec{comp: comp})
		entries = append(entries, Entry{Namespace: 'X', Path: "listing/titleOrdered/v1",
			MimeType: "application/octet-stream+zimlisting", Data: make([]byte, 4*len(front)),
			Cluster: ClusterID(len(clusters) - 1)})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Namespace != entries[j].Namespace {
			return entries[i].Namespace < entries[j].Namespace
		}
		return entries[i].Path < entries[j].Path
	})
	layout := &Layout{EntryIndex: map[string]uint32{}, DirentOffset: map[string]int64{}, TitlePtrPos: -1}
	for i, e := range entries {
		k := key(e.Namespace, e.Path)
		if _, dup := layout.EntryIndex[k]; dup {
			return nil, nil, fmt.Errorf("zimtest: duplicate entry %s", k)
		}
		layout.EntryIndex[k] = uint32(i)
	}

	// The v1 listing holds front entry indexes sorted by title.
	if b.TitleListV1 && len(front) > 0 {
		sort.SliceStable(front, func(i, j int) bool { return titleOf(front[i]) < titleOf(front[j]) })
		listing := make([]byte, 4*len(front))
		for i, e := range front {
			binary.LittleEndian.PutUint32(listing[4*i:], layout.EntryIndex[key(e.Namespace, e.Path)])
		}
		for i := range entries {
			if entries[i].Namespace == 'X' && entries[i].Path == "listing/titleOrdered/v1" {
				entries[i].Data = listing
			}
		}
	}

	mimeSet := map[string]bool{}
	for _, e := range entries {
		if e.Redirect == "" && e.RawRedirectIndex == nil {
			mimeSet[e.MimeType] = true
		}
	}
	for m := range mimeSet {
		layout.MimeTypes = append(layout.MimeTypes, m)
	}
	sort.Strings(layout.MimeTypes)
	mimeIndex := map[string]uint16{}
	for i, m := range layout.MimeTypes {
		mimeIndex[m] = uint16(i)
	}

	built := make([]builtEntry, len(entries))
	for i, e := range entries {
		be := builtEntry{Entry: e, key: key(e.Namespace, e.Path)}
		if e.Redirect == "" && e.RawRedirectIndex == nil {
			if int(e.Cluster) < 0 || int(e.Cluster) >= len(clusters) {
				return nil, nil, fmt.Errorf("zimtest: %s uses unknown cluster %d", be.key, e.Cluster)
			}
			c := clusters[e.Cluster]
			be.mime = mimeIndex[e.MimeType]
			be.cluster = uint32(e.Cluster)
			if c.raw != nil {
				be.blob = uint32(e.Blob)
			} else {
				be.blob = uint32(len(c.blobs))
				c.blobs = append(c.blobs, e.Data)
			}
		}
		built[i] = be
	}

	var out bytes.Buffer
	out.Write(make([]byte, 80)) // header, filled in at the end
	for _, m := range layout.MimeTypes {
		out.WriteString(m)
		out.WriteByte(0)
	}
	out.WriteByte(0)

	for _, c := range clusters {
		layout.ClusterOffsets = append(layout.ClusterOffsets, int64(out.Len()))
		data, err := encodeCluster(c)
		if err != nil {
			return nil, nil, err
		}
		out.Write(data)
	}

	direntOffsets := make([]uint64, len(built))
	for i, be := range built {
		direntOffsets[i] = uint64(out.Len())
		layout.DirentOffset[be.key] = int64(out.Len())
		d, err := encodeDirent(be, layout.EntryIndex)
		if err != nil {
			return nil, nil, err
		}
		out.Write(d)
	}

	layout.PathPtrPos = int64(out.Len())
	for _, off := range direntOffsets {
		_ = binary.Write(&out, binary.LittleEndian, off)
	}

	if b.TitleListV0 {
		layout.TitlePtrPos = int64(out.Len())
		order := make([]int, len(built))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(i, j int) bool {
			a, c := built[order[i]], built[order[j]]
			if a.Namespace != c.Namespace {
				return a.Namespace < c.Namespace
			}
			return titleOf(a.Entry) < titleOf(c.Entry)
		})
		for _, idx := range order {
			_ = binary.Write(&out, binary.LittleEndian, uint32(idx))
		}
	}

	layout.ClusterPtrPos = int64(out.Len())
	for _, off := range layout.ClusterOffsets {
		_ = binary.Write(&out, binary.LittleEndian, uint64(off))
	}
	layout.ChecksumPos = int64(out.Len())

	data := out.Bytes()
	le := binary.LittleEndian
	le.PutUint32(data[0:], 0x044D495A)
	le.PutUint16(data[4:], b.Major)
	le.PutUint16(data[6:], b.Minor)
	copy(data[8:24], b.UUID[:])
	le.PutUint32(data[24:], uint32(len(built)))
	le.PutUint32(data[28:], uint32(len(clusters)))
	le.PutUint64(data[32:], uint64(layout.PathPtrPos))
	titlePos := ^uint64(0)
	if layout.TitlePtrPos >= 0 {
		titlePos = uint64(layout.TitlePtrPos)
	}
	le.PutUint64(data[40:], titlePos)
	le.PutUint64(data[48:], uint64(layout.ClusterPtrPos))
	le.PutUint64(data[56:], 80)
	mainPage := uint32(0xFFFFFFFF)
	if idx, ok := layout.EntryIndex["W/mainPage"]; ok {
		mainPage = idx
	} else if b.headerMain != "" {
		idx, ok := layout.EntryIndex[b.headerMain]
		if !ok {
			return nil, nil, fmt.Errorf("zimtest: header main page %s does not exist", b.headerMain)
		}
		mainPage = idx
	}
	le.PutUint32(data[64:], mainPage)
	le.PutUint32(data[68:], 0xFFFFFFFF)
	le.PutUint64(data[72:], uint64(layout.ChecksumPos))
	sum := md5.Sum(data) //nolint:gosec // ZIM checksum format
	return append(data, sum[:]...), layout, nil
}

func encodeDirent(be builtEntry, index map[string]uint32) ([]byte, error) {
	var d bytes.Buffer
	le := binary.LittleEndian
	if be.Redirect != "" || be.RawRedirectIndex != nil {
		target := uint32(0)
		if be.RawRedirectIndex != nil {
			target = *be.RawRedirectIndex
		} else {
			idx, ok := index[be.Redirect]
			if !ok {
				return nil, fmt.Errorf("zimtest: redirect %s targets missing %s", be.key, be.Redirect)
			}
			target = idx
		}
		_ = binary.Write(&d, le, uint16(0xFFFF))
		d.WriteByte(0) // parameter length
		d.WriteByte(be.Namespace)
		_ = binary.Write(&d, le, uint32(0)) // revision
		_ = binary.Write(&d, le, target)
	} else {
		_ = binary.Write(&d, le, be.mime)
		d.WriteByte(0)
		d.WriteByte(be.Namespace)
		_ = binary.Write(&d, le, uint32(0))
		_ = binary.Write(&d, le, be.cluster)
		_ = binary.Write(&d, le, be.blob)
	}
	d.WriteString(be.Path)
	d.WriteByte(0)
	if be.Title != be.Path {
		d.WriteString(be.Title)
	}
	d.WriteByte(0)
	return d.Bytes(), nil
}

func encodeCluster(c *clusterSpec) ([]byte, error) {
	raw := c.raw
	if raw == nil {
		raw = clusterBody(c.blobs, c.extended)
	}
	info := byte(c.comp)
	if c.extended {
		info |= 0x10
	}
	var payload []byte
	switch c.comp {
	case CompressionZstd:
		enc, err := zstd.NewWriter(nil)
		if err != nil {
			return nil, err
		}
		payload = enc.EncodeAll(raw, nil)
		_ = enc.Close()
	case CompressionXZ:
		var buf bytes.Buffer
		w, err := xz.NewWriter(&buf)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(raw); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		payload = buf.Bytes()
	default:
		payload = raw
	}
	return append([]byte{info}, payload...), nil
}

// clusterBody lays out an offset table (one offset per blob plus the end)
// followed by the blob bytes.
func clusterBody(blobs [][]byte, extended bool) []byte {
	width := 4
	if extended {
		width = 8
	}
	offsets := make([]uint64, 0, len(blobs)+1)
	pos := uint64(width * (len(blobs) + 1))
	for _, blob := range blobs {
		offsets = append(offsets, pos)
		pos += uint64(len(blob))
	}
	offsets = append(offsets, pos)
	var buf bytes.Buffer
	for _, off := range offsets {
		if extended {
			_ = binary.Write(&buf, binary.LittleEndian, off)
		} else {
			_ = binary.Write(&buf, binary.LittleEndian, uint32(off))
		}
	}
	for _, blob := range blobs {
		buf.Write(blob)
	}
	return buf.Bytes()
}

// ClusterBody exposes the offset-table layout for raw-cluster tests.
func ClusterBody(blobs [][]byte, extended bool) []byte { return clusterBody(blobs, extended) }

// WriteFile builds the archive into tb.TempDir() and returns its path.
func (b *Builder) WriteFile(tb testing.TB) (string, *Layout) {
	tb.Helper()
	data, layout, err := b.Build()
	if err != nil {
		tb.Fatalf("build ZIM: %v", err)
	}
	return WriteBytes(tb, data), layout
}

// WriteBytes stores (possibly mutated) archive bytes in tb.TempDir().
func WriteBytes(tb testing.TB, data []byte) string {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), "test.zim")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		tb.Fatalf("write ZIM: %v", err)
	}
	return path
}
