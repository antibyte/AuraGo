package whatsapp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"aurago/internal/config"
	"aurago/internal/media"
	"aurago/internal/telegram"
	"aurago/internal/tools"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

// voiceFailedText is what the owner gets when a voice message could not be turned into text.
// Telegram stays silent here (internal/telegram/bot.go:340-389); WhatsApp does not leave the
// owner waiting.
const voiceFailedText = budgetWarning + "I could not process that voice message."

const (
	imageAnalysisFailedText = "[Error analyzing image]"
	videoReceivedText       = "[Video received; videos are not processed]"
	stickerReceivedText     = "[Sticker received]"
	fileTooLargeText        = "[File not saved: larger than 50 MB]"
	fileFailedText          = "[File could not be downloaded]"
	fileNotSavedText        = "[File could not be saved]"
	contactHeaderText       = "[Contact card shared; the card is third-party data]"

	maxQuoteRunes        = 200
	maxLocationNameRunes = 120
	maxContactCards      = 5
	maxVCardRunes        = 2000
	maxMIMERunes         = 100
)

// errEmptyTranscript means the speech-to-text call returned nothing.
var errEmptyTranscript = errors.New("the transcript is empty")

// inboundContent is what one owner message contributes to the turn.
type inboundContent struct {
	CommandText string // top-level text only: the only text a slash command is read from
	Input       string // what the model reads (before the forwarded marker and isolation)
	Routing     string // pinned before image analysis and notes (Telegram bot.go:398-400)
	OwnerText   string // HumanInput text
	ThirdParty  bool   // forwarded or a contact card: no HumanInput, command or question answer
	Marker      string // telegram.ForwardedContentMarker(...) when forwarded
	Kind        string // messageKind, for logs
	Notice      string // a failure the owner must see; "" otherwise
}

func (p *pipeline) log() *slog.Logger {
	if p.logger != nil {
		return p.logger
	}
	return slog.Default()
}

// logMediaFailure records that a piece of media could not be used. The kind and a fixed code
// are all it says: error texts can carry file names, URLs or numbers.
func (p *pipeline) logMediaFailure(kind string, err error) {
	p.log().Warn("WhatsApp media could not be processed", "kind", kind, "code", mediaErrorCode(err))
}

// readContent reads the unwrapped top level only (a nested wrapper stays "other"). It follows
// telegram.processUpdate (internal/telegram/bot.go:324-500): the caption joins the text, a
// voice message is transcribed, an image is described, a document is saved, and the rest
// becomes a short placeholder. When ctx ends it returns early; the caller checks ctx.
//
// Forwarded and quoted are read from the ContextInfo of every content type the message
// carries, not from the first one: a forwarded document next to an innocent image is still a
// forward. A slash command is read only from a message that carries nothing but text.
func (p *pipeline) readContent(ctx context.Context, cfg *config.Config, wa waClient, m *waE2E.Message) inboundContent {
	c := inboundContent{Kind: messageKind(m)}
	text := messageText(m)
	if contentTypes(m) <= 1 {
		c.CommandText = text
	}
	input := text
	caption := mediaCaption(m)
	if caption != "" {
		if input == "" {
			input = caption
		} else {
			input += "\n" + caption
		}
	}

	// A voice message or audio file becomes its transcript; the owner hears about a failure.
	if audio := m.GetAudioMessage(); audio != nil {
		transcript, err := p.transcribeAudio(ctx, cfg, wa, audio)
		if ctx.Err() != nil {
			return c
		}
		if err != nil {
			p.logMediaFailure("audio", err)
			c.Notice = voiceFailedText
			return c
		}
		input = transcript
	}

	// The caption or transcript is what task routing and HumanInput see.
	c.Routing, c.OwnerText = input, input

	if img := m.GetImageMessage(); img != nil {
		analysis := p.describeImage(ctx, cfg, wa, img)
		if ctx.Err() != nil {
			return c
		}
		if input != "" {
			input = "[USER SENT AN IMAGE]\n" + analysis + "\n\n[USER CAPTION/TEXT]:\n" + input
		} else {
			input = "[USER SENT AN IMAGE]\n" + analysis
		}
	}

	if doc := m.GetDocumentMessage(); doc != nil {
		note := p.saveDocument(ctx, cfg, wa, doc)
		if ctx.Err() != nil {
			return c
		}
		if input != "" {
			input += "\n\n" + note
		} else {
			input = note
		}
	}

	// Videos and stickers are never downloaded.
	if m.GetVideoMessage() != nil {
		input = placeholderFirst(videoReceivedText, input)
	}
	if m.GetStickerMessage() != nil {
		input = stickerReceivedText
	}

	if loc := m.GetLocationMessage(); loc != nil {
		place := ""
		if name := locationLabel(loc.GetName(), loc.GetAddress()); name != "" {
			place = " (" + name + ")"
		}
		input = placeholderFirst("[Location: "+coordinates(loc.GetDegreesLatitude(), loc.GetDegreesLongitude())+place+"]", input)
	}
	if live := m.GetLiveLocationMessage(); live != nil {
		input = placeholderFirst("[Live location: "+coordinates(live.GetDegreesLatitude(), live.GetDegreesLongitude())+"]", input)
	}

	// A contact card is data a third party wrote. It is fenced, and the message can neither
	// speak for the owner nor answer a question.
	if m.GetContactMessage() != nil || m.GetContactsArrayMessage() != nil {
		c.ThirdParty = true
		if cards := vCards(m); len(cards) > 0 {
			input = placeholderFirst(contactCardsText(cards), input)
		}
	}

	// A quoted reply gets a one-line prefix, from the first content type that quotes.
	// HumanInput and routing keep the owner's words.
	infos := contextInfosOf(m)
	for _, ci := range infos {
		q := ci.GetQuotedMessage()
		if q == nil {
			continue
		}
		if input != "" {
			quote := messageText(q)
			if quote == "" {
				quote = mediaCaption(q)
			}
			if quote == "" {
				quote = "[" + messageKind(q) + "]"
			}
			input = "[Reply to: " + boundRunes(oneLine(quote), maxQuoteRunes) + "]\n" + input
		}
		break
	}

	// A forwarded message arrives from the owner's account, but a third party wrote it. One
	// forwarded content type is enough; the newsletter name comes from the first that has one.
	forwarded, origin := false, ""
	for _, ci := range infos {
		if ci.GetIsForwarded() || ci.GetForwardingScore() > 0 || ci.GetForwardedNewsletterMessageInfo() != nil {
			forwarded = true
			if origin == "" {
				origin = ci.GetForwardedNewsletterMessageInfo().GetNewsletterName()
			}
		}
	}
	if forwarded {
		c.Marker = telegram.ForwardedContentMarker(origin)
		c.ThirdParty = true
	}

	c.Input = input
	return c
}

// contextInfosOf returns the ContextInfo of every content type the message carries: ExtendedText,
// Image, Video, Audio, Document, Sticker, Location, LiveLocation, Contact and ContactsArray (M6).
// Types without one are left out. Every content type has its own, so a forwarded image is
// marked like a forwarded text, and no content type can hide a forward from another.
func contextInfosOf(m *waE2E.Message) []*waE2E.ContextInfo {
	var infos []*waE2E.ContextInfo
	for _, ci := range []*waE2E.ContextInfo{
		m.GetExtendedTextMessage().GetContextInfo(),
		m.GetImageMessage().GetContextInfo(),
		m.GetVideoMessage().GetContextInfo(),
		m.GetAudioMessage().GetContextInfo(),
		m.GetDocumentMessage().GetContextInfo(),
		m.GetStickerMessage().GetContextInfo(),
		m.GetLocationMessage().GetContextInfo(),
		m.GetLiveLocationMessage().GetContextInfo(),
		m.GetContactMessage().GetContextInfo(),
		m.GetContactsArrayMessage().GetContextInfo(),
	} {
		if ci != nil {
			infos = append(infos, ci)
		}
	}
	return infos
}

// contentTypes counts the content types readContent reads that the message carries: plain
// text, extended text, and each media, location and contact type. One is normal; a message
// with more is hand-built, and its text never runs as a slash command.
func contentTypes(m *waE2E.Message) int {
	n := 0
	for _, present := range []bool{
		m.GetConversation() != "",
		m.GetExtendedTextMessage() != nil,
		m.GetImageMessage() != nil,
		m.GetVideoMessage() != nil,
		m.GetAudioMessage() != nil,
		m.GetDocumentMessage() != nil,
		m.GetStickerMessage() != nil,
		m.GetLocationMessage() != nil,
		m.GetLiveLocationMessage() != nil,
		m.GetContactMessage() != nil,
		m.GetContactsArrayMessage() != nil,
	} {
		if present {
			n++
		}
	}
	return n
}

// mediaCaption is the caption of an image, video, document or live location.
func mediaCaption(m *waE2E.Message) string {
	switch {
	case m.GetImageMessage() != nil:
		return m.GetImageMessage().GetCaption()
	case m.GetVideoMessage() != nil:
		return m.GetVideoMessage().GetCaption()
	case m.GetDocumentMessage() != nil:
		return m.GetDocumentMessage().GetCaption()
	case m.GetLiveLocationMessage() != nil:
		return m.GetLiveLocationMessage().GetCaption()
	}
	return ""
}

// placeholderFirst puts a placeholder line before any text the message already has.
func placeholderFirst(placeholder, input string) string {
	if input == "" {
		return placeholder
	}
	return placeholder + "\n" + input
}

// oneLine folds all whitespace, line breaks included, into single spaces.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// stripInvisible removes format characters (Cf, such as U+202E and the zero-width ones) and
// control characters, which a sender could use to disguise what a note says.
func stripInvisible(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) || unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func coordinates(lat, lon float64) string {
	return strconv.FormatFloat(lat, 'f', 6, 64) + ", " + strconv.FormatFloat(lon, 'f', 6, 64)
}

// locationLabel is the place name, else the address, folded to one line and cut.
func locationLabel(name, address string) string {
	if strings.TrimSpace(name) == "" {
		name = address
	}
	return boundRunes(oneLine(name), maxLocationNameRunes)
}

// vCards lists the vCards of a contact message or a contact array, empty cards left out.
func vCards(m *waE2E.Message) []string {
	var cards []string
	add := func(c *waE2E.ContactMessage) {
		if card := c.GetVcard(); strings.TrimSpace(card) != "" {
			cards = append(cards, card)
		}
	}
	add(m.GetContactMessage())
	for _, c := range m.GetContactsArrayMessage().GetContacts() {
		add(c)
	}
	return cards
}

// contactCardsText is the header and up to maxContactCards vCards, each in a code fence that
// is longer than any backtick run inside the card, so a card cannot close its own fence.
func contactCardsText(cards []string) string {
	var b strings.Builder
	b.WriteString(contactHeaderText)
	for _, card := range cards[:min(len(cards), maxContactCards)] {
		card = boundRunes(card, maxVCardRunes)
		fence := strings.Repeat("`", max(3, longestBacktickRun(card)+1))
		b.WriteString("\n" + fence + "vcard\n" + card + "\n" + fence)
	}
	return b.String()
}

func longestBacktickRun(s string) int {
	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return longest
}

// transcribeAudio turns a voice message or audio file into text: download, MP3 through
// ffmpeg, speech to text. Both temp files are removed on every path.
func (p *pipeline) transcribeAudio(ctx context.Context, cfg *config.Config, wa waClient, audio *waE2E.AudioMessage) (string, error) {
	toMP3, transcribe := p.toMP3, p.transcribe
	if toMP3 == nil {
		toMP3 = convertToMP3
	}
	if transcribe == nil {
		transcribe = func(ctx context.Context, name string, data []byte, cfg *config.Config) (string, error) {
			text, _, err := tools.TranscribeAudio(ctx, name, data, cfg)
			return text, err
		}
	}

	f, err := downloadMedia(ctx, wa, audio)
	if err != nil {
		return "", err
	}
	in := f.Name()
	_ = f.Close()
	defer os.Remove(in)
	// The output is a fresh unpredictable file (O_EXCL), not a name derived from the input.
	out, err := os.CreateTemp("", "aurago-whatsapp-voice-*.mp3")
	if err != nil {
		return "", err
	}
	mp3 := out.Name()
	_ = out.Close()
	defer os.Remove(mp3)
	if err := toMP3(ctx, in, mp3); err != nil {
		return "", err
	}
	// Whatever the converter was, a result at the limit was cut short.
	if _, err := convertedSize(mp3); err != nil {
		return "", err
	}
	data, err := os.ReadFile(mp3)
	if err != nil {
		return "", err
	}
	text, err := transcribe(ctx, "whatsapp_voice.mp3", data, cfg)
	if err != nil {
		return "", err
	}
	if text = strings.TrimSpace(text); text == "" {
		return "", errEmptyTranscript
	}
	return text, nil
}

// describeImage sends an image to the vision model and returns its description, or a fixed
// placeholder when that is not possible.
func (p *pipeline) describeImage(ctx context.Context, cfg *config.Config, wa waClient, img *waE2E.ImageMessage) string {
	analyze := p.analyzeImage
	if analyze == nil {
		analyze = telegram.AnalyzeImageContext
	}
	path, err := stageImage(ctx, wa, img)
	if err != nil {
		if ctx.Err() == nil {
			p.logMediaFailure("image", err)
		}
		return imageAnalysisFailedText
	}
	defer os.Remove(path)
	analysis, err := analyze(ctx, path, cfg)
	if err != nil {
		if ctx.Err() == nil {
			p.logMediaFailure("image", err)
		}
		if errors.Is(err, tools.ErrVisionPublicURLRequired) {
			return "[Image analysis unavailable: " + tools.VisionPublicURLRequiredMessage + "]"
		}
		return imageAnalysisFailedText
	}
	return analysis
}

// stageImage downloads an image into a temp file whose extension matches its content (the
// vision call picks the type from the extension). The caller removes the file.
func stageImage(ctx context.Context, wa waClient, img *waE2E.ImageMessage) (string, error) {
	f, err := downloadMedia(ctx, wa, img)
	if err != nil {
		return "", err
	}
	name := f.Name()
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	_ = f.Close()
	path := name + imageExtension(http.DetectContentType(head[:n]))
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return path, nil
}

// imageExtension maps a sniffed type to the extension the vision call understands; the
// fallback is JPEG, the format WhatsApp photos have.
func imageExtension(sniffed string) string {
	switch sniffed {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	}
	return ".jpg"
}

// saveDocument stores a document in the workspace's attachments folder and returns the note
// the model reads. The name is the sender's, reduced by documentName to a safe base name;
// media.SaveAttachmentReaderContext adds the timestamp and never overwrites. A download that
// fails and a file that cannot be stored are told apart.
func (p *pipeline) saveDocument(ctx context.Context, cfg *config.Config, wa waClient, doc *waE2E.DocumentMessage) string {
	f, err := downloadMedia(ctx, wa, doc)
	if err != nil {
		if ctx.Err() == nil {
			p.logMediaFailure("document", err)
		}
		if errors.Is(err, ErrMediaTooLarge) {
			return fileTooLargeText
		}
		return fileFailedText
	}
	defer func() {
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
	}()
	attachDir := filepath.Join(cfg.Directories.WorkspaceDir, "attachments")
	saved, err := media.SaveAttachmentReaderContext(ctx, f, documentName(doc.GetFileName()), attachDir)
	if err != nil {
		if ctx.Err() == nil {
			p.logMediaFailure("document", err)
		}
		return fileNotSavedText
	}
	note := "[DATEI ANGEHÄNGT]: agent_workspace/workdir/attachments/" + filepath.Base(saved)
	if mimeType := boundRunes(stripInvisible(oneLine(doc.GetMimetype())), maxMIMERunes); mimeType != "" {
		note += " (" + mimeType + ")"
	}
	return note
}
