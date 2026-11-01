package whatsapp

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// isolateTempDir points os.TempDir at a fresh directory, so a test can see which temp files a
// download leaves behind.
func isolateTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, dir)
	}
	return dir
}

func downloadFixture(path string, declared uint64) *waE2E.DocumentMessage {
	return &waE2E.DocumentMessage{DirectPath: proto.String(path), FileLength: proto.Uint64(declared)}
}

func TestDownloadMediaRefusesADeclaredSizeOverTheCap(t *testing.T) {
	fake := &fakeClient{media: map[string][]byte{"/v/big": []byte("x")}}
	f, err := downloadMedia(context.Background(), fake, downloadFixture("/v/big", maxMediaBytes+1))
	if !errors.Is(err, ErrMediaTooLarge) {
		t.Fatalf("downloadMedia error = %v, want ErrMediaTooLarge", err)
	}
	if f != nil {
		t.Fatal("downloadMedia returned a file for a refused download")
	}
	if len(fake.downloads) != 0 {
		t.Fatalf("a refused download reached the network: %v", fake.downloads)
	}
}

func TestDownloadMediaStopsWritingAtTheCap(t *testing.T) {
	tmp := isolateTempDir(t)
	fake := &fakeClient{media: map[string][]byte{"/v/lying": bytes.Repeat([]byte("a"), 4096)}}
	// The sender declares 100 bytes and delivers 4096.
	f, err := downloadMediaLimit(context.Background(), fake, downloadFixture("/v/lying", 100), 1024)
	if !errors.Is(err, ErrMediaTooLarge) {
		t.Fatalf("downloadMediaLimit error = %v, want ErrMediaTooLarge", err)
	}
	if f != nil {
		t.Fatal("downloadMediaLimit returned a file for an oversized download")
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Fatalf("the temp file was not removed: %d entries", len(left))
	}
	if len(fake.downloadCtxErr) != 1 || !errors.Is(fake.downloadCtxErr[0], context.Canceled) {
		t.Fatalf("the trip did not cancel the download context: %v", fake.downloadCtxErr)
	}
}

func TestDownloadMediaKeepsOnlyBytesWithinTheLimit(t *testing.T) {
	isolateTempDir(t)
	for _, tc := range []struct {
		name string
		size int
		ok   bool
	}{
		{"exactly the limit", 1024, true},
		{"within the padding allowance but over the limit", 1030, false},
		{"far over the limit", 4096, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := bytes.Repeat([]byte("b"), tc.size)
			fake := &fakeClient{media: map[string][]byte{"/v/x": want}}
			f, err := downloadMediaLimit(context.Background(), fake, downloadFixture("/v/x", 1), 1024)
			if !tc.ok {
				if !errors.Is(err, ErrMediaTooLarge) || f != nil {
					t.Fatalf("got (%v, %v), want ErrMediaTooLarge and no file", f, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("downloadMediaLimit: %v", err)
			}
			defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
			got, err := io.ReadAll(f)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("the returned file is not at offset 0 with the bytes: err=%v len=%d", err, len(got))
			}
		})
	}
}

func TestCappedFileHidesReaderFromAndWriterTo(t *testing.T) {
	var c any = &cappedFile{}
	if _, ok := c.(io.ReaderFrom); ok {
		t.Error("cappedFile implements io.ReaderFrom: io.Copy would write around the cap")
	}
	if _, ok := c.(io.WriterTo); ok {
		t.Error("cappedFile implements io.WriterTo")
	}
	if _, ok := c.(whatsmeow.File); !ok {
		t.Error("cappedFile does not implement whatsmeow.File")
	}
}

func TestCappedFileTripsForGoodAndCancels(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "capped-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cancelled := 0
	c := &cappedFile{f: f, limit: 10, cancel: func() { cancelled++ }}
	if n, err := c.Write([]byte("0123456789")); n != 10 || err != nil {
		t.Fatalf("a write up to the limit = (%d, %v)", n, err)
	}
	if _, err := c.WriteAt([]byte("x"), 10); !errors.Is(err, ErrMediaTooLarge) {
		t.Fatalf("WriteAt past the limit = %v, want ErrMediaTooLarge", err)
	}
	if _, err := c.WriteAt([]byte("y"), 0); !errors.Is(err, ErrMediaTooLarge) {
		t.Fatalf("a write after the trip = %v, want ErrMediaTooLarge", err)
	}
	if _, err := c.Write(nil); !errors.Is(err, ErrMediaTooLarge) {
		t.Fatalf("Write after the trip = %v, want ErrMediaTooLarge", err)
	}
	if cancelled == 0 {
		t.Fatal("the trip did not cancel the download")
	}
}

func TestMediaKindForMapsContentToMessageTypes(t *testing.T) {
	jpeg := append([]byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00"), make([]byte, 32)...)
	png := append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), make([]byte, 32)...)
	mp4 := []byte("\x00\x00\x00\x18ftypisom\x00\x00\x00\x00isommp42\x00\x00\x00\x08free")
	id3 := append([]byte("ID3\x03\x00\x00\x00\x00\x00\x00"), make([]byte, 32)...)
	rawMP3 := []byte("\xff\xfb\x90\x00\x00\x00\x00\x00")
	adts := []byte("\xff\xf1\x50\x80\x00\x1f\xfc\x00")
	ogg := append([]byte("OggS\x00\x02"), make([]byte, 32)...)
	gif := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00")
	webp := []byte("RIFF\x24\x00\x00\x00WEBPVP8 ")
	wav := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")
	pdf := []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	text := []byte("just some text, not an image")

	tests := []struct {
		name     string
		file     string
		head     []byte
		wantKind mediaKind
		wantMIME string // "" = not asserted (the host's MIME database decides)
	}{
		{"jpeg under any extension", "photo.bin", jpeg, mediaImage, "image/jpeg"},
		{"png under any extension", "photo.dat", png, mediaImage, "image/png"},
		{"png bytes under a jpg name", "photo.jpg", png, mediaImage, "image/png"},
		{"text under a png name", "photo.png", text, mediaDocument, "image/png"},
		{"mp4", "clip.mp4", mp4, mediaVideo, "video/mp4"},
		{"m4v", "clip.M4V", mp4, mediaVideo, "video/mp4"},
		{"mp4 content under another extension", "clip.mov", mp4, mediaDocument, ""},
		{"mp4 name with other content", "clip.mp4", text, mediaDocument, ""},
		{"mp3 with an id3 tag", "song.mp3", id3, mediaAudio, "audio/mpeg"},
		{"mp3 without a tag", "song.mp3", rawMP3, mediaAudio, "audio/mpeg"},
		{"mp3 name with text", "song.mp3", text, mediaDocument, ""},
		{"m4a", "song.m4a", mp4, mediaAudio, "audio/mp4"},
		{"aac", "song.aac", adts, mediaAudio, "audio/aac"},
		{"ogg", "song.ogg", ogg, mediaAudio, "audio/ogg"},
		{"opus", "song.opus", ogg, mediaAudio, "audio/ogg"},
		{"ogg name with text", "song.ogg", text, mediaDocument, ""},
		{"gif stays a document", "anim.gif", gif, mediaDocument, "image/gif"},
		{"webp stays a document", "pic.webp", webp, mediaDocument, "image/webp"},
		{"wav stays a document", "sound.wav", wav, mediaDocument, ""},
		{"pdf", "report.pdf", pdf, mediaDocument, "application/pdf"},
		{"unknown extension with text", "notes.zzqq", text, mediaDocument, "text/plain"},
		{"unknown extension with binary", "blob.zzqq", []byte("\x00\x01\x02\x03"), mediaDocument, "application/octet-stream"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kind, mimeType := mediaKindFor(tc.file, tc.head)
			if kind != tc.wantKind {
				t.Errorf("kind = %v, want %v", kind, tc.wantKind)
			}
			if tc.wantMIME != "" && mimeType != tc.wantMIME {
				t.Errorf("mimetype = %q, want %q", mimeType, tc.wantMIME)
			}
			if strings.Contains(mimeType, ";") || mimeType == "" {
				t.Errorf("mimetype %q is empty or has parameters", mimeType)
			}
		})
	}
}

func TestMediaMessageCopiesTheUploadAndMarksVoiceNotes(t *testing.T) {
	up := whatsmeow.UploadResponse{
		URL: "https://mmg.example/u", DirectPath: "/v/fake",
		MediaKey: []byte{1}, FileSHA256: []byte{2}, FileEncSHA256: []byte{3},
		FileLength: 1234, StreamingSidecar: []byte{4},
	}
	type copied struct {
		url, directPath  *string
		key, sha, encSHA []byte
		length           *uint64
	}
	check := func(t *testing.T, got copied) {
		t.Helper()
		if got.url == nil || *got.url != up.URL || got.directPath == nil || *got.directPath != up.DirectPath {
			t.Errorf("URL or DirectPath not copied: %v %v", got.url, got.directPath)
		}
		if !bytes.Equal(got.key, up.MediaKey) || !bytes.Equal(got.sha, up.FileSHA256) || !bytes.Equal(got.encSHA, up.FileEncSHA256) {
			t.Errorf("key or hashes not copied: %v %v %v", got.key, got.sha, got.encSHA)
		}
		if got.length == nil || *got.length != up.FileLength {
			t.Errorf("FileLength not copied: %v", got.length)
		}
	}

	t.Run("voice note", func(t *testing.T) {
		m := mediaMessage(mediaVoiceNote, up, mediaMeta{MIME: "audio/mpeg", Caption: "ignored", Seconds: 7}).GetAudioMessage()
		if m == nil {
			t.Fatal("no AudioMessage")
		}
		check(t, copied{m.URL, m.DirectPath, m.MediaKey, m.FileSHA256, m.FileEncSHA256, m.FileLength})
		if !m.GetPTT() || m.GetMimetype() != "audio/ogg; codecs=opus" || m.GetSeconds() != 7 {
			t.Errorf("voice note PTT=%v mimetype=%q seconds=%d", m.GetPTT(), m.GetMimetype(), m.GetSeconds())
		}
	})
	t.Run("voice note without a known duration", func(t *testing.T) {
		m := mediaMessage(mediaVoiceNote, up, mediaMeta{}).GetAudioMessage()
		if m.Seconds != nil {
			t.Errorf("Seconds = %d, want unset", *m.Seconds)
		}
	})
	t.Run("audio file", func(t *testing.T) {
		m := mediaMessage(mediaAudio, up, mediaMeta{MIME: "audio/mpeg"}).GetAudioMessage()
		if m == nil {
			t.Fatal("no AudioMessage")
		}
		check(t, copied{m.URL, m.DirectPath, m.MediaKey, m.FileSHA256, m.FileEncSHA256, m.FileLength})
		if m.GetPTT() || m.GetMimetype() != "audio/mpeg" {
			t.Errorf("audio file PTT=%v mimetype=%q", m.GetPTT(), m.GetMimetype())
		}
	})
	t.Run("image", func(t *testing.T) {
		m := mediaMessage(mediaImage, up, mediaMeta{MIME: "image/png", Caption: "look"}).GetImageMessage()
		if m == nil {
			t.Fatal("no ImageMessage")
		}
		check(t, copied{m.URL, m.DirectPath, m.MediaKey, m.FileSHA256, m.FileEncSHA256, m.FileLength})
		if m.GetMimetype() != "image/png" || m.GetCaption() != "look" {
			t.Errorf("image mimetype=%q caption=%q", m.GetMimetype(), m.GetCaption())
		}
	})
	t.Run("image without a caption", func(t *testing.T) {
		m := mediaMessage(mediaImage, up, mediaMeta{MIME: "image/png"}).GetImageMessage()
		if m.Caption != nil {
			t.Errorf("Caption = %q, want unset", *m.Caption)
		}
	})
	t.Run("video", func(t *testing.T) {
		m := mediaMessage(mediaVideo, up, mediaMeta{MIME: "video/mp4", Caption: "clip", Seconds: 3}).GetVideoMessage()
		if m == nil {
			t.Fatal("no VideoMessage")
		}
		check(t, copied{m.URL, m.DirectPath, m.MediaKey, m.FileSHA256, m.FileEncSHA256, m.FileLength})
		if !bytes.Equal(m.StreamingSidecar, up.StreamingSidecar) || m.GetCaption() != "clip" || m.GetSeconds() != 3 {
			t.Errorf("video sidecar=%v caption=%q seconds=%d", m.StreamingSidecar, m.GetCaption(), m.GetSeconds())
		}
	})
	t.Run("document", func(t *testing.T) {
		m := mediaMessage(mediaDocument, up, mediaMeta{MIME: "application/pdf", FileName: "../dir/report.pdf", Caption: "see"}).GetDocumentMessage()
		if m == nil {
			t.Fatal("no DocumentMessage")
		}
		check(t, copied{m.URL, m.DirectPath, m.MediaKey, m.FileSHA256, m.FileEncSHA256, m.FileLength})
		if m.GetFileName() != "report.pdf" || m.GetTitle() != "report.pdf" || m.GetMimetype() != "application/pdf" || m.GetCaption() != "see" {
			t.Errorf("document name=%q title=%q mimetype=%q caption=%q", m.GetFileName(), m.GetTitle(), m.GetMimetype(), m.GetCaption())
		}
	})
}

func TestVoiceNoteArgsAreOpusMonoSixteenKilohertzVoip(t *testing.T) {
	want := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-i", "in.wav",
		"-vn", "-map_metadata", "-1",
		"-c:a", "libopus", "-application", "voip", "-ac", "1", "-ar", "16000", "-b:a", "24k",
		"-f", "ogg", "out.ogg"}
	if got := voiceNoteArgs("in.wav", "out.ogg"); !slices.Equal(got, want) {
		t.Fatalf("voiceNoteArgs = %v, want %v", got, want)
	}
}

// oggPage builds a minimal Ogg page: header, one segment, payload.
func oggPage(granule int64, payload []byte) []byte {
	page := []byte("OggS\x00\x00")
	page = binary.LittleEndian.AppendUint64(page, uint64(granule))
	page = append(page, make([]byte, 12)...) // serial, sequence, checksum
	page = append(page, 1, byte(len(payload)))
	return append(page, payload...)
}

func opusHead(preSkip uint16) []byte {
	head := []byte("OpusHead\x01\x01")
	head = binary.LittleEndian.AppendUint16(head, preSkip)
	head = binary.LittleEndian.AppendUint32(head, 16000)
	return append(head, 0, 0, 0)
}

func TestOggOpusSecondsReadsTheLastGranule(t *testing.T) {
	stream := func(preSkip uint16, lastGranule int64) []byte {
		data := oggPage(0, opusHead(preSkip))
		data = append(data, oggPage(0, []byte("OpusTags"))...)
		return append(data, oggPage(lastGranule, []byte("audio"))...)
	}
	tests := []struct {
		name string
		data []byte
		want uint32
	}{
		{"three seconds", stream(312, 3*48000+312), 3},
		{"rounds up", stream(312, 2*48000+312+1), 3},
		{"less than the pre-skip", stream(312, 100), 0},
		{"page without a finished packet", stream(312, -1), 0},
		{"no OpusHead", oggPage(48000, []byte("audio")), 0},
		{"garbage", []byte("not an ogg stream at all"), 0},
		{"empty", nil, 0},
		{"truncated header", []byte("OpusHead\x01\x01\x38\x01 OggS\x00"), 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := oggOpusSeconds(tc.data); got != tc.want {
				t.Errorf("oggOpusSeconds = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestConvertToVoiceNoteWithFFmpeg(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	if encoders, _ := exec.Command(ffmpeg, "-hide_banner", "-encoders").Output(); !bytes.Contains(encoders, []byte("libopus")) {
		t.Skip("this ffmpeg has no libopus encoder")
	}
	in := filepath.Join(t.TempDir(), "in.wav")
	gen := exec.Command(ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1", in)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("building the input: %v: %s", err, out)
	}
	tmp := isolateTempDir(t)

	data, seconds, err := convertToVoiceNote(context.Background(), in)
	if err != nil {
		t.Fatalf("convertToVoiceNote: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("OggS")) {
		t.Fatalf("the result is not an Ogg stream: % x", data[:min(len(data), 8)])
	}
	head := bytes.Index(data, []byte("OpusHead"))
	if head < 0 || len(data) < head+16 {
		t.Fatal("the result has no OpusHead")
	}
	if channels := data[head+9]; channels != 1 {
		t.Errorf("OpusHead channel count = %d, want 1", channels)
	}
	if rate := binary.LittleEndian.Uint32(data[head+12 : head+16]); rate != 16000 {
		t.Errorf("OpusHead input rate = %d, want 16000", rate)
	}
	if seconds != 1 {
		t.Errorf("seconds = %d, want 1", seconds)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Errorf("convertToVoiceNote left %d temp files", len(left))
	}
}
