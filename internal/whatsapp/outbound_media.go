package whatsapp

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/media"
	"aurago/internal/telegram"

	"go.mau.fi/whatsmeow/types"
)

// liveClient is the logged-in client of this generation, or nil.
func (p *pipeline) liveClient() waClient {
	if p.client == nil {
		return nil
	}
	return p.client()
}

// uploadAndSend uploads r once and sends it to the owner through the single sender. A failed
// upload is logged with a fixed code only (uploadMedia has counted it); a failed send was
// logged and counted by the sender.
func (p *pipeline) uploadAndSend(ctx context.Context, wa waClient, owner types.JID, kind mediaKind, r io.Reader, size int64, meta mediaMeta, what string) error {
	up, err := uploadMedia(ctx, wa, r, size, kind)
	if err != nil {
		if ctx.Err() == nil {
			p.logMediaFailure(kind.String(), err)
		}
		return err
	}
	return p.sender.deliverMedia(ctx, owner, mediaMessage(kind, up, meta), what)
}

// deliverCapturedAudio sends a TTS file captured by the broker. Only files under
// <data_dir>/tts are accepted, opened through os.OpenRoot so no link leads out (deviation 21).
// In voice mode it becomes a voice note (voiceNote); a failed conversion falls back to a
// normal audio message, like Telegram (bot.go:674-688).
func (p *pipeline) deliverCapturedAudio(ctx context.Context, cfg *config.Config, owner types.JID, af telegram.CapturedAudio) {
	wa := p.liveClient()
	if wa == nil || ctx.Err() != nil {
		return
	}
	f, name, size, reason := openTTSFile(cfg, af.FilePath)
	if f == nil {
		p.log().Warn("WhatsApp audio skipped", "reason", reason)
		return
	}
	defer f.Close()

	if agent.GetVoiceMode() {
		data, seconds, err := p.convertVoice(ctx, f, size)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			_ = p.uploadAndSend(ctx, wa, owner, mediaVoiceNote, bytes.NewReader(data), int64(len(data)), mediaMeta{Seconds: seconds}, "voice note")
			return
		}
		p.log().Warn("WhatsApp voice conversion failed, sending audio instead", "code", mediaErrorCode(err))
	}

	head := make([]byte, 512)
	n, _ := f.ReadAt(head, 0)
	kind, mimeType := mediaKindFor(name, head[:n])
	_ = p.uploadAndSend(ctx, wa, owner, kind, io.NewSectionReader(f, 0, size), size, mediaMeta{MIME: mimeType, FileName: name}, "audio")
}

// openTTSFile opens a TTS file that lies inside <data_dir>/tts. The open goes through an
// os.Root bound to that folder, so a link that leads out is refused and no path is opened
// twice. On a refusal the file is nil and reason is a fixed code. The caller closes the file.
func openTTSFile(cfg *config.Config, filePath string) (f *os.File, name string, size int64, reason string) {
	dataDir := strings.TrimSpace(cfg.Directories.DataDir)
	if dataDir == "" {
		return nil, "", 0, "outside_tts_dir"
	}
	ttsDir, err := filepath.Abs(filepath.Join(dataDir, "tts"))
	if err != nil {
		return nil, "", 0, "outside_tts_dir"
	}
	target, err := filepath.Abs(filePath)
	if err != nil {
		return nil, "", 0, "outside_tts_dir"
	}
	rel, err := filepath.Rel(ttsDir, target)
	if err != nil || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", 0, "outside_tts_dir"
	}
	root, err := os.OpenRoot(ttsDir)
	if err != nil {
		return nil, "", 0, "tts_dir_unavailable"
	}
	defer root.Close()
	file, err := root.Open(rel)
	if err != nil {
		return nil, "", 0, "unreadable" // also a link that leads out of the folder
	}
	info, err := file.Stat()
	switch {
	case err != nil || !info.Mode().IsRegular():
		_ = file.Close()
		return nil, "", 0, "not_a_file"
	case info.Size() > maxMediaBytes:
		_ = file.Close()
		return nil, "", 0, "too_large"
	}
	return file, filepath.Base(rel), info.Size(), ""
}

// convertVoice turns the confined file into a voice note. ffmpeg reads a private copy of the
// open handle, never the path again, so a link swapped in after the confinement check cannot
// redirect it.
func (p *pipeline) convertVoice(ctx context.Context, f *os.File, size int64) ([]byte, uint32, error) {
	voiceNote := p.voiceNote
	if voiceNote == nil {
		voiceNote = convertToVoiceNote
	}
	tmp, err := os.CreateTemp("", "aurago-whatsapp-tts-*")
	if err != nil {
		return nil, 0, err
	}
	defer os.Remove(tmp.Name())
	_, copyErr := io.Copy(tmp, io.NewSectionReader(f, 0, size))
	if err := tmp.Close(); err != nil || copyErr != nil {
		if copyErr == nil {
			copyErr = err
		}
		return nil, 0, copyErr
	}
	return voiceNote(ctx, tmp.Name())
}

// deliverAnswerImages sends each Markdown image as an image message with its alt text as
// caption (sender.caption, cut=true). /files/ URLs go through media.StageWorkspaceImage (the
// staged copy is removed afterwards); http(s) URLs through media.DownloadAndSanitizeImageContext
// into <workspace>/images; anything else is not fetched (bot.go:699-730). A content that is not
// an image goes out as a document. Images that could not be sent come back as appendImageLines
// lines, with the alt text only.
func (p *pipeline) deliverAnswerImages(ctx context.Context, cfg *config.Config, owner types.JID, text string, images []media.ImageRef) string {
	wa := p.liveClient()
	var failed []media.ImageRef
	for _, img := range images {
		if ctx.Err() != nil {
			return text
		}
		if wa == nil || !p.sendAnswerImage(ctx, cfg, wa, owner, img) {
			failed = append(failed, img)
		}
	}
	return appendImageLines(text, failed)
}

// sendAnswerImage stages, opens and sends one answer image. It reports whether it went out.
// Logs carry the kind and a fixed code, never the URL or the path.
func (p *pipeline) sendAnswerImage(ctx context.Context, cfg *config.Config, wa waClient, owner types.JID, img media.ImageRef) bool {
	var localPath, name string
	switch {
	case strings.HasPrefix(img.URL, "/files/"):
		staged, err := media.StageWorkspaceImage(cfg.Directories.WorkspaceDir, img.URL)
		if err != nil {
			p.logMediaFailure("image", err)
			return false
		}
		defer os.Remove(staged)
		localPath = staged
		if parsed, err := url.Parse(img.URL); err == nil {
			name = path.Base(parsed.Path)
		}
	case strings.HasPrefix(img.URL, "http://"), strings.HasPrefix(img.URL, "https://"):
		sanitized, err := media.DownloadAndSanitizeImageContext(ctx, img.URL, filepath.Join(cfg.Directories.WorkspaceDir, "images"))
		if err != nil {
			if ctx.Err() == nil {
				p.logMediaFailure("image", err)
			}
			return false
		}
		localPath = sanitized
	default:
		p.log().Warn("WhatsApp answer image skipped", "reason", "unsupported_url")
		return false
	}
	if name == "" {
		name = filepath.Base(localPath)
	}

	file, err := os.Open(localPath)
	if err != nil {
		p.logMediaFailure("image", err)
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		p.logMediaFailure("image", err)
		return false
	}
	if info.Size() > maxMediaBytes {
		p.logMediaFailure("image", ErrMediaTooLarge)
		return false
	}
	head := make([]byte, 512)
	n, _ := file.ReadAt(head, 0)
	head = head[:n]
	kind, mimeType := mediaKindFor(name, head)
	if kind != mediaImage { // not a JPEG or PNG (a GIF, a WebP): a document keeps it intact
		kind, mimeType = mediaDocument, documentMIME(strings.ToLower(filepath.Ext(name)), http.DetectContentType(head))
	}
	caption, _ := p.sender.caption(img.Caption, true)
	err = p.uploadAndSend(ctx, wa, owner, kind, io.NewSectionReader(file, 0, info.Size()), info.Size(),
		mediaMeta{MIME: mimeType, FileName: name, Caption: caption}, "answer image")
	return err == nil
}
