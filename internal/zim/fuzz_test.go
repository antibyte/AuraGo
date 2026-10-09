package zim

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"aurago/internal/zim/zimtest"
)

const fuzzClusterLimit = 1 << 20

func FuzzParseHeader(f *testing.F) {
	data, _, err := sampleBuilder().Build()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data[:headerSize], int64(len(data)))
	f.Add(make([]byte, headerSize), int64(headerSize))
	f.Fuzz(func(t *testing.T, b []byte, size int64) {
		h, err := parseHeader(b, size)
		if err != nil {
			return
		}
		end := uint64(h.dataEnd(size))
		if end > uint64(size) || !tableFits(h.pathPtrPos, uint64(h.entryCount), 8, h.mimeListPos, end) ||
			!tableFits(h.clusterPtrPos, uint64(h.clusterCount), 8, h.mimeListPos, end) {
			t.Fatalf("accepted header with out-of-bounds tables: %+v (size %d)", h, size)
		}
	})
}

func FuzzParseDirent(f *testing.F) {
	f.Add(contentDirent(0, 'C', 1, 2, "Berlin", "Berlin"))
	f.Add(redirectDirent('W', 3, "mainPage", ""))
	f.Add([]byte{0xFE, 0xFF, 0, 'A', 0, 0, 0, 0, 'x', 0, 0})
	mimes := []string{"text/html", "image/png"}
	f.Fuzz(func(t *testing.T, b []byte) {
		e, err := parseDirent(b, mimes)
		if err != nil {
			return
		}
		if len(e.Path)+len(e.Title) > 2*len(b) || (e.kind == kindContent && e.MimeType == "") {
			t.Fatalf("implausible entry %+v from %d bytes", e, len(b))
		}
	})
}

func FuzzDecodeCluster(f *testing.F) {
	body := zimtest.ClusterBody([][]byte{[]byte("one"), []byte("two")}, false)
	f.Add(byte(compNone), body)
	f.Add(byte(compNone|clusterExtendedFlag), zimtest.ClusterBody([][]byte{[]byte("x")}, true))
	data, layout, err := sampleBuilder().Build()
	if err != nil {
		f.Fatal(err)
	}
	for i := 0; i+1 < len(layout.ClusterOffsets); i++ {
		c := data[layout.ClusterOffsets[i]:layout.ClusterOffsets[i+1]]
		f.Add(c[0], c[1:])
	}
	f.Fuzz(func(t *testing.T, info byte, payload []byte) {
		ci := clusterInfo{comp: info & 0x0F, extended: info&clusterExtendedFlag != 0}
		if !ci.compressed() && ci.comp != compNone && ci.comp != compNoneLegacy {
			return
		}
		cd, err := decodeCluster(bytes.NewReader(payload), ci, fuzzClusterLimit, 1000)
		if err != nil {
			return
		}
		if int64(len(cd.data)) > fuzzClusterLimit {
			t.Fatalf("decoded %d bytes past the %d limit", len(cd.data), fuzzClusterLimit)
		}
		// The offset table is ordered, starts right behind itself, ends at the
		// data end and every offset lies inside the data.
		tableEnd := (cd.count + 1) * cd.width
		prev := tableEnd
		for i := uint64(0); i <= cd.count; i++ {
			off := readOffset(cd.data[i*cd.width:(i+1)*cd.width], cd.width)
			if off < prev || off > uint64(len(cd.data)) || (i == 0 && off != tableEnd) {
				t.Fatalf("offset %d = %d (previous %d, table end %d, data %d bytes)", i, off, prev, tableEnd, len(cd.data))
			}
			prev = off
		}
		if prev != uint64(len(cd.data)) {
			t.Fatalf("last offset %d, data has %d bytes", prev, len(cd.data))
		}
		for n := uint32(0); uint64(n) < cd.count; n++ {
			if _, err := cd.blob(n); err != nil {
				t.Fatalf("validated cluster has unreadable blob %d: %v", n, err)
			}
		}
	})
}

// legacyBuilder is a pre-5.1 style archive: minor version 0, 'A' content
// namespace, a v0 title list and no v1 listing.
func legacyBuilder() *zimtest.Builder {
	b := zimtest.New()
	b.Minor = 0
	b.TitleListV1 = false
	b.TitleListV0 = true
	c := b.AddCluster(zimtest.CompressionZstd, false)
	b.AddArticle(c, 'A', "Zebra", "Zebra", "<p>zebra</p>")
	b.AddArticle(c, 'A', "Apfel", "Apfel", "<p>apfel</p>")
	b.Add(zimtest.Entry{Namespace: '-', Path: "style.css", MimeType: "text/css", Data: []byte("p{}"), Cluster: c})
	b.Add(zimtest.Entry{Namespace: 'I', Path: "logo.png", MimeType: "image/png", Data: []byte("png"), Cluster: c})
	b.AddMetadata(c, "Language", "deu")
	b.SetHeaderMainPage("A/Zebra")
	return b
}

// readErrorIsClassified reports whether err is one of the errors the reader
// documents for a well-formed call on a damaged archive. The fuzz input is
// in memory, so a genuine I/O error cannot occur.
func readErrorIsClassified(err error) bool {
	for _, target := range []error{ErrNotFound, ErrCorrupt, ErrUnsupported, ErrRedirectLoop, ErrIsRedirect} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func FuzzOpenArchive(f *testing.F) {
	compressedV1 := sampleBuilder()
	compressedV1.TitleListV0 = true
	compressedV1.CompressTitleListV1 = true
	for _, b := range []*zimtest.Builder{sampleBuilder(), compressedV1, legacyBuilder()} {
		data, _, err := b.Build()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		a, err := newArchive(bytes.NewReader(b), int64(len(b)), Options{ClusterCacheBytes: fuzzClusterLimit}, limits{maxClusterBytes: fuzzClusterLimit})
		if err != nil {
			if !errors.Is(err, ErrNotZIM) && !errors.Is(err, ErrCorrupt) && !errors.Is(err, ErrUnsupported) {
				t.Fatalf("Open failed with an unclassified error: %v", err)
			}
			return
		}
		defer a.Close()
		// ok reports whether err is nil and fails the fuzz run on an error the
		// reader does not document.
		ok := func(what string, err error) bool {
			t.Helper()
			if err == nil {
				return true
			}
			if !readErrorIsClassified(err) {
				t.Fatalf("%s failed with an unclassified error: %v", what, err)
			}
			return false
		}

		for i := uint32(0); i < a.EntryCount() && i < 64; i++ {
			e, err := a.EntryAt(i)
			if !ok("EntryAt", err) {
				continue
			}
			if e.Index != i {
				t.Fatalf("EntryAt(%d) returned index %d", i, e.Index)
			}
			if found, err := a.EntryByPath(e.Namespace, e.Path); ok("EntryByPath", err) &&
				(found.Namespace != e.Namespace || found.Path != e.Path) {
				t.Fatalf("EntryByPath(%c, %q) returned %c/%q", e.Namespace, e.Path, found.Namespace, found.Path)
			}
			res, err := a.Resolve(e)
			if !ok("Resolve", err) {
				continue
			}
			if res.IsRedirect {
				t.Fatalf("Resolve(%c/%s) returned the redirect %c/%s", e.Namespace, e.Path, res.Namespace, res.Path)
			}
			r, err := a.Open(res)
			if !ok("Open", err) {
				continue
			}
			if _, err := io.Copy(io.Discard, io.LimitReader(r, fuzzClusterLimit)); err != nil {
				t.Fatalf("reading the validated blob of %c/%s failed: %v", res.Namespace, res.Path, err)
			}
		}

		for i := 0; i < a.ArticleCount() && i < 16; i++ {
			if e, err := a.ArticleAt(i); ok("ArticleAt", err) && e.Namespace != a.ContentNamespace() {
				t.Fatalf("ArticleAt(%d) = %c/%s, outside the content namespace %c", i, e.Namespace, e.Path, a.ContentNamespace())
			}
		}
		if _, err := a.ArticleAt(a.ArticleCount()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("ArticleAt(ArticleCount()) error = %v, want ErrNotFound", err)
		}

		const limit = 5
		for _, prefix := range []string{"B", "", "\xff"} {
			got, err := a.TitlePrefix(prefix, limit)
			if !ok("TitlePrefix", err) {
				continue
			}
			if len(got) > limit {
				t.Fatalf("TitlePrefix(%q, %d) returned %d entries", prefix, limit, len(got))
			}
			for _, e := range got {
				if !strings.HasPrefix(e.Title, prefix) {
					t.Fatalf("TitlePrefix(%q) returned %q", prefix, e.Title)
				}
			}
		}

		if e, err := a.MainEntry(); ok("MainEntry", err) && e.IsRedirect {
			t.Fatalf("MainEntry returned the redirect %c/%s", e.Namespace, e.Path)
		}
		_, err = a.Metadata("Title")
		ok("Metadata", err)
		ok("VerifyChecksum", a.VerifyChecksum(context.Background()))
	})
}
