package whatsapp

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"aurago/internal/integrationstatus"
	"aurago/internal/media"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

const (
	maxMediaBytes   = 50 << 20
	maxCaptionRunes = 1024
	voiceNoteMIME   = "audio/ogg; codecs=opus"
	ffmpegTimeout   = 60 * time.Second // Telegram's ceiling (internal/telegram/voice.go:92)
)

// mediaKind is the WhatsApp message type a file travels as.
type mediaKind int

const (
	mediaDocument mediaKind = iota
	mediaImage
	mediaVideo
	mediaAudio
	mediaVoiceNote
)

// String is the kind's log and notice name.
func (k mediaKind) String() string {
	switch k {
	case mediaImage:
		return "image"
	case mediaVideo:
		return "video"
	case mediaAudio:
		return "audio"
	case mediaVoiceNote:
		return "voice"
	default:
		return "document"
	}
}

// uploadType is the key family the upload is encrypted under: voice notes and audio files
// share the audio keys, everything unknown is a document.
func (k mediaKind) uploadType() whatsmeow.MediaType {
	switch k {
	case mediaImage:
		return whatsmeow.MediaImage
	case mediaVideo:
		return whatsmeow.MediaVideo
	case mediaAudio, mediaVoiceNote:
		return whatsmeow.MediaAudio
	default:
		return whatsmeow.MediaDocument
	}
}

// mediaKindFor picks the message type from the name and the first bytes. A file becomes an
// image, video or audio message only when its content matches; anything else is a document.
func mediaKindFor(name string, head []byte) (kind mediaKind, mimeType string) {
	sniffed := http.DetectContentType(head)
	ext := strings.ToLower(filepath.Ext(name))
	switch {
	case sniffed == "image/jpeg" || sniffed == "image/png":
		return mediaImage, sniffed
	case sniffed == "video/mp4" && (ext == ".mp4" || ext == ".m4v"):
		return mediaVideo, "video/mp4"
	case ext == ".mp3" && (sniffed == "audio/mpeg" || sniffed == "application/octet-stream"):
		return mediaAudio, "audio/mpeg"
	case ext == ".m4a" && sniffed == "video/mp4":
		return mediaAudio, "audio/mp4"
	case ext == ".aac" && sniffed == "application/octet-stream":
		return mediaAudio, "audio/aac"
	case (ext == ".ogg" || ext == ".opus") && sniffed == "application/ogg":
		return mediaAudio, "audio/ogg"
	}
	return mediaDocument, documentMIME(ext, sniffed)
}

// documentMIME is the document's type: the extension's, without parameters, else the sniffed
// one, else application/octet-stream.
func documentMIME(ext, sniffed string) string {
	if t := mimeWithoutParams(mime.TypeByExtension(ext)); t != "" {
		return t
	}
	if t := mimeWithoutParams(sniffed); t != "" {
		return t
	}
	return "application/octet-stream"
}

func mimeWithoutParams(t string) string {
	base, _, _ := strings.Cut(t, ";")
	return strings.TrimSpace(base)
}

// downloadable is what the download needs from an image, video, audio, document or sticker.
type downloadable interface {
	whatsmeow.DownloadableMessage
	GetFileLength() uint64
}

// downloadMedia returns a temp file (mode 0600) holding the decrypted attachment, at offset 0.
// The caller closes and removes it. The declared FileLength is refused first, then cappedFile
// bounds what really arrives (FileLength is sender-declared, download.go:316).
func downloadMedia(ctx context.Context, client waClient, msg downloadable) (*os.File, error) {
	return downloadMediaLimit(ctx, client, msg, maxMediaBytes)
}

func downloadMediaLimit(ctx context.Context, client waClient, msg downloadable, limit int64) (*os.File, error) {
	if msg.GetFileLength() > uint64(limit) {
		return nil, ErrMediaTooLarge
	}
	f, err := os.CreateTemp("", "aurago-whatsapp-media-*")
	if err != nil {
		return nil, err
	}
	dctx, cancel := context.WithCancel(ctx)
	defer cancel()
	capped := &cappedFile{f: f, limit: limit + 32, cancel: cancel} // padding block + 10-byte MAC
	err = client.DownloadToFile(dctx, msg, capped)
	if err == nil {
		if info, statErr := f.Stat(); statErr != nil || info.Size() > limit {
			err = ErrMediaTooLarge
		}
	}
	if capped.over.Load() {
		err = ErrMediaTooLarge
	}
	if err == nil {
		_, err = f.Seek(0, io.SeekStart)
	}
	if err != nil {
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
		return nil, err
	}
	return f, nil
}

// cappedFile is the whatsmeow.File a download writes into. It holds *os.File in a named field:
// embedding would promote ReadFrom, and io.Copy (download-to-file.go:205) would then write
// around the cap. It is not an *os.File, so whatsmeow preallocates nothing (:197-203). The first
// write past limit trips it for good and cancels the download, so no other host is tried (:111).
type cappedFile struct {
	f      *os.File
	limit  int64
	cancel context.CancelFunc
	over   atomic.Bool
}

// check trips the cap when a write of n bytes at off would end past limit, or when it
// tripped before. The comparison is written so that a huge off cannot overflow.
func (c *cappedFile) check(off int64, n int) error {
	if c.over.Load() || off > c.limit-int64(n) {
		c.over.Store(true)
		c.cancel()
		return ErrMediaTooLarge
	}
	return nil
}

func (c *cappedFile) Write(p []byte) (int, error) {
	pos, err := c.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	if err := c.check(pos, len(p)); err != nil {
		return 0, err
	}
	return c.f.Write(p)
}

func (c *cappedFile) WriteAt(p []byte, off int64) (int, error) {
	if err := c.check(off, len(p)); err != nil {
		return 0, err
	}
	return c.f.WriteAt(p, off)
}

// Truncate can grow a file, so it checks the new size like a write that ends there.
func (c *cappedFile) Truncate(size int64) error {
	if err := c.check(size, 0); err != nil {
		return err
	}
	return c.f.Truncate(size)
}

func (c *cappedFile) Read(p []byte) (int, error)                { return c.f.Read(p) }
func (c *cappedFile) Seek(off int64, whence int) (int64, error) { return c.f.Seek(off, whence) }
func (c *cappedFile) ReadAt(p []byte, off int64) (int, error)   { return c.f.ReadAt(p, off) }
func (c *cappedFile) Stat() (os.FileInfo, error)                { return c.f.Stat() }

// mediaErrorCode is the fixed code a media failure logs; the error text can carry file
// names, URLs or numbers and is never logged.
func mediaErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrMediaTooLarge):
		return "too_large"
	case errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith404), errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith410):
		return "expired"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "cancelled"
	default:
		return "failed"
	}
}

// mediaUploadTimeout mirrors telegramUploadTimeout (internal/tools/telegram_document.go:64):
// a minute plus a second per 128 KiB, at most ten minutes.
func mediaUploadTimeout(size int64) time.Duration {
	return min(time.Minute+time.Duration(max(size, 0)/(128<<10))*time.Second, 10*time.Minute)
}

// uploadMedia uploads r under kind's key type within mediaUploadTimeout(size). A failure
// that is not a cancellation counts MarkWhatsAppDeliveryFailure once: the file never arrives.
func uploadMedia(ctx context.Context, client waClient, r io.Reader, size int64, kind mediaKind) (whatsmeow.UploadResponse, error) {
	uctx, cancel := context.WithTimeout(ctx, mediaUploadTimeout(size))
	defer cancel()
	up, err := client.Upload(uctx, r, kind.uploadType())
	if err != nil {
		if ctx.Err() == nil {
			integrationstatus.MarkWhatsAppDeliveryFailure()
		}
		return whatsmeow.UploadResponse{}, fmt.Errorf("whatsapp %s upload: %w", kind, err)
	}
	return up, nil
}

// mediaMeta is what a media message carries besides the upload.
type mediaMeta struct {
	MIME, FileName, Caption string
	Seconds                 uint32
}

// mediaMessage builds the message for kind from the upload (upload.go:52-72). Voice notes are
// PTT with voiceNoteMIME and no caption; video carries StreamingSidecar; a document carries
// FileName (documentName) and Title = FileName. Empty strings and zero Seconds stay nil.
func mediaMessage(kind mediaKind, up whatsmeow.UploadResponse, meta mediaMeta) *waE2E.Message {
	url, directPath, length := proto.String(up.URL), proto.String(up.DirectPath), proto.Uint64(up.FileLength)
	mimeType, caption, seconds := optString(meta.MIME), optString(meta.Caption), optSeconds(meta.Seconds)
	switch kind {
	case mediaImage:
		return &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
			URL: url, DirectPath: directPath, MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: length,
			Mimetype: mimeType, Caption: caption,
		}}
	case mediaVideo:
		return &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
			URL: url, DirectPath: directPath, MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: length,
			Mimetype: mimeType, Caption: caption, Seconds: seconds,
			StreamingSidecar: up.StreamingSidecar,
		}}
	case mediaAudio:
		return &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
			URL: url, DirectPath: directPath, MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: length,
			Mimetype: mimeType, Seconds: seconds,
		}}
	case mediaVoiceNote:
		return &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
			URL: url, DirectPath: directPath, MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: length,
			Mimetype: proto.String(voiceNoteMIME), Seconds: seconds, PTT: proto.Bool(true),
		}}
	default:
		name := documentName(meta.FileName)
		return &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
			URL: url, DirectPath: directPath, MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: length,
			Mimetype: mimeType, Caption: caption,
			FileName: proto.String(name), Title: proto.String(name),
		}}
	}
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return proto.String(s)
}

func optSeconds(n uint32) *uint32 {
	if n == 0 {
		return nil
	}
	return proto.Uint32(n)
}

// documentName is the base name a document is stored and sent under. It is
// media.SanitizeFileName (base name only; reserved characters, control and format
// characters replaced; trailing dots and spaces removed; about 100 bytes at most); a name
// without a usable base becomes "document". telegramDocumentName
// (internal/tools/telegram_document.go:392-405) is the weaker ancestor.
func documentName(name string) string {
	if safe := media.SanitizeFileName(name); safe != "" {
		return safe
	}
	return "document"
}

// boundRunes cuts s to n runes, a trailing "…" included, like boundPhotoCaption
// (internal/telegram/send_photo.go:67-73).
func boundRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

const (
	// ffmpeg runs on files a remote party sent, so it may open local files and pipes only,
	// and only audio and video containers: no playlists, concat scripts or network demuxers.
	// This follows internal/videostudio/probe.go (allowedProtocols, allowedDemuxers) and
	// internal/tools/media_conversion.go (mediaConversionProtocols).
	ffmpegProtocols = "file,pipe"
	ffmpegDemuxers  = "ogg,mp3,mov,matroska,wav,aac,flac,amr,mpeg,mpegts,asf,avi,flv,webm"
)

var errEmptyConversion = errors.New("ffmpeg produced no audio")

// ffmpegBaseArgs starts every conversion: quiet, no stdin, and the protocol and container
// whitelists, which apply to the input that follows.
func ffmpegBaseArgs() []string {
	return []string{"-nostdin", "-hide_banner", "-loglevel", "error",
		"-protocol_whitelist", ffmpegProtocols, "-format_whitelist", ffmpegDemuxers}
}

// outputLimitArgs stops ffmpeg from writing more than maxMediaBytes (-fs allows "slightly
// more"; convertedSize treats a result at the limit as cut short).
func outputLimitArgs() []string { return []string{"-fs", strconv.FormatInt(maxMediaBytes, 10)} }

// voiceNoteArgs is the ffmpeg command line for a WhatsApp voice note: Ogg Opus, mono,
// 16 kHz, voip tuning, no input metadata, at most 50 MB of output. out is a file the caller
// created with os.CreateTemp.
func voiceNoteArgs(in, out string) []string {
	args := append(ffmpegBaseArgs(), "-y", "-i", in,
		"-vn", "-map_metadata", "-1",
		"-c:a", "libopus", "-application", "voip", "-ac", "1", "-ar", "16000", "-b:a", "24k")
	args = append(args, outputLimitArgs()...)
	return append(args, "-f", "ogg", out)
}

// mp3Args is the ffmpeg command line that turns an inbound voice message or audio file into
// the MP3 the speech-to-text call reads: mono, 16 kHz and 64 kbit/s are plenty for speech
// and keep the result small. The input is restricted like voiceNoteArgs', and the output is
// at most 50 MB. out is a file the caller created with os.CreateTemp.
func mp3Args(in, out string) []string {
	args := append(ffmpegBaseArgs(), "-y", "-i", in,
		"-vn", "-map_metadata", "-1", "-ac", "1", "-ar", "16000", "-b:a", "64k")
	args = append(args, outputLimitArgs()...)
	return append(args, "-f", "mp3", out)
}

// runFFmpeg runs ffmpeg within ffmpegTimeout. Stdout and Stderr stay nil, so ffmpeg's output is
// discarded and never logged (it can echo the input path).
func runFFmpeg(ctx context.Context, args []string) error {
	cctx, cancel := context.WithTimeout(ctx, ffmpegTimeout)
	defer cancel()
	if err := exec.CommandContext(cctx, "ffmpeg", args...).Run(); err != nil {
		if cerr := cctx.Err(); cerr != nil {
			return fmt.Errorf("ffmpeg conversion stopped: %w", cerr)
		}
		return fmt.Errorf("ffmpeg conversion failed: %w", err)
	}
	return nil
}

// convertedSize is the size of a conversion result. A result at or above maxMediaBytes was cut
// short by -fs: that is ErrMediaTooLarge, never a truncated success. An empty result is a
// failure too.
func convertedSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("read converted audio: %w", err)
	}
	switch size := info.Size(); {
	case size >= maxMediaBytes:
		return 0, ErrMediaTooLarge
	case size == 0:
		return 0, errEmptyConversion
	default:
		return size, nil
	}
}

// convertToMP3 converts an inbound audio file to MP3 for transcription. WhatsApp has its own
// converter, not telegram.ConvertOggToMp3Context: that one has no input whitelist and no
// output limit. outputPath must be a fresh file from os.CreateTemp.
func convertToMP3(ctx context.Context, inputPath, outputPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := runFFmpeg(ctx, mp3Args(inputPath, outputPath)); err != nil {
		return err
	}
	_, err := convertedSize(outputPath)
	return err
}

// convertToVoiceNote runs ffmpeg with voiceNoteArgs within ffmpegTimeout and returns the Ogg Opus
// bytes and their seconds. ffmpeg output is never logged (it can echo the input path).
func convertToVoiceNote(ctx context.Context, inputPath string) ([]byte, uint32, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	out, err := os.CreateTemp("", "aurago-whatsapp-voice-*.ogg")
	if err != nil {
		return nil, 0, fmt.Errorf("create temporary voice note: %w", err)
	}
	outPath := out.Name()
	_ = out.Close()
	defer os.Remove(outPath)

	if err := runFFmpeg(ctx, voiceNoteArgs(inputPath, outPath)); err != nil {
		return nil, 0, err
	}
	if _, err := convertedSize(outPath); err != nil {
		return nil, 0, err
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, 0, fmt.Errorf("read converted voice note: %w", err)
	}
	return data, oggOpusSeconds(data), nil
}

// oggOpusSeconds is the stream duration: the last page's granule position counts 48 kHz
// samples (RFC 7845) minus the OpusHead pre-skip (bytes 10-11), rounded up. 0 = unknown;
// display only.
func oggOpusSeconds(data []byte) uint32 {
	head := bytes.Index(data, []byte("OpusHead"))
	last := bytes.LastIndex(data, []byte("OggS"))
	if head < 0 || last < 0 || len(data) < last+14 || len(data) < head+12 {
		return 0
	}
	granule := int64(binary.LittleEndian.Uint64(data[last+6 : last+14]))
	preSkip := int64(binary.LittleEndian.Uint16(data[head+10 : head+12]))
	if granule <= preSkip {
		return 0
	}
	return uint32((granule - preSkip + 47999) / 48000)
}
