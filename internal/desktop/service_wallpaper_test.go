package desktop

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func encodeWallpaperPNG(t *testing.T, width, height int, fill color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if fill != nil {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				img.Set(x, y, fill)
			}
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("encode PNG fixture: %v", err)
	}
	return buffer.Bytes()
}

func pngWithDimensions(t *testing.T, width, height int) []byte {
	t.Helper()
	data := encodeWallpaperPNG(t, 1, 1, nil)
	binary.BigEndian.PutUint32(data[16:20], uint32(width))
	binary.BigEndian.PutUint32(data[20:24], uint32(height))
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	return data
}

func writeWallpaperFixture(t *testing.T, svc *Service, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(svc.Config().WorkspaceDir, filepath.FromSlash(path))), 0o700); err != nil {
		t.Fatalf("create wallpaper fixture directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(svc.Config().WorkspaceDir, filepath.FromSlash(path)), data, 0o600); err != nil {
		t.Fatalf("write wallpaper fixture %q: %v", path, err)
	}
}

func wallpaperCode(t *testing.T, err error) string {
	t.Helper()
	var wallpaperErr *WallpaperError
	if !errors.As(err, &wallpaperErr) {
		t.Fatalf("error %v is not a WallpaperError", err)
	}
	return wallpaperErr.Code
}

func TestReadCustomWallpaperReturnsValidatedSnapshot(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	first := encodeWallpaperPNG(t, 3, 2, color.RGBA{R: 0xff, A: 0xff})
	second := encodeWallpaperPNG(t, 2, 4, color.RGBA{B: 0xff, A: 0xff})
	writeWallpaperFixture(t, svc, "Pictures/wall.png", first)

	got, err := svc.ReadCustomWallpaper(ctx, "Pictures/wall.png")
	if err != nil {
		t.Fatalf("ReadCustomWallpaper: %v", err)
	}
	if got.Path != "Pictures/wall.png" || got.MIMEType != "image/png" || got.Width != 3 || got.Height != 2 || !bytes.Equal(got.Bytes, first) {
		t.Fatalf("validated wallpaper = path %q, MIME %q, size %dx%d, %d bytes", got.Path, got.MIMEType, got.Width, got.Height, len(got.Bytes))
	}

	// A repeated read of the same path must validate and return the new file contents.
	writeWallpaperFixture(t, svc, "Pictures/wall.png", second)
	got, err = svc.ReadCustomWallpaper(ctx, "Pictures/wall.png")
	if err != nil {
		t.Fatalf("ReadCustomWallpaper after replacement: %v", err)
	}
	if got.Width != 2 || got.Height != 4 || !bytes.Equal(got.Bytes, second) {
		t.Fatalf("replacement wallpaper was not re-read: %+v", got)
	}
	if err := svc.ValidateWallpaperValue(ctx, "file:Pictures/wall.png"); err != nil {
		t.Fatalf("ValidateWallpaperValue: %v", err)
	}
	writeWallpaperFixture(t, svc, "Pictures/wall.png", []byte("not an image"))
	if err := svc.ValidateWallpaperValue(ctx, "file:Pictures/wall.png"); wallpaperCode(t, err) != "wallpaper_invalid_image" {
		t.Fatalf("changed invalid file error code = %q", wallpaperCode(t, err))
	}
	if err := svc.ValidateWallpaperValue(ctx, "groupshoot"); err != nil {
		t.Fatalf("built-in value should be left to the existing setting validator: %v", err)
	}
}

func TestCustomWallpaperRejectsPathsAndFormatMismatch(t *testing.T) {
	svc := testService(t)
	valid := encodeWallpaperPNG(t, 2, 1, nil)
	writeWallpaperFixture(t, svc, "Pictures/wall.png", valid)
	for _, path := range []string{"", "/etc/passwd", "../escape.png", "Pictures/../wall.png", `Pictures\\wall.png`, "Pictures//wall.png", "Pictures/./wall.png"} {
		if _, err := svc.ReadCustomWallpaper(context.Background(), path); wallpaperCode(t, err) != "wallpaper_invalid_path" {
			t.Errorf("ReadCustomWallpaper(%q) code = %q, want invalid path", path, wallpaperCode(t, err))
		}
	}
	if _, err := svc.ReadCustomWallpaper(context.Background(), "Pictures/wall.gif"); wallpaperCode(t, err) != "wallpaper_unsupported_format" {
		t.Fatalf("unsupported extension error code = %q", wallpaperCode(t, err))
	}
	writeWallpaperFixture(t, svc, "Pictures/wall.jpg", valid)
	if _, err := svc.ReadCustomWallpaper(context.Background(), "Pictures/wall.jpg"); wallpaperCode(t, err) != "wallpaper_format_mismatch" {
		t.Fatalf("mismatched format error code = %q", wallpaperCode(t, err))
	}
}

func TestCustomWallpaperRejectsSymlinksAndOutsideRoot(t *testing.T) {
	svc := testService(t)
	workspace := svc.Config().WorkspaceDir
	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, encodeWallpaperPNG(t, 2, 2, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(workspace, "Pictures", "outside.png")
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	if _, err := svc.ReadCustomWallpaper(context.Background(), "Pictures/outside.png"); wallpaperCode(t, err) != "wallpaper_invalid_path" {
		t.Fatalf("outside symlink error code = %q", wallpaperCode(t, err))
	}

	outsideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideDir, "wall.png"), encodeWallpaperPNG(t, 1, 1, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(workspace, "Pictures", "linked")); err != nil {
		t.Skipf("directory symlink creation is unavailable: %v", err)
	}
	if _, err := svc.ReadCustomWallpaper(context.Background(), "Pictures/linked/wall.png"); wallpaperCode(t, err) != "wallpaper_invalid_path" {
		t.Fatalf("outside directory symlink error code = %q", wallpaperCode(t, err))
	}
}

func TestCustomWallpaperRejectsSizeDimensionsAndPixels(t *testing.T) {
	svc := testService(t)
	workspace := svc.Config().WorkspaceDir
	writeWallpaperFixture(t, svc, "Pictures/wide.png", pngWithDimensions(t, customWallpaperMaxDimension+1, 1))
	if _, err := svc.ReadCustomWallpaper(context.Background(), "Pictures/wide.png"); wallpaperCode(t, err) != "wallpaper_dimensions_exceeded" {
		t.Fatalf("dimension limit code = %q", wallpaperCode(t, err))
	}
	writeWallpaperFixture(t, svc, "Pictures/pixels.png", pngWithDimensions(t, 4097, 4097))
	if _, err := svc.ReadCustomWallpaper(context.Background(), "Pictures/pixels.png"); wallpaperCode(t, err) != "wallpaper_pixel_limit" {
		t.Fatalf("pixel limit code = %q", wallpaperCode(t, err))
	}
	largePath := filepath.Join(workspace, "Pictures", "large.png")
	file, err := os.Create(largePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(customWallpaperMaxBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReadCustomWallpaper(context.Background(), "Pictures/large.png"); wallpaperCode(t, err) != "wallpaper_too_large" {
		t.Fatalf("byte limit code = %q", wallpaperCode(t, err))
	}
}

func TestCustomWallpaperPixelLimitBoundary(t *testing.T) {
	if !customWallpaperDimensionsAllowed(4000, 4000) {
		t.Fatal("exactly 16,000,000 pixels should be allowed")
	}
	if customWallpaperDimensionsAllowed(4000, 4001) {
		t.Fatal("16,004,000 pixels should exceed the 16-megapixel limit")
	}
	if customWallpaperDimensionsAllowed(customWallpaperMaxDimension+1, 1) {
		t.Fatal("dimensions above 8192 should be rejected independently of pixel count")
	}
}

func TestCustomWallpaperRejectsMalformedAndTruncatedImages(t *testing.T) {
	svc := testService(t)
	valid := encodeWallpaperPNG(t, 2, 2, nil)
	writeWallpaperFixture(t, svc, "Pictures/truncated.png", valid[:len(valid)-8])
	if _, err := svc.ReadCustomWallpaper(context.Background(), "Pictures/truncated.png"); wallpaperCode(t, err) != "wallpaper_invalid_image" {
		t.Fatalf("truncated PNG code = %q", wallpaperCode(t, err))
	}
}

func TestCustomWallpaperSupportsJPEGAndBMP(t *testing.T) {
	svc := testService(t)
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, img, nil); err != nil {
		t.Fatal(err)
	}
	writeWallpaperFixture(t, svc, "Pictures/photo.jpg", jpegData.Bytes())
	got, err := svc.ReadCustomWallpaper(context.Background(), "Pictures/photo.jpg")
	if err != nil || got.MIMEType != "image/jpeg" {
		t.Fatalf("JPEG validation = %+v, %v", got, err)
	}

	bmpData := make([]byte, 58)
	copy(bmpData[:2], "BM")
	binary.LittleEndian.PutUint32(bmpData[2:6], uint32(len(bmpData)))
	binary.LittleEndian.PutUint32(bmpData[10:14], 54)
	binary.LittleEndian.PutUint32(bmpData[14:18], 40)
	binary.LittleEndian.PutUint32(bmpData[18:22], 1)
	binary.LittleEndian.PutUint32(bmpData[22:26], 1)
	binary.LittleEndian.PutUint16(bmpData[26:28], 1)
	binary.LittleEndian.PutUint16(bmpData[28:30], 24)
	binary.LittleEndian.PutUint32(bmpData[34:38], 4)
	bmpData[54], bmpData[55], bmpData[56], bmpData[57] = 0x20, 0x80, 0xe0, 0
	writeWallpaperFixture(t, svc, "Pictures/pixel.bmp", bmpData)
	got, err = svc.ReadCustomWallpaper(context.Background(), "Pictures/pixel.bmp")
	if err != nil || got.MIMEType != "image/bmp" || got.Width != 1 || got.Height != 1 {
		t.Fatalf("BMP validation = %+v, %v", got, err)
	}

	webpFixture, err := base64.StdEncoding.DecodeString("UklGRhwAAABXRUJQVlA4TA8AAAAvAUAAAAcQ9Y/+ByKi/wEA")
	if err != nil {
		t.Fatalf("decode WebP fixture: %v", err)
	}
	writeWallpaperFixture(t, svc, "Pictures/pixel.webp", webpFixture)
	got, err = svc.ReadCustomWallpaper(context.Background(), "Pictures/pixel.webp")
	if err != nil || got.MIMEType != "image/webp" || got.Width != 2 || got.Height != 2 {
		t.Fatalf("WebP validation = %+v, %v", got, err)
	}
}

func TestCustomWallpaperHonorsCanceledContext(t *testing.T) {
	svc := testService(t)
	writeWallpaperFixture(t, svc, "Pictures/wall.png", encodeWallpaperPNG(t, 1, 1, nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.ReadCustomWallpaper(ctx, "Pictures/wall.png"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled wallpaper read error = %v", err)
	}
}
