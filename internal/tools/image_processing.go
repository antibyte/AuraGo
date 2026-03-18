package tools

import (
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	"golang.org/x/image/tiff"
)

type imageResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	Format  string `json:"format,omitempty"`
}

func imageJSON(r imageResult) string {
	b, _ := json.Marshal(r)
	return string(b)
}

// ExecuteImageProcessing dispatches image processing operations.
func ExecuteImageProcessing(operation, inputFile, outputFile, outputFormat string, width, height, quality, x, y, cropW, cropH, angle int) string {
	switch strings.ToLower(operation) {
	case "resize":
		return imageResize(inputFile, outputFile, width, height, quality)
	case "convert":
		return imageConvert(inputFile, outputFile, outputFormat, quality)
	case "compress":
		return imageCompress(inputFile, outputFile, quality)
	case "crop":
		return imageCrop(inputFile, outputFile, x, y, cropW, cropH)
	case "rotate":
		return imageRotate(inputFile, outputFile, angle)
	case "info":
		return imageInfo(inputFile)
	default:
		return imageJSON(imageResult{
			Status:  "error",
			Message: fmt.Sprintf("unknown operation: %s (valid: resize, convert, compress, crop, rotate, info)", operation),
		})
	}
}

func loadImage(path string) (image.Image, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()
	img, format, err := image.Decode(f)
	if err != nil {
		return nil, "", fmt.Errorf("cannot decode image: %w", err)
	}
	return img, format, nil
}

func saveImage(img image.Image, path, format string, quality int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("cannot create output directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	defer f.Close()

	switch strings.ToLower(format) {
	case "png":
		return png.Encode(f, img)
	case "jpeg", "jpg":
		q := quality
		if q <= 0 || q > 100 {
			q = 85
		}
		return jpeg.Encode(f, img, &jpeg.Options{Quality: q})
	case "gif":
		return gif.Encode(f, img, nil)
	case "bmp":
		return bmp.Encode(f, img)
	case "tiff", "tif":
		return tiff.Encode(f, img, nil)
	default:
		return fmt.Errorf("unsupported output format: %s (supported: png, jpeg, gif, bmp, tiff)", format)
	}
}

func detectImageFormat(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "jpg":
		return "jpeg"
	case "tif":
		return "tiff"
	case "":
		return "png"
	default:
		return ext
	}
}

func imageResize(inputFile, outputFile string, width, height, quality int) string {
	if inputFile == "" {
		return imageJSON(imageResult{Status: "error", Message: "file_path is required"})
	}
	if width <= 0 && height <= 0 {
		return imageJSON(imageResult{Status: "error", Message: "width and/or height must be > 0"})
	}

	img, srcFormat, err := loadImage(inputFile)
	if err != nil {
		return imageJSON(imageResult{Status: "error", Message: err.Error()})
	}

	bounds := img.Bounds()
	origW, origH := bounds.Dx(), bounds.Dy()

	// Maintain aspect ratio if only one dimension specified
	newW, newH := width, height
	if newW <= 0 {
		newW = origW * newH / origH
	}
	if newH <= 0 {
		newH = origH * newW / origW
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	if outputFile == "" {
		outputFile = addSuffix(inputFile, fmt.Sprintf("_%dx%d", newW, newH))
	}

	outFmt := detectImageFormat(outputFile)
	if outFmt == "" {
		outFmt = srcFormat
	}

	if err := saveImage(dst, outputFile, outFmt, quality); err != nil {
		return imageJSON(imageResult{Status: "error", Message: err.Error()})
	}

	return imageJSON(imageResult{
		Status:  "success",
		Message: fmt.Sprintf("resized %dx%d → %dx%d", origW, origH, newW, newH),
		File:    outputFile,
		Width:   newW,
		Height:  newH,
		Format:  outFmt,
	})
}

func imageConvert(inputFile, outputFile, outputFormat string, quality int) string {
	if inputFile == "" {
		return imageJSON(imageResult{Status: "error", Message: "file_path is required"})
	}
	if outputFormat == "" && outputFile == "" {
		return imageJSON(imageResult{Status: "error", Message: "output_format or output_file is required"})
	}

	img, _, err := loadImage(inputFile)
	if err != nil {
		return imageJSON(imageResult{Status: "error", Message: err.Error()})
	}

	outFmt := strings.ToLower(outputFormat)
	if outFmt == "jpg" {
		outFmt = "jpeg"
	}
	if outFmt == "tif" {
		outFmt = "tiff"
	}

	if outputFile == "" {
		base := strings.TrimSuffix(inputFile, filepath.Ext(inputFile))
		ext := outFmt
		if ext == "jpeg" {
			ext = "jpg"
		}
		outputFile = base + "." + ext
	}
	if outFmt == "" {
		outFmt = detectImageFormat(outputFile)
	}

	if err := saveImage(img, outputFile, outFmt, quality); err != nil {
		return imageJSON(imageResult{Status: "error", Message: err.Error()})
	}

	bounds := img.Bounds()
	return imageJSON(imageResult{
		Status:  "success",
		Message: fmt.Sprintf("converted %s → %s", filepath.Base(inputFile), filepath.Base(outputFile)),
		File:    outputFile,
		Width:   bounds.Dx(),
		Height:  bounds.Dy(),
		Format:  outFmt,
	})
}

func imageCompress(inputFile, outputFile string, quality int) string {
	if inputFile == "" {
		return imageJSON(imageResult{Status: "error", Message: "file_path is required"})
	}
	if quality <= 0 || quality > 100 {
		quality = 75
	}

	img, srcFormat, err := loadImage(inputFile)
	if err != nil {
		return imageJSON(imageResult{Status: "error", Message: err.Error()})
	}

	// Compression only makes sense for JPEG
	outFmt := srcFormat
	if outFmt != "jpeg" {
		outFmt = "jpeg"
	}

	if outputFile == "" {
		base := strings.TrimSuffix(inputFile, filepath.Ext(inputFile))
		outputFile = base + "_compressed.jpg"
	}

	if err := saveImage(img, outputFile, outFmt, quality); err != nil {
		return imageJSON(imageResult{Status: "error", Message: err.Error()})
	}

	origInfo, _ := os.Stat(inputFile)
	newInfo, _ := os.Stat(outputFile)
	var msg string
	if origInfo != nil && newInfo != nil {
		msg = fmt.Sprintf("compressed %s → %s (quality=%d, %s → %s)",
			filepath.Base(inputFile), filepath.Base(outputFile), quality,
			humanSize(origInfo.Size()), humanSize(newInfo.Size()))
	} else {
		msg = fmt.Sprintf("compressed to quality=%d", quality)
	}

	return imageJSON(imageResult{
		Status:  "success",
		Message: msg,
		File:    outputFile,
		Format:  outFmt,
	})
}

func imageCrop(inputFile, outputFile string, x, y, cropW, cropH int) string {
	if inputFile == "" {
		return imageJSON(imageResult{Status: "error", Message: "file_path is required"})
	}
	if cropW <= 0 || cropH <= 0 {
		return imageJSON(imageResult{Status: "error", Message: "crop_width and crop_height must be > 0"})
	}

	img, srcFormat, err := loadImage(inputFile)
	if err != nil {
		return imageJSON(imageResult{Status: "error", Message: err.Error()})
	}

	bounds := img.Bounds()
	cropRect := image.Rect(x, y, x+cropW, y+cropH)
	if !cropRect.In(bounds) {
		return imageJSON(imageResult{
			Status:  "error",
			Message: fmt.Sprintf("crop area (%d,%d,%d,%d) exceeds image bounds (%dx%d)", x, y, x+cropW, y+cropH, bounds.Dx(), bounds.Dy()),
		})
	}

	dst := image.NewRGBA(image.Rect(0, 0, cropW, cropH))
	draw.Copy(dst, image.Point{}, img, cropRect, draw.Over, nil)

	if outputFile == "" {
		outputFile = addSuffix(inputFile, "_cropped")
	}

	outFmt := detectImageFormat(outputFile)
	if outFmt == "" {
		outFmt = srcFormat
	}

	if err := saveImage(dst, outputFile, outFmt, 85); err != nil {
		return imageJSON(imageResult{Status: "error", Message: err.Error()})
	}

	return imageJSON(imageResult{
		Status:  "success",
		Message: fmt.Sprintf("cropped to %dx%d from (%d,%d)", cropW, cropH, x, y),
		File:    outputFile,
		Width:   cropW,
		Height:  cropH,
		Format:  outFmt,
	})
}

func imageRotate(inputFile, outputFile string, angle int) string {
	if inputFile == "" {
		return imageJSON(imageResult{Status: "error", Message: "file_path is required"})
	}

	// Normalize angle to 0, 90, 180, 270
	angle = ((angle % 360) + 360) % 360
	if angle != 90 && angle != 180 && angle != 270 {
		return imageJSON(imageResult{
			Status:  "error",
			Message: fmt.Sprintf("angle must be 90, 180, or 270 (got %d)", angle),
		})
	}

	img, srcFormat, err := loadImage(inputFile)
	if err != nil {
		return imageJSON(imageResult{Status: "error", Message: err.Error()})
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	var dst *image.RGBA
	switch angle {
	case 90:
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
		for iy := bounds.Min.Y; iy < bounds.Max.Y; iy++ {
			for ix := bounds.Min.X; ix < bounds.Max.X; ix++ {
				dst.Set(h-1-(iy-bounds.Min.Y), ix-bounds.Min.X, img.At(ix, iy))
			}
		}
	case 180:
		dst = image.NewRGBA(image.Rect(0, 0, w, h))
		for iy := bounds.Min.Y; iy < bounds.Max.Y; iy++ {
			for ix := bounds.Min.X; ix < bounds.Max.X; ix++ {
				dst.Set(w-1-(ix-bounds.Min.X), h-1-(iy-bounds.Min.Y), img.At(ix, iy))
			}
		}
	case 270:
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
		for iy := bounds.Min.Y; iy < bounds.Max.Y; iy++ {
			for ix := bounds.Min.X; ix < bounds.Max.X; ix++ {
				dst.Set(iy-bounds.Min.Y, w-1-(ix-bounds.Min.X), img.At(ix, iy))
			}
		}
	}

	if outputFile == "" {
		outputFile = addSuffix(inputFile, fmt.Sprintf("_rot%d", angle))
	}

	outFmt := detectImageFormat(outputFile)
	if outFmt == "" {
		outFmt = srcFormat
	}

	if err := saveImage(dst, outputFile, outFmt, 85); err != nil {
		return imageJSON(imageResult{Status: "error", Message: err.Error()})
	}

	newBounds := dst.Bounds()
	return imageJSON(imageResult{
		Status:  "success",
		Message: fmt.Sprintf("rotated %d° (%dx%d → %dx%d)", angle, w, h, newBounds.Dx(), newBounds.Dy()),
		File:    outputFile,
		Width:   newBounds.Dx(),
		Height:  newBounds.Dy(),
		Format:  outFmt,
	})
}

func imageInfo(inputFile string) string {
	if inputFile == "" {
		return imageJSON(imageResult{Status: "error", Message: "file_path is required"})
	}

	img, format, err := loadImage(inputFile)
	if err != nil {
		return imageJSON(imageResult{Status: "error", Message: err.Error()})
	}

	bounds := img.Bounds()
	fi, _ := os.Stat(inputFile)
	var sizeStr string
	if fi != nil {
		sizeStr = humanSize(fi.Size())
	}

	return imageJSON(imageResult{
		Status:  "success",
		Message: fmt.Sprintf("%s: %dx%d %s (%s)", filepath.Base(inputFile), bounds.Dx(), bounds.Dy(), format, sizeStr),
		File:    inputFile,
		Width:   bounds.Dx(),
		Height:  bounds.Dy(),
		Format:  format,
	})
}
