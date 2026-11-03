package whatsapp

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/gif" // decoders for image.DecodeConfig in sendAnswerImage
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/media"
	"aurago/internal/telegram"
	"aurago/internal/tools"

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

const (
	// maxAnswerImages is how many Markdown images of one reply are fetched. The rest become
	// "[Image: alt]" lines without a fetch: the model decides the number, and each one is a file
	// open or an HTTP download.
	maxAnswerImages = 5
	// maxAnswerImagePixels is the decode limit media.StageWorkspaceImage applied to workspace
	// images.
	maxAnswerImagePixels = 40_000_000
)

// errNotAnImage means a /files/ file did not decode as an image within the pixel limit.
var errNotAnImage = errors.New("the file is not a supported image")

// deliverAnswerImages sends each Markdown image as an image message with its alt text as
// caption (sender.caption, cut=true), the first maxAnswerImages of a reply. /files/ URLs name a
// workspace file, opened through tools.OpenOutgoingAttachment like every file that leaves AuraGo;
// http(s) URLs go through the pipeline's fetchImage (media.DownloadAndSanitizeImageContext) into
// <workspace>/images, and the fetched copy is removed after the send; anything else is not
// fetched (bot.go:699-730). A content that is not a JPEG or PNG goes out as a document. Images
// that were not sent come back as appendImageLines lines, with the alt text only.
func (p *pipeline) deliverAnswerImages(ctx context.Context, cfg *config.Config, owner types.JID, text string, images []media.ImageRef) string {
	wa := p.liveClient()
	var failed []media.ImageRef
	for i, img := range images {
		if ctx.Err() != nil {
			return text
		}
		if i >= maxAnswerImages {
			failed = append(failed, img) // over the cap: never fetched
			continue
		}
		if wa == nil || !p.sendAnswerImage(ctx, cfg, wa, owner, img) {
			failed = append(failed, img)
		}
	}
	if len(images) > maxAnswerImages {
		p.log().Warn("WhatsApp answer images over the cap were not fetched", "skipped", len(images)-maxAnswerImages)
	}
	return appendImageLines(text, failed)
}

// openWorkspaceImage opens the workspace file a /files/ URL names. The URL is checked like
// media.StageWorkspaceImage checks it, and the file opens through tools.OpenOutgoingAttachment,
// which keeps out roots that hold data_dir, protected names (.env, vault, master key, config,
// device store) and hard links to them. The caller closes the file.
func openWorkspaceImage(cfg *config.Config, rawURL string) (*os.File, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/files/") {
		return nil, "", errors.New("invalid workspace image URL")
	}
	rel := strings.TrimPrefix(parsed.Path, "/files/")
	if !fs.ValidPath(rel) || strings.ContainsAny(rel, "\\:\x00") {
		return nil, "", errors.New("invalid workspace image path")
	}
	workspace := strings.TrimSpace(cfg.Directories.WorkspaceDir)
	if workspace == "" {
		return nil, "", errors.New("no workspace")
	}
	abs, err := filepath.Abs(filepath.Join(workspace, filepath.FromSlash(rel)))
	if err != nil {
		return nil, "", err
	}
	return tools.OpenOutgoingAttachment(abs, cfg)
}

// sendAnswerImage opens and sends one answer image. It reports whether it went out. Logs carry
// the kind and a fixed code, never the URL or the path.
func (p *pipeline) sendAnswerImage(ctx context.Context, cfg *config.Config, wa waClient, owner types.JID, img media.ImageRef) bool {
	var file *os.File
	var name string
	switch {
	case strings.HasPrefix(img.URL, "/files/"):
		f, resolved, err := openWorkspaceImage(cfg, img.URL)
		if err != nil {
			p.logMediaFailure("image", err)
			return false
		}
		file, name = f, filepath.Base(resolved)
	case strings.HasPrefix(img.URL, "http://"), strings.HasPrefix(img.URL, "https://"):
		fetch := p.fetchImage
		if fetch == nil {
			fetch = media.DownloadAndSanitizeImageContext
		}
		fetched, err := fetch(ctx, img.URL, filepath.Join(cfg.Directories.WorkspaceDir, "images"))
		if err != nil {
			if ctx.Err() == nil {
				p.logMediaFailure("image", err)
			}
			return false
		}
		defer os.Remove(fetched) // after the Close below: the copy was made for this send only
		f, err := os.Open(fetched)
		if err != nil {
			p.logMediaFailure("image", err)
			return false
		}
		file, name = f, filepath.Base(fetched)
	default:
		p.log().Warn("WhatsApp answer image skipped", "reason", "unsupported_url")
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
	if strings.HasPrefix(img.URL, "/files/") {
		// A workspace file is whatever the agent or the user put there: it must decode as an
		// image of sane size. A fetched copy has been decoded and re-encoded already.
		conf, _, err := image.DecodeConfig(io.NewSectionReader(file, 0, info.Size()))
		if err != nil || conf.Width <= 0 || conf.Height <= 0 || int64(conf.Width)*int64(conf.Height) > maxAnswerImagePixels {
			p.logMediaFailure("image", errNotAnImage)
			return false
		}
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
