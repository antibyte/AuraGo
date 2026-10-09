package zim

import (
	"encoding/binary"
	"errors"
	"testing"
)

func contentDirent(mime uint16, ns byte, cluster, blob uint32, path, title string) []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint16(b[0:], mime)
	b[3] = ns
	binary.LittleEndian.PutUint32(b[8:], cluster)
	binary.LittleEndian.PutUint32(b[12:], blob)
	b = append(b, path...)
	b = append(b, 0)
	b = append(b, title...)
	return append(b, 0)
}

func redirectDirent(ns byte, target uint32, path, title string) []byte {
	b := make([]byte, 12)
	binary.LittleEndian.PutUint16(b[0:], mimeRedirect)
	b[3] = ns
	binary.LittleEndian.PutUint32(b[8:], target)
	b = append(b, path...)
	b = append(b, 0)
	b = append(b, title...)
	return append(b, 0)
}

func TestParseDirentContent(t *testing.T) {
	e, err := parseDirent(contentDirent(1, 'C', 7, 3, "Berlin", ""), []string{"text/css", "text/html"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Namespace != 'C' || e.Path != "Berlin" || e.Title != "Berlin" || e.MimeType != "text/html" {
		t.Fatalf("entry = %+v", e)
	}
	if e.IsRedirect || e.kind != kindContent || e.cluster != 7 || e.blob != 3 {
		t.Fatalf("entry location = %+v", e)
	}
}

func TestParseDirentRedirect(t *testing.T) {
	e, err := parseDirent(redirectDirent('W', 42, "mainPage", "Main"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !e.IsRedirect || e.RedirectTo != 42 || e.MimeType != "" || e.Title != "Main" || e.kind != kindRedirect {
		t.Fatalf("entry = %+v", e)
	}
}

func TestParseDirentDeprecatedEntry(t *testing.T) {
	b := []byte{0xFE, 0xFF, 0, 'A', 0, 0, 0, 0, 'o', 'l', 'd', 0, 0}
	e, err := parseDirent(b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if e.kind != kindDeprecated || e.Path != "old" || e.IsRedirect {
		t.Fatalf("entry = %+v", e)
	}
}

func TestParseDirentShortAndInvalid(t *testing.T) {
	full := contentDirent(0, 'C', 0, 0, "Path", "Title")
	for _, n := range []int{0, 7, 15, 18, len(full) - 1} {
		if _, err := parseDirent(full[:n], []string{"text/html"}); !errors.Is(err, errShortDirent) {
			t.Fatalf("parseDirent(%d bytes) error = %v, want errShortDirent", n, err)
		}
	}
	withParams := append(append([]byte(nil), full...), 'p')
	withParams[2] = 2 // two parameter bytes declared, one present
	if _, err := parseDirent(withParams, []string{"text/html"}); !errors.Is(err, errShortDirent) {
		t.Fatalf("parameter overrun error = %v, want errShortDirent", err)
	}
	_, err := parseDirent(contentDirent(5, 'C', 0, 0, "Path", ""), []string{"text/html"})
	wantErr(t, err, ErrCorrupt)
}

func TestCompareKeyOrdersNamespaceThenPath(t *testing.T) {
	if compareKey('A', "z", 'C', "a") >= 0 || compareKey('C', "a", 'C', "b") >= 0 || compareKey('C', "b", 'C', "b") != 0 {
		t.Fatal("compareKey ordering is wrong")
	}
}
