package server

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

type oraTestEntry struct {
	name   string
	data   []byte
	method uint16
	extra  []byte
}

func oraTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 40), G: uint8(y * 40), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func oraTestArchive(t *testing.T, entries []oraTestEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: entry.method, Extra: entry.extra}
		w, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const oraTestStack = `<?xml version="1.0" encoding="UTF-8"?>
<image w="4" h="3" version="0.0.5" xmlns:aurago="urn:aurago:openraster">
 <stack>
  <layer name="Top &amp; &quot;more&quot;" src="data/layer-1.png" x="0" y="0" opacity="0.5" visibility="visible" composite-op="svg:multiply" aurago:locked-pixels="true"/>
  <layer name="Bottom" src="data/layer-0.png" x="1" y="1" opacity="1" visibility="hidden" composite-op="svg:src-over" aurago:mask="data/layer-0-mask.png" aurago:mask-enabled="true"/>
 </stack>
</image>`

// oraTestEntries is a complete, valid project; tests copy it and break one thing.
func oraTestEntries(t *testing.T) []oraTestEntry {
	t.Helper()
	full := oraTestPNG(t, 4, 3)
	small := oraTestPNG(t, 2, 2)
	return []oraTestEntry{
		{name: "mimetype", data: []byte("image/openraster"), method: zip.Store},
		{name: "stack.xml", data: []byte(oraTestStack), method: zip.Deflate},
		{name: "data/layer-0.png", data: small, method: zip.Store},
		{name: "data/layer-0-mask.png", data: full, method: zip.Store},
		{name: "data/layer-1.png", data: full, method: zip.Store},
		{name: "mergedimage.png", data: full, method: zip.Store},
		{name: "Thumbnails/thumbnail.png", data: small, method: zip.Deflate},
	}
}

func oraTestWithout(entries []oraTestEntry, name string) []oraTestEntry {
	var out []oraTestEntry
	for _, entry := range entries {
		if entry.name != name {
			out = append(out, entry)
		}
	}
	return out
}

func oraTestReplace(entries []oraTestEntry, name string, data []byte) []oraTestEntry {
	out := append([]oraTestEntry(nil), entries...)
	for i := range out {
		if out[i].name == name {
			out[i].data = data
		}
	}
	return out
}

const oraTestLimit = 32 << 20

func TestPixelORAAcceptsValidArchive(t *testing.T) {
	data := oraTestArchive(t, oraTestEntries(t))
	if err := pixelValidateORA(context.Background(), data, oraTestLimit); err != nil {
		t.Fatalf("valid archive rejected: %v", err)
	}
	format, err := pixelValidateSavePayload(context.Background(), data, oraTestLimit, "Pictures/project.ora", "ora")
	if err != nil || format != "ora" {
		t.Fatalf("payload format = %q, %v", format, err)
	}
	format, err = pixelValidateSavePayload(context.Background(), data, oraTestLimit, "Pictures/PROJECT.ORA", "")
	if err != nil || format != "ora" {
		t.Fatalf("upper-case extension: %q, %v", format, err)
	}

	// Foreign writers add explicit directory entries; the file set stays within the allow-list.
	withDirs := append([]oraTestEntry{{name: "mimetype", data: []byte("image/openraster"), method: zip.Store}, {name: "data/", method: zip.Store}}, oraTestEntries(t)[1:]...)
	if err := pixelValidateORA(context.Background(), oraTestArchive(t, withDirs), oraTestLimit); err != nil {
		t.Fatalf("archive with a data/ directory entry rejected: %v", err)
	}
}

func TestPixelORAKeepsPlainImageRules(t *testing.T) {
	if format, err := pixelValidateSavePayload(context.Background(), oraTestPNG(t, 3, 3), oraTestLimit, "Pictures/a.png", "png"); err != nil || format != "png" {
		t.Fatalf("png: %q, %v", format, err)
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 3, 3)), nil); err != nil {
		t.Fatal(err)
	}
	if format, err := pixelValidateSavePayload(context.Background(), jpg.Bytes(), oraTestLimit, "Pictures/a.jpg", "jpeg"); err != nil || format != "jpeg" {
		t.Fatalf("jpeg: %q, %v", format, err)
	}
	if _, err := pixelValidateSavePayload(context.Background(), oraTestPNG(t, 3, 3), oraTestLimit, "Pictures/a.ora", "ora"); err == nil {
		t.Fatal("a PNG was accepted as an OpenRaster project")
	}
}

func TestPixelORARejectsBrokenArchives(t *testing.T) {
	valid := oraTestEntries(t)
	validBytes := oraTestArchive(t, valid)
	full := oraTestPNG(t, 4, 3)

	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	stackWith := func(layers string) []byte {
		return []byte(`<image w="4" h="3" xmlns:aurago="urn:aurago:openraster"><stack>` + layers + `</stack></image>`)
	}
	many := []oraTestEntry{valid[0], valid[1]}
	for i := 0; i < 127; i++ {
		many = append(many, oraTestEntry{name: fmt.Sprintf("data/n%d.png", i), data: full, method: zip.Store})
	}
	withComment := append([]byte(nil), validBytes...)
	binary.LittleEndian.PutUint16(withComment[len(withComment)-2:], 1)
	withComment = append(withComment, 'x')
	truncatedPNG := full[:len(full)-12]

	cases := map[string][]byte{
		"not a zip":            []byte("PK this is not an archive at all"),
		"empty":                {},
		"archive comment":      withComment,
		"traversal":            oraTestArchive(t, append(append([]oraTestEntry(nil), valid...), oraTestEntry{name: "../evil.png", data: full})),
		"nested traversal":     oraTestArchive(t, append(append([]oraTestEntry(nil), valid...), oraTestEntry{name: "data/../../evil.png", data: full})),
		"absolute path":        oraTestArchive(t, append(append([]oraTestEntry(nil), valid...), oraTestEntry{name: "/etc/evil.png", data: full})),
		"backslash":            oraTestArchive(t, append(append([]oraTestEntry(nil), valid...), oraTestEntry{name: `data\evil.png`, data: full})),
		"drive letter":         oraTestArchive(t, append(append([]oraTestEntry(nil), valid...), oraTestEntry{name: "C:evil.png", data: full})),
		"unexpected file":      oraTestArchive(t, append(append([]oraTestEntry(nil), valid...), oraTestEntry{name: "payload.exe", data: []byte("MZ")})),
		"nested data folder":   oraTestArchive(t, append(append([]oraTestEntry(nil), valid...), oraTestEntry{name: "data/sub/x.png", data: full})),
		"duplicate entry":      oraTestArchive(t, append(append([]oraTestEntry(nil), valid...), oraTestEntry{name: "DATA/layer-1.png", data: full})),
		"missing mimetype":     oraTestArchive(t, oraTestWithout(valid, "mimetype")),
		"mimetype not first":   oraTestArchive(t, append(append([]oraTestEntry(nil), valid[1:]...), valid[0])),
		"wrong mimetype":       oraTestArchive(t, oraTestReplace(valid, "mimetype", []byte("application/zip"))),
		"mimetype newline":     oraTestArchive(t, oraTestReplace(valid, "mimetype", []byte("image/openraster\n"))),
		"mimetype compressed":  oraTestArchive(t, append([]oraTestEntry{{name: "mimetype", data: []byte("image/openraster"), method: zip.Deflate}}, valid[1:]...)),
		"mimetype extra field": oraTestArchive(t, append([]oraTestEntry{{name: "mimetype", data: []byte("image/openraster"), method: zip.Store, extra: []byte{0xFE, 0xCA, 0, 0}}}, valid[1:]...)),
		"no stack":             oraTestArchive(t, oraTestWithout(valid, "stack.xml")),
		"stack not xml":        oraTestArchive(t, oraTestReplace(valid, "stack.xml", []byte("this is { not xml"))),
		"stack unclosed":       oraTestArchive(t, oraTestReplace(valid, "stack.xml", []byte(`<image w="4" h="3"><stack>`))),
		"stack mismatched":     oraTestArchive(t, oraTestReplace(valid, "stack.xml", []byte(`<image w="4" h="3"><stack></image></stack>`))),
		"stack wrong root":     oraTestArchive(t, oraTestReplace(valid, "stack.xml", []byte(`<picture w="4" h="3"/>`))),
		"stack two roots":      oraTestArchive(t, oraTestReplace(valid, "stack.xml", []byte(`<image w="4" h="3"/><image w="4" h="3"/>`))),
		"stack doctype":        oraTestArchive(t, oraTestReplace(valid, "stack.xml", []byte(`<!DOCTYPE image [<!ENTITY a "b">]><image w="4" h="3"><stack/></image>`))),
		"stack zero size":      oraTestArchive(t, oraTestReplace(valid, "stack.xml", []byte(`<image w="0" h="3"><stack/></image>`))),
		"stack huge size":      oraTestArchive(t, oraTestReplace(valid, "stack.xml", []byte(`<image w="99999" h="99999"><stack/></image>`))),
		"stack text outside":   oraTestArchive(t, oraTestReplace(valid, "stack.xml", []byte(`junk<image w="4" h="3"><stack/></image>`))),
		"stack oversize":       oraTestArchive(t, oraTestReplace(valid, "stack.xml", append([]byte(`<image w="4" h="3"><stack/></image><!--`), append(bytes.Repeat([]byte("a"), pixelORAMaxXMLBytes), []byte("-->")...)...))),
		"stack deep nesting":   oraTestArchive(t, oraTestReplace(valid, "stack.xml", []byte(`<image w="4" h="3">`+strings.Repeat("<stack>", 40)+strings.Repeat("</stack>", 40)+`</image>`))),
		"layer without src":    oraTestArchive(t, oraTestReplace(valid, "stack.xml", stackWith(`<layer name="x"/>`))),
		"layer missing file":   oraTestArchive(t, oraTestReplace(valid, "stack.xml", stackWith(`<layer src="data/missing.png"/>`))),
		"mask missing file":    oraTestArchive(t, oraTestReplace(valid, "stack.xml", stackWith(`<layer src="data/layer-1.png" aurago:mask="data/none.png"/>`))),
		"layer src escapes":    oraTestArchive(t, oraTestReplace(valid, "stack.xml", stackWith(`<layer src="../layer-1.png"/>`))),
		"layer src not png":    oraTestArchive(t, oraTestReplace(valid, "stack.xml", stackWith(`<layer src="stack.xml"/>`))),
		"layer bad opacity":    oraTestArchive(t, oraTestReplace(valid, "stack.xml", stackWith(`<layer src="data/layer-1.png" opacity="7"/>`))),
		"layer bad offset":     oraTestArchive(t, oraTestReplace(valid, "stack.xml", stackWith(`<layer src="data/layer-1.png" x="abc"/>`))),
		"garbage png":          oraTestArchive(t, oraTestReplace(valid, "data/layer-1.png", []byte("definitely not a png"))),
		"truncated png":        oraTestArchive(t, oraTestReplace(valid, "data/layer-1.png", truncatedPNG)),
		"jpeg as layer":        oraTestArchive(t, oraTestReplace(valid, "data/layer-1.png", jpg.Bytes())),
		"layer larger":         oraTestArchive(t, oraTestReplace(valid, "data/layer-1.png", oraTestPNG(t, 9, 9))),
		"too many entries":     oraTestArchive(t, many),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			err := pixelValidateORA(context.Background(), data, oraTestLimit)
			if err == nil {
				t.Fatal("archive was accepted")
			}
			t.Logf("rejected: %v", err)
		})
	}
	if err := pixelValidateORA(context.Background(), validBytes, int64(len(validBytes))-1); !errors.Is(err, errPixelORATooLarge) {
		t.Fatalf("oversize archive: %v", err)
	}
	if err := pixelValidateORA(context.Background(), validBytes, int64(len(validBytes))); err != nil {
		t.Fatalf("archive exactly at the limit rejected: %v", err)
	}
}

// oraTestRaw writes entries whose header sizes are chosen by the test instead of measured, like a hostile archive.
func oraTestRaw(t *testing.T, entries []oraTestEntry, lies map[string]uint64) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, entry := range entries {
		payload := entry.data
		header := &zip.FileHeader{Name: entry.name, Method: entry.method}
		header.CRC32 = crc32.ChecksumIEEE(payload)
		header.UncompressedSize64 = uint64(len(payload))
		if entry.method == zip.Deflate {
			var comp bytes.Buffer
			fw, err := flate.NewWriter(&comp, flate.BestCompression)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = fw.Write(payload)
			_ = fw.Close()
			payload = comp.Bytes()
		}
		header.CompressedSize64 = uint64(len(payload))
		if lie, ok := lies[entry.name]; ok {
			header.UncompressedSize64 = lie
		}
		w, err := zw.CreateRaw(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPixelORARejectsZipBombs(t *testing.T) {
	valid := oraTestEntries(t)

	t.Run("sanity: raw writer round trip", func(t *testing.T) {
		if err := pixelValidateORA(context.Background(), oraTestRaw(t, valid, nil), oraTestLimit); err != nil {
			t.Fatalf("raw archive rejected: %v", err)
		}
	})

	t.Run("png declared small, actual large", func(t *testing.T) {
		// 20 MiB of zeros deflate to a few KiB; the header claims 100 bytes.
		bomb := []oraTestEntry{valid[0], valid[1], {name: "data/layer-1.png", data: make([]byte, 20<<20), method: zip.Deflate}}
		data := oraTestRaw(t, bomb, map[string]uint64{"data/layer-1.png": 100})
		if len(data) > 1<<20 {
			t.Fatalf("test archive is not small: %d bytes", len(data))
		}
		if err := pixelValidateORA(context.Background(), data, oraTestLimit); err == nil || !strings.Contains(err.Error(), "entry") {
			t.Fatalf("bomb not stopped by the entry size guard: %v", err)
		}
	})

	t.Run("stack declared small, actual large", func(t *testing.T) {
		bomb := []oraTestEntry{valid[0], {name: "stack.xml", data: bytes.Repeat([]byte(" "), 3<<20), method: zip.Deflate}}
		data := oraTestRaw(t, bomb, map[string]uint64{"stack.xml": 64})
		if err := pixelValidateORA(context.Background(), data, oraTestLimit); err == nil || !strings.Contains(err.Error(), "entry") {
			t.Fatalf("bomb not stopped by the entry size guard: %v", err)
		}
	})

	t.Run("stack declared above its limit", func(t *testing.T) {
		bomb := []oraTestEntry{valid[0], {name: "stack.xml", data: bytes.Repeat([]byte(" "), pixelORAMaxXMLBytes+1), method: zip.Deflate}}
		if err := pixelValidateORA(context.Background(), oraTestRaw(t, bomb, nil), oraTestLimit); err == nil {
			t.Fatal("oversized stack.xml accepted")
		}
	})

	t.Run("declared total above the cap", func(t *testing.T) {
		entries := []oraTestEntry{valid[0], valid[1]}
		lies := map[string]uint64{}
		for i := 0; i < 10; i++ {
			name := fmt.Sprintf("data/big-%d.png", i)
			entries = append(entries, oraTestEntry{name: name, data: []byte("x"), method: zip.Store})
			lies[name] = 60 << 20
		}
		err := pixelValidateORA(context.Background(), oraTestRaw(t, entries, lies), 64<<20)
		if !errors.Is(err, errPixelORATooLarge) {
			t.Fatalf("total size cap not enforced: %v", err)
		}
	})

	t.Run("single png above the byte limit", func(t *testing.T) {
		entries := []oraTestEntry{valid[0], valid[1], {name: "data/layer-1.png", data: []byte("x"), method: zip.Store}}
		err := pixelValidateORA(context.Background(), oraTestRaw(t, entries, map[string]uint64{"data/layer-1.png": oraTestLimit + 1}), oraTestLimit)
		if !errors.Is(err, errPixelORATooLarge) {
			t.Fatalf("per-entry cap not enforced: %v", err)
		}
	})

	t.Run("corrupt crc", func(t *testing.T) {
		data := oraTestArchive(t, valid)
		// Flip one byte inside the stored stack... find a stored PNG payload and damage it.
		index := bytes.Index(data, []byte("IDAT"))
		if index < 0 {
			t.Fatal("no IDAT in test archive")
		}
		data[index+6] ^= 0xFF
		if err := pixelValidateORA(context.Background(), data, oraTestLimit); err == nil {
			t.Fatal("corrupt archive accepted")
		}
	})
}

func oraTestPayload(t *testing.T, path, format string, data []byte) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]string{"path": path, "format": format, "data": "data:application/zip;base64," + base64.StdEncoding.EncodeToString(data)})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestPixelSaveStoresOpenRasterProjects(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	archive := oraTestArchive(t, oraTestEntries(t))
	call := func(path, format string, data []byte, header, value string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/pixel/save", bytes.NewReader(oraTestPayload(t, path, format, data)))
		if header != "" {
			r.Header.Set(header, value)
		}
		w := httptest.NewRecorder()
		handlePixelSave(s)(w, r)
		return w
	}
	stored := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, "Pictures", "project.ora")

	if w := call("Pictures/project.ora", "ora", archive, "", ""); w.Code != 428 {
		t.Fatalf("missing precondition: %d", w.Code)
	}
	w := call("Pictures/project.ora", "ora", archive, "If-None-Match", "*")
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	version := desktop.NoteVersion(archive)
	var saved struct {
		Path    string `json:"path"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil || w.Header().Get("ETag") != version || saved.Version != version || saved.Path != "Pictures/project.ora" {
		t.Fatalf("version not returned: %q %s", w.Header().Get("ETag"), w.Body.String())
	}
	if got, err := os.ReadFile(stored); err != nil || !bytes.Equal(got, archive) {
		t.Fatalf("stored bytes differ: %v", err)
	}

	if w := call("Pictures/project.ora", "ora", archive, "If-None-Match", "*"); w.Code != 412 {
		t.Fatalf("collision: %d", w.Code)
	}
	if w := call("Pictures/project.ora", "ora", archive, "If-Match", `"`+strings.Repeat("0", 64)+`"`); w.Code != 412 {
		t.Fatalf("stale version: %d", w.Code)
	}
	updatedEntries := oraTestReplace(oraTestEntries(t), "data/layer-1.png", oraTestPNG(t, 4, 3))
	updatedEntries = oraTestReplace(updatedEntries, "stack.xml", []byte(strings.Replace(oraTestStack, "Bottom", "Renamed", 1)))
	updated := oraTestArchive(t, updatedEntries)
	if w := call("Pictures/project.ora", "ora", updated, "If-Match", version); w.Code != 200 {
		t.Fatalf("update with the observed version: %d %s", w.Code, w.Body.String())
	}
	if got, _ := os.ReadFile(stored); !bytes.Equal(got, updated) {
		t.Fatal("update was not stored")
	}

	// Rejected payloads leave the stored project untouched.
	currentVersion := desktop.NoteVersion(updated)
	traversal := oraTestArchive(t, append(oraTestEntries(t), oraTestEntry{name: "../escape.png", data: oraTestPNG(t, 2, 2)}))
	for name, rejected := range map[string][]byte{"traversal": traversal, "png on ora path": oraTestPNG(t, 2, 2), "garbage": []byte("nope")} {
		if w := call("Pictures/project.ora", "ora", rejected, "If-Match", currentVersion); w.Code != 400 {
			t.Fatalf("%s accepted: %d %s", name, w.Code, w.Body.String())
		}
	}
	if got, _ := os.ReadFile(stored); !bytes.Equal(got, updated) {
		t.Fatal("rejected save changed the stored project")
	}

	// Format and extension must agree in both directions.
	if w := call("Pictures/project.png", "ora", archive, "If-None-Match", "*"); w.Code != 400 {
		t.Fatalf("ora data on a .png path: %d", w.Code)
	}
	if w := call("Pictures/other.ora", "png", archive, "If-None-Match", "*"); w.Code != 400 {
		t.Fatalf("png format on a .ora path: %d", w.Code)
	}
	if w := call("Pictures/other.ora", "", oraTestPNG(t, 2, 2), "If-None-Match", "*"); w.Code != 400 {
		t.Fatalf("PNG bytes on a .ora path: %d", w.Code)
	}
	if _, err := os.Stat(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, "Pictures", "other.ora")); !os.IsNotExist(err) {
		t.Fatal("rejected project was created")
	}
	if w := call("../escape.ora", "ora", archive, "If-None-Match", "*"); w.Code < 400 {
		t.Fatal("path traversal accepted")
	}

	s.Cfg.VirtualDesktop.ReadOnly = true
	if w := call("Pictures/readonly.ora", "ora", archive, "If-None-Match", "*"); w.Code != 403 {
		t.Fatalf("readonly: %d", w.Code)
	}
}

func TestPixelSaveRejectsOversizedOpenRasterBody(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	s.Cfg.VirtualDesktop.MaxFileSizeMB = 1
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	original := oraTestArchive(t, oraTestEntries(t))
	if err := svc.WriteFileBytes(context.Background(), "Pictures/limit.ora", original, desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/pixel/save", bytes.NewReader(oraTestPayload(t, "Pictures/limit.ora", "ora", make([]byte, 2<<20))))
	r.Header.Set("If-Match", desktop.NoteVersion(original))
	w := httptest.NewRecorder()
	handlePixelSave(s)(w, r)
	if w.Code != 413 {
		t.Fatalf("oversize: %d %s", w.Code, w.Body.String())
	}
	if got, _, err := svc.ReadFileBytes(context.Background(), "Pictures/limit.ora"); err != nil || !bytes.Equal(got, original) {
		t.Fatal("original changed")
	}
}
