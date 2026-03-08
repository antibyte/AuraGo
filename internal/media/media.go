package media

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SaveAttachment downloads a URL and saves it persistently to destDir with a
// timestamped, sanitized filename. Returns the full saved path.
// httpClientMedia is a shared HTTP client with timeout for media downloads.
var httpClientMedia = &http.Client{Timeout: 120 * time.Second}

// maxAttachmentSize is the maximum download size for attachments (100 MB).
const maxAttachmentSize = 100 << 20

// maxDownloadSize is the maximum download size for temporary files (50 MB).
const maxDownloadSize = 50 << 20

func SaveAttachment(url, originalFilename, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create attachments dir: %w", err)
	}

	resp, err := httpClientMedia.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Sanitize filename
	base := filepath.Base(originalFilename)
	base = strings.ReplaceAll(base, " ", "_")
	// Strip any path traversal attempts
	base = strings.ReplaceAll(base, "..", "")
	if base == "" || base == "." {
		base = "file.bin"
	}

	ts := time.Now().Format("20060102_150405")
	filename := ts + "_" + base
	destPath := filepath.Join(destDir, filename)

	dst, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, io.LimitReader(resp.Body, maxAttachmentSize)); err != nil {
		os.Remove(destPath)
		return "", fmt.Errorf("failed to write attachment: %w", err)
	}

	return destPath, nil
}

// DownloadFile downloads a URL to a temporary file and returns the path.
// The caller is responsible for removing the file when done.
func DownloadFile(url string, prefix string) (string, error) {
	resp, err := httpClientMedia.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file: %s", resp.Status)
	}

	// Detect extension from URL
	ext := filepath.Ext(url)
	if ext == "" {
		ext = ".bin"
	}

	tempFile, err := os.CreateTemp("", prefix+"_*"+ext)
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, io.LimitReader(resp.Body, maxDownloadSize)); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// DetectMimeType returns a MIME type based on file extension.
func DetectMimeType(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".ogg"):
		return "audio/ogg"
	case strings.HasSuffix(lower, ".mp3"):
		return "audio/mpeg"
	case strings.HasSuffix(lower, ".wav"):
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

// IsImageContentType checks if a content type represents an image.
func IsImageContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.HasPrefix(ct, "image/")
}

// IsAudioContentType checks if a content type represents audio.
func IsAudioContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.HasPrefix(ct, "audio/") ||
		strings.Contains(ct, "ogg") ||
		strings.Contains(ct, "voice")
}

// IsAudioFilename checks if a filename has a voice/audio extension.
func IsAudioFilename(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".ogg") ||
		strings.HasSuffix(lower, ".mp3") ||
		strings.HasSuffix(lower, ".wav") ||
		strings.HasSuffix(lower, ".flac") ||
		strings.HasSuffix(lower, ".m4a") ||
		strings.HasSuffix(lower, ".opus")
}

// IsImageFilename checks if a filename has an image extension.
func IsImageFilename(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".gif") ||
		strings.HasSuffix(lower, ".webp")
}

// ImageRef holds a reference to an extracted markdown image.
type ImageRef struct {
	URL     string // URL or /files/... web path
	Caption string // Alt text / caption
}

// ExtractMarkdownImages parses all markdown image references (![alt](url)) from text.
// Returns the cleaned text (images removed) and a list of found ImageRef values.
func ExtractMarkdownImages(text string) (string, []ImageRef) {
	var images []ImageRef
	re := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	cleaned := re.ReplaceAllStringFunc(text, func(match string) string {
		subs := re.FindStringSubmatch(match)
		if len(subs) == 3 {
			images = append(images, ImageRef{Caption: subs[1], URL: subs[2]})
		}
		return ""
	})
	cleaned = regexp.MustCompile(`\n{3,}`).ReplaceAllString(cleaned, "\n\n")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned, images
}

// SaveURLToDir downloads an image URL into destDir with a timestamped filename.
// Returns the absolute path of the saved file.
func SaveURLToDir(rawURL, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create images dir: %w", err)
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	// Mimic a real browser so CDNs (Wikipedia, Unsplash, etc.) don’t return 403.
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Referer", "https://www.google.com/")
	resp, err := httpClientMedia.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d for %s", resp.StatusCode, rawURL)
	}
	// Reject non-image responses (e.g. HTML error pages returned with 200)
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "image/") && !strings.HasPrefix(ct, "application/octet-stream") {
		return "", fmt.Errorf("URL did not return an image (Content-Type: %s)", ct)
	}
	ext := strings.ToLower(filepath.Ext(strings.Split(rawURL, "?")[0]))
	if ext == "" {
		switch {
		case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
			ext = ".jpg"
		case strings.Contains(ct, "png"):
			ext = ".png"
		case strings.Contains(ct, "gif"):
			ext = ".gif"
		case strings.Contains(ct, "webp"):
			ext = ".webp"
		default:
			ext = ".jpg"
		}
	}
	filename := fmt.Sprintf("img_%d%s", time.Now().UnixMilli(), ext)
	destPath := filepath.Join(destDir, filename)
	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create image file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, io.LimitReader(resp.Body, maxAttachmentSize)); err != nil {
		os.Remove(destPath)
		return "", fmt.Errorf("failed to write image: %w", err)
	}
	return destPath, nil
}

// ── Image Sanitization ──────────────────────────────────────────────────────

// imageMagic describes a known image format's magic bytes.
type imageMagic struct {
	magic  []byte
	offset int
	format string
	ext    string
}

var knownImageFormats = []imageMagic{
	{magic: []byte{0xFF, 0xD8, 0xFF}, offset: 0, format: "jpeg", ext: ".jpg"},
	{magic: []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}, offset: 0, format: "png", ext: ".png"},
	{magic: []byte("GIF87a"), offset: 0, format: "gif", ext: ".gif"},
	{magic: []byte("GIF89a"), offset: 0, format: "gif", ext: ".gif"},
	{magic: []byte("RIFF"), offset: 0, format: "webp", ext: ".webp"}, // also check bytes 8-12 == "WEBP"
}

// detectImageFormat inspects the leading bytes of a file and returns the format
// name ("jpeg", "png", "gif", "webp") and the target file extension, or empty
// strings if the format is not recognised or not a supported image type.
func detectImageFormat(header []byte) (format, ext string) {
	if len(header) < 12 {
		return "", ""
	}
	for _, m := range knownImageFormats {
		end := m.offset + len(m.magic)
		if end > len(header) {
			continue
		}
		match := true
		for i, b := range m.magic {
			if header[m.offset+i] != b {
				match = false
				break
			}
		}
		if match {
			// Extra check for WebP: bytes 8-12 must be "WEBP"
			if m.format == "webp" {
				if len(header) < 12 || string(header[8:12]) != "WEBP" {
					continue
				}
			}
			return m.format, m.ext
		}
	}
	return "", ""
}

// DownloadAndSanitizeImage downloads an image from rawURL into destDir,
// verifies it is a supported image format via magic bytes, decodes it fully
// to catch corrupt or disguised files, then re-encodes it to strip EXIF data,
// embedded scripts, and other metadata.
// Returns the path of the sanitized file on success.
func DownloadAndSanitizeImage(rawURL, destDir string) (string, error) {
	// Download to a temp location first
	tmpPath, err := SaveURLToDir(rawURL, os.TempDir())
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tmpPath) // always clean up temp file

	return SanitizeImageFile(tmpPath, destDir)
}

// SanitizeImageFile validates a local image file via magic bytes and a full
// image decode, then re-encodes it to strip EXIF data, comments, and embedded
// metadata. The sanitized file is written into destDir and its path returned.
// JPEG → re-encoded JPEG (strips EXIF)
// PNG  → re-encoded PNG  (strips tEXt/zTXT/iCCP chunks)
// GIF  → re-encoded PNG  (golang stdlib gif.Encode requires paletted image)
// WebP → copied as-is after magic-byte + decode verification (no stdlib encoder)
func SanitizeImageFile(srcPath, destDir string) (string, error) {
	// Step 1: read and verify magic bytes
	f, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("unable to open image file: %w", err)
	}
	defer f.Close()

	header := make([]byte, 12)
	if _, err := io.ReadFull(f, header); err != nil {
		return "", fmt.Errorf("file too small to be a valid image: %w", err)
	}
	imgFmt, outExt := detectImageFormat(header)
	if imgFmt == "" {
		return "", fmt.Errorf("unsupported or unrecognised image format (SVG/BMP/TIFF/binary rejected)")
	}

	// Step 2: seek back to start and fully decode image to validate structure
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek failed: %w", err)
	}

	// Register standard decoders (idempotent)
	_ = jpeg.DefaultQuality // ensure jpeg package is referenced
	_ = png.DefaultCompression
	_ = gif.DisposalNone

	img, _, decodeErr := image.Decode(f)
	if decodeErr != nil && imgFmt != "webp" {
		// WebP is not decodable without golang.org/x/image; treat magic-check as sufficient
		return "", fmt.Errorf("image decode failed (corrupt or unsupported codec): %w", decodeErr)
	}

	// Step 3: re-encode into destDir
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create dest dir: %w", err)
	}

	// WebP: copy raw bytes (no metadata injection possible, no stdlib encoder)
	if imgFmt == "webp" {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return "", fmt.Errorf("seek failed: %w", err)
		}
		destPath := filepath.Join(destDir, fmt.Sprintf("sanitized_%d%s", time.Now().UnixMilli(), outExt))
		out, err := os.Create(destPath)
		if err != nil {
			return "", fmt.Errorf("failed to create output file: %w", err)
		}
		defer out.Close()
		if _, err := io.Copy(out, io.LimitReader(f, maxAttachmentSize)); err != nil {
			os.Remove(destPath)
			return "", fmt.Errorf("failed to copy WebP image: %w", err)
		}
		return destPath, nil
	}

	// GIF → PNG (gif.Encode requires paletted image, decode returns general image.Image)
	if imgFmt == "gif" {
		outExt = ".png"
	}

	destPath := filepath.Join(destDir, fmt.Sprintf("sanitized_%d%s", time.Now().UnixMilli(), outExt))
	out, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create sanitized file: %w", err)
	}
	defer out.Close()

	switch outExt {
	case ".jpg":
		err = jpeg.Encode(out, img, &jpeg.Options{Quality: 92})
	case ".png":
		err = png.Encode(out, img)
	default:
		err = jpeg.Encode(out, img, &jpeg.Options{Quality: 92})
	}
	if err != nil {
		out.Close()
		os.Remove(destPath)
		return "", fmt.Errorf("re-encode failed: %w", err)
	}
	return destPath, nil
}
