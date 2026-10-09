package zim

import (
	"bytes"
	"errors"
	"testing"

	"aurago/internal/zim/zimtest"
)

// sampleBuilder returns a new-scheme archive covering every cluster kind:
// zstd, xz, uncompressed, extended zstd and extended uncompressed.
// Cluster order: 0 zstd, 1 xz, 2 uncompressed, 3 extended zstd,
// 4 extended uncompressed, 5 title listing (added by Build).
func sampleBuilder() *zimtest.Builder {
	b := zimtest.New()
	z := b.AddCluster(zimtest.CompressionZstd, false)
	x := b.AddCluster(zimtest.CompressionXZ, false)
	u := b.AddCluster(zimtest.CompressionNone, false)
	ez := b.AddCluster(zimtest.CompressionZstd, true)
	eu := b.AddCluster(zimtest.CompressionNone, true)
	b.AddArticle(z, 'C', "Berlin", "Berlin", "<html><body><p>Berlin ist die Hauptstadt.</p></body></html>")
	b.AddArticle(z, 'C', "Bern", "Bern", "<html><body><p>Bern.</p></body></html>")
	b.AddArticle(z, 'C', "index", "Hauptseite", "<html><body><p>Willkommen</p></body></html>")
	b.AddArticle(x, 'C', "Hamburg", "Hamburg", "<html><body><p>Hamburg liegt an der Elbe.</p></body></html>")
	b.AddArticle(ez, 'C', "Köln", "Köln", "<html><body><p>Köln am Rhein.</p></body></html>")
	b.Add(zimtest.Entry{Namespace: 'C', Path: "Berlin_(Stadt)", Title: "Berlin (Stadt)", Redirect: "C/Berlin", Front: true})
	b.Add(zimtest.Entry{Namespace: 'C', Path: "map.webp", MimeType: "image/webp", Data: []byte("RIFF-webp-bytes"), Cluster: u})
	b.Add(zimtest.Entry{Namespace: 'C', Path: "big.bin", MimeType: "application/octet-stream", Data: bytes.Repeat([]byte{0xAB}, 4096), Cluster: eu})
	b.Add(zimtest.Entry{Namespace: 'X', Path: "fulltext/xapian", MimeType: "application/octet-stream+xapian", Data: []byte("fulltext-index-bytes"), Cluster: u})
	b.Add(zimtest.Entry{Namespace: 'X', Path: "title/xapian", MimeType: "application/octet-stream+xapian", Data: []byte("title-index-bytes"), Cluster: u})
	b.AddMetadata(z, "Language", "deu")
	b.AddMetadata(z, "Title", "Wikipedia (Test)")
	b.AddMetadata(z, "Date", "2026-10-01")
	b.AddRedirect('M', "Alias", "", "M/Title")
	b.AddRedirect('W', "mainPage", "", "C/index")
	return b
}

func wantErr(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error = %v, want %v", err, target)
	}
}
