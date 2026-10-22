package server

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// oraTestChunk builds one PNG chunk with a correct CRC.
func oraTestChunk(kind string, body []byte) []byte {
	out := binary.BigEndian.AppendUint32(make([]byte, 0, len(body)+12), uint32(len(body)))
	out = append(out, kind...)
	out = append(out, body...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(out[4:]))
}

// oraTestPNGWithIDAT builds a 1-bit grayscale PNG whose header claims w x h and whose single IDAT chunk holds exactly
// the given bytes: a valid stream when idat is a zlib stream of the scanlines, a header-only fake otherwise.
func oraTestPNGWithIDAT(w, h int, idat []byte) []byte {
	header := binary.BigEndian.AppendUint32(nil, uint32(w))
	header = binary.BigEndian.AppendUint32(header, uint32(h))
	header = append(header, 1, 0, 0, 0, 0) // 1 bit per pixel, grayscale, deflate, filter method 0, no interlace
	out := []byte(pixelPNGSignature)
	out = append(out, oraTestChunk("IHDR", header)...)
	out = append(out, oraTestChunk("IDAT", idat)...)
	return append(out, oraTestChunk("IEND", nil)...)
}

// oraTestConstantPNG is a genuinely valid constant-colour PNG of any size. Rows of zeros deflate to a few KB, so an
// 8192x4096 layer (33.5 Mpx, 134 MB as an RGBA bitmap) is a ~4 KB archive entry: the shape of the attack.
func oraTestConstantPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var packed bytes.Buffer
	zw, err := zlib.NewWriterLevel(&packed, zlib.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	row := make([]byte, 1+(w+7)/8)
	for y := 0; y < h; y++ {
		if _, err := zw.Write(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return oraTestPNGWithIDAT(w, h, packed.Bytes())
}

// oraTestLayersArchive is a project of `layers` identical full-size layers (and optionally mergedimage.png).
func oraTestLayersArchive(t *testing.T, w, h, layers int, layerPNG []byte, merged bool) []byte {
	t.Helper()
	var stack strings.Builder
	fmt.Fprintf(&stack, `<image w="%d" h="%d"><stack>`, w, h)
	entries := []oraTestEntry{{name: "mimetype", data: []byte("image/openraster"), method: zip.Store}, {name: "stack.xml", method: zip.Deflate}}
	for i := 0; i < layers; i++ {
		fmt.Fprintf(&stack, `<layer name="L%d" src="data/l%d.png"/>`, i, i)
		entries = append(entries, oraTestEntry{name: fmt.Sprintf("data/l%d.png", i), data: layerPNG, method: zip.Store})
	}
	stack.WriteString(`</stack></image>`)
	entries[1].data = []byte(stack.String())
	if merged {
		entries = append(entries, oraTestEntry{name: "mergedimage.png", data: layerPNG, method: zip.Store})
	}
	return oraTestArchive(t, entries)
}

func oraTestAllocated(fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestPixelORATestPNGBuilderMakesRealPNGs(t *testing.T) {
	// The fixtures below rely on the hand-built PNGs being valid ones; prove it on a size the decoder can afford.
	data := oraTestConstantPNG(t, 37, 11)
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Dx() != 37 || img.Bounds().Dy() != 11 {
		t.Fatalf("builder output does not decode: %v", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(oraTestConstantPNG(t, 8192, 4096)))
	if err != nil || format != "png" || cfg.Width != 8192 || cfg.Height != 4096 {
		t.Fatalf("large fixture header = %+v %q %v", cfg, format, err)
	}
}

// The finding: ~4 MB of constant-colour 8192x4096 layers used to pass every cap and then get fully decoded, about
// 134 MB of heap per layer. ORA layers are now checked by header and chunk structure only.
func TestPixelORAValidatesLargeLayersWithoutDecodingPixels(t *testing.T) {
	layer := oraTestConstantPNG(t, 8192, 4096)
	archive := oraTestLayersArchive(t, 8192, 4096, 24, layer, true)
	if len(archive) > 1<<20 {
		t.Fatalf("fixture is not small: %d bytes", len(archive))
	}
	var err error
	started := time.Now()
	allocated := oraTestAllocated(func() { err = pixelValidateORA(context.Background(), archive, oraTestLimit) })
	if err != nil {
		t.Fatalf("project of large constant-colour layers rejected: %v", err)
	}
	// 25 full decodes would allocate ~3.3 GB; the header check only reads the few KB of each entry.
	if allocated > 32<<20 {
		t.Fatalf("validation allocated %d MiB, the layers are being decoded", allocated>>20)
	}
	t.Logf("24 layers of 8192x4096: %d MiB allocated, %v", allocated>>20, time.Since(started))
}

// Decision: the server stores the archive and never uses the pixels, so a layer whose header and chunk stream are
// sound but whose IDAT data does not inflate to a bitmap is accepted (the zip CRC already proves the bytes are the
// ones the editor sent, and the client decodes its own file when it opens it). The plain PNG/JPEG save path is
// unchanged and still decodes everything.
func TestPixelORADoesNotInflateLayerPixelsByDesign(t *testing.T) {
	notInflatable := oraTestPNGWithIDAT(2, 2, bytes.Repeat([]byte{0xAB}, 64))
	if _, err := pixelValidateImage(notInflatable, oraTestLimit); err == nil {
		t.Fatal("fixture decodes as a plain image; the test would prove nothing")
	}
	data := oraTestArchive(t, oraTestReplace(oraTestEntries(t), "data/layer-0.png", notInflatable))
	if err := pixelValidateORA(context.Background(), data, oraTestLimit); err != nil {
		t.Fatalf("header-valid layer rejected: %v", err)
	}
	if _, err := pixelValidateSavePayload(context.Background(), notInflatable, oraTestLimit, "Pictures/a.png", "png"); err == nil {
		t.Fatal("the plain PNG save path must keep decoding fully")
	}
}

func TestPixelORARejectsLayersOverTheLimits(t *testing.T) {
	big := oraTestConstantPNG(t, 8192, 4096)
	wide := oraTestConstantPNG(t, 8200, 2048) // 16.8 Mpx: inside the pixel cap, but wider than the 8192 px image
	cases := map[string]struct {
		archive []byte
		want    string
	}{
		"layer wider than the image":  {oraTestLayersArchive(t, 8192, 4096, 1, wide, false), "larger than the image"},
		"layer over the pixel cap":    {oraTestLayersArchive(t, 8192, 4096, 1, oraTestPNGWithIDAT(40000, 40000, []byte("x")), false), "dimensions exceed"},
		"layer without width":         {oraTestLayersArchive(t, 8192, 4096, 1, oraTestPNGWithIDAT(0, 5, []byte("x")), false), "not a valid PNG"},
		"total pixels over the cap":   {oraTestLayersArchive(t, 8192, 4096, 33, big, false), "too large"},
		"total with the merged image": {oraTestLayersArchive(t, 8192, 4096, 32, big, true), "too large"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var err error
			allocated := oraTestAllocated(func() { err = pixelValidateORA(context.Background(), tc.archive, oraTestLimit) })
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want one containing %q", err, tc.want)
			}
			if allocated > 32<<20 {
				t.Fatalf("rejection allocated %d MiB", allocated>>20)
			}
		})
	}
	// Exactly at the cap is fine: 32 layers of 8192x4096 are 1<<30 pixels.
	if err := pixelValidateORA(context.Background(), oraTestLayersArchive(t, 8192, 4096, 32, big, false), oraTestLimit); err != nil {
		t.Fatalf("project exactly at the total pixel cap rejected: %v", err)
	}
}

func TestPixelORAChecksThePNGChunkStream(t *testing.T) {
	valid := oraTestConstantPNG(t, 16, 16)
	if _, err := pixelORACheckPNG(valid); err != nil {
		t.Fatalf("valid PNG rejected: %v", err)
	}
	if _, err := pixelORACheckPNG(append(append([]byte(nil), valid...), "trailing bytes"...)); err != nil {
		t.Fatalf("bytes after IEND are ignored like the standard decoder does, got: %v", err)
	}
	flipped := func(offset int) []byte {
		out := append([]byte(nil), valid...)
		out[offset] ^= 0xFF
		return out
	}
	noIDAT := append(append([]byte(pixelPNGSignature), valid[8:8+25]...), oraTestChunk("IEND", nil)...)
	hugeLength := append([]byte(nil), valid...)
	binary.BigEndian.PutUint32(hugeLength[8+25:], 0xFFFFFFF0) // the IDAT chunk claims more bytes than exist
	cases := map[string][]byte{
		"empty":                 {},
		"signature only":        []byte(pixelPNGSignature),
		"truncated before IEND": valid[:len(valid)-12],
		"truncated in a chunk":  valid[:len(valid)-20],
		"bad IHDR checksum":     flipped(8 + 8 + 2),
		"bad IDAT checksum":     flipped(8 + 25 + 8 + 1),
		"no IDAT":               noIDAT,
		"oversized chunk":       hugeLength,
		"not a PNG":             bytes.Repeat([]byte{0x42}, 100),
	}
	for name, data := range cases {
		if _, err := pixelORACheckPNG(data); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Concurrent project saves queue instead of inflating side by side.
func TestPixelORAValidationSlotsAreBounded(t *testing.T) {
	archive := oraTestArchive(t, oraTestEntries(t))
	var held []func()
	t.Cleanup(func() {
		for _, release := range held {
			release()
		}
	})
	for i := 0; i < pixelORAMaxConcurrent; i++ {
		release, err := pixelORAAcquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, release)
	}

	// Cheap rejections happen before the queue and need no slot.
	if err := pixelValidateORA(context.Background(), []byte("nope, not a zip at all"), oraTestLimit); !errors.Is(err, errPixelORAInvalid) {
		t.Fatalf("invalid archive with all slots taken: %v", err)
	}
	if err := pixelValidateORA(context.Background(), archive, int64(len(archive))-1); !errors.Is(err, errPixelORATooLarge) {
		t.Fatalf("oversize archive with all slots taken: %v", err)
	}

	// With every slot taken a request waits for as long as its context lives, then gives up.
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := pixelValidateORA(ctx, archive, oraTestLimit); !errors.Is(err, errPixelORABusy) {
		t.Fatalf("waiting request error = %v, want errPixelORABusy", err)
	}
	if waited := time.Since(started); waited < 30*time.Millisecond {
		t.Fatalf("gave up after %v: it did not wait for a slot", waited)
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if err := pixelValidateORA(cancelled, archive, oraTestLimit); !errors.Is(err, errPixelORABusy) {
		t.Fatalf("cancelled request error = %v", err)
	}

	// A waiting request proceeds as soon as one slot is free.
	done := make(chan error, 1)
	go func() { done <- pixelValidateORA(context.Background(), archive, oraTestLimit) }()
	select {
	case err := <-done:
		t.Fatalf("validated while every slot was taken: %v", err)
	case <-time.After(60 * time.Millisecond):
	}
	held[0]()
	held = held[1:]
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("queued validation failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued validation never got the freed slot")
	}
}

// Many simultaneous saves all finish and every slot is handed back (also on the failure paths).
func TestPixelORAConcurrentValidationsFinishAndReleaseTheirSlots(t *testing.T) {
	layer := oraTestConstantPNG(t, 1024, 1024)
	archive := oraTestLayersArchive(t, 1024, 1024, 16, layer, true)
	broken := oraTestArchive(t, oraTestReplace(oraTestEntries(t), "stack.xml", []byte("not xml")))
	var wg sync.WaitGroup
	errs := make(chan error, 24)
	for i := 0; i < 12; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := pixelValidateORA(context.Background(), archive, oraTestLimit); err != nil {
				errs <- err
			}
		}()
		go func() {
			defer wg.Done()
			if err := pixelValidateORA(context.Background(), broken, oraTestLimit); err == nil {
				errs <- errors.New("broken archive accepted")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent validation: %v", err)
	}
	if len(pixelORASlots) != 0 {
		t.Fatalf("%d slots were never released", len(pixelORASlots))
	}
}

// A save request that cannot get a validation slot before its context ends is answered 503, and stores nothing.
func TestPixelSaveAnswers503WhenValidationSlotsStayTaken(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	if _, _, err := s.getDesktopService(context.Background()); err != nil {
		t.Fatal(err)
	}
	var held []func()
	for i := 0; i < pixelORAMaxConcurrent; i++ {
		release, err := pixelORAAcquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, release)
	}
	defer func() {
		for _, release := range held {
			release()
		}
	}()
	archive := oraTestArchive(t, oraTestEntries(t))
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	r := httptest.NewRequest("POST", "/api/pixel/save", bytes.NewReader(oraTestPayload(t, "Pictures/queued.ora", "ora", archive))).WithContext(ctx)
	r.Header.Set("If-None-Match", "*")
	w := httptest.NewRecorder()
	handlePixelSave(s)(w, r)
	if w.Code != 503 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}
