package whatsapp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// OwnerAttachment is a file AuraGo made itself (a camera photo, a newspaper PDF). It never
// comes from an agent path, so it skips OpenOutgoingAttachment.
type OwnerAttachment struct {
	Name string
	Data []byte
}

// ownerFile is what sendOwner uploads: a name, a size and random access to the bytes.
type ownerFile struct {
	name string
	size int64
	r    io.ReaderAt
}

// errMessageRequired is returned for an owner send that has neither text nor a file.
var errMessageRequired = errors.New("message is required")

// OwnerSendReady reports that this generation is running and holds a connection to send over.
// Readonly is not part of it: every send checks the live configuration itself.
func (b *Bot) OwnerSendReady() bool {
	return b != nil && b.ctx.Err() == nil && b.currentClient() != nil
}

// ownerSendAllowed is the gate every agent- or system-initiated send passes. It returns the
// live configuration, which OpenOutgoingAttachment needs for the current workspace and
// document roots. The read-only switch is read from that live configuration on every call, so a
// toggle takes effect before the generation is replaced.
func (b *Bot) ownerSendAllowed() (*config.Config, error) {
	if b == nil || b.ctx.Err() != nil || b.pipeline == nil {
		return nil, ErrNotConnected
	}
	cfg := b.pipeline.base
	if b.deps.agent.Snapshot != nil {
		if live, _ := b.deps.agent.Snapshot(); live != nil {
			cfg = live
		}
	}
	switch {
	case cfg == nil || cfg.EggMode.Enabled:
		return nil, ErrNotConnected
	case cfg.WhatsApp.ReadOnly:
		return nil, ErrReadOnly
	case !b.initial.Equal(cfg.WhatsApp):
		return nil, ErrNotConnected // a newer generation is taking over
	}
	return cfg, nil
}

// safeOwnerError keeps errors.Is but masks digits and scrubs secrets in Error(): whatsmeow's
// ErrRecipientNotOnWhatsApp names the owner's JID, and tool results reach the model and logs.
type safeOwnerError struct{ err error }

func (e safeOwnerError) Error() string { return maskedError(e.err) }
func (e safeOwnerError) Unwrap() error { return e.err }

// maskOwnerError wraps err for the caller of the owner API; nil stays nil.
func maskOwnerError(err error) error {
	if err == nil {
		return nil
	}
	return safeOwnerError{err}
}

// SendOwnerMessage sends text and/or the workspace or documents file at filePath to every owner
// recipient and to nobody else. A file is opened only through tools.OpenOutgoingAttachment, so
// configuration, vault and databases cannot leave. Text that does not fit as a caption (and the
// text of an audio file) goes first as a message of its own, like SendTelegramFile
// (internal/tools/telegram_document.go:91-138): the file is opened and checked before anything
// is sent, so a refused file sends no text either. The error is masked (safeOwnerError).
func (b *Bot) SendOwnerMessage(ctx context.Context, text, filePath string) (tools.WhatsAppSendResult, error) {
	res, err := b.sendOwnerMessage(ctx, text, filePath)
	return res, maskOwnerError(err)
}

func (b *Bot) sendOwnerMessage(ctx context.Context, text, filePath string) (tools.WhatsAppSendResult, error) {
	cfg, err := b.ownerSendAllowed()
	if err != nil {
		return tools.WhatsAppSendResult{}, err
	}
	if strings.TrimSpace(text) == "" && strings.TrimSpace(filePath) == "" {
		return tools.WhatsAppSendResult{}, errMessageRequired
	}
	if strings.TrimSpace(filePath) == "" {
		return b.sendOwner(ctx, text, nil)
	}
	f, resolved, err := tools.OpenOutgoingAttachment(filePath, cfg)
	if err != nil {
		return tools.WhatsAppSendResult{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	switch {
	case err != nil || !info.Mode().IsRegular():
		return tools.WhatsAppSendResult{}, errors.New("the attachment is not a regular file")
	case info.Size() > maxMediaBytes:
		return tools.WhatsAppSendResult{}, ErrMediaTooLarge
	case info.Size() == 0:
		return tools.WhatsAppSendResult{}, errors.New("the attachment is empty")
	}
	return b.sendOwner(ctx, text, &ownerFile{name: filepath.Base(resolved), size: info.Size(), r: f})
}

// SendOwnerAttachment sends a file AuraGo made itself, with a caption, to every owner recipient.
// It passes the same read-only gate as SendOwnerMessage, and its bytes never come from an agent
// path. The caption follows the same rules as the text of SendOwnerMessage. The error is masked.
func (b *Bot) SendOwnerAttachment(ctx context.Context, caption string, att OwnerAttachment) (tools.WhatsAppSendResult, error) {
	res, err := b.sendOwnerAttachment(ctx, caption, att)
	return res, maskOwnerError(err)
}

func (b *Bot) sendOwnerAttachment(ctx context.Context, caption string, att OwnerAttachment) (tools.WhatsAppSendResult, error) {
	if _, err := b.ownerSendAllowed(); err != nil {
		return tools.WhatsAppSendResult{}, err
	}
	switch {
	case len(att.Data) == 0:
		return tools.WhatsAppSendResult{}, errors.New("the attachment is empty")
	case int64(len(att.Data)) > maxMediaBytes:
		return tools.WhatsAppSendResult{}, ErrMediaTooLarge
	}
	return b.sendOwner(ctx, caption, &ownerFile{name: documentName(att.Name), size: int64(len(att.Data)), r: bytes.NewReader(att.Data)})
}

// sendOwner is the shared part of both entry points; the caller has passed ownerSendAllowed.
// Without a file the text goes to every owner. With one, the file is uploaded once and then sent
// to each owner, preceded by the text when it does not fit as a caption or when the file is audio
// (audio has no caption). A failure for one owner does not stop the others; the first error
// is returned.
func (b *Bot) sendOwner(ctx context.Context, text string, file *ownerFile) (tools.WhatsAppSendResult, error) {
	client := b.currentClient()
	if client == nil || !client.IsLoggedIn() {
		return tools.WhatsAppSendResult{}, ErrNotConnected
	}
	if file == nil {
		err := b.sender.deliverToOwners(ctx, text, "owner message")
		return tools.WhatsAppSendResult{Kind: "text", TextSent: err == nil}, err
	}

	head := make([]byte, 512)
	n, _ := file.r.ReadAt(head, 0)
	kind, mimeType := mediaKindFor(file.name, head[:n])
	res := tools.WhatsAppSendResult{Kind: kind.String(), File: file.name}
	owners, err := ownerRecipients(b.initial, client.OwnJID())
	if err != nil {
		return res, err
	}
	up, err := uploadMedia(ctx, client, io.NewSectionReader(file.r, 0, file.size), file.size, kind)
	if err != nil {
		return res, err
	}

	hasText := strings.TrimSpace(text) != ""
	caption, fits := b.sender.caption(text, false)
	textFirst := hasText && (kind == mediaAudio || !fits)
	meta := mediaMeta{MIME: mimeType, FileName: file.name, Caption: caption}
	if textFirst {
		meta.Caption = ""
	}
	var first error
	for _, to := range owners {
		if textFirst {
			if err := b.sender.deliverText(ctx, to, text, "owner message"); err != nil {
				if first == nil {
					first = err
				}
				continue // this owner gets no file without its text
			}
			res.TextSent = true
		}
		if err := b.sender.deliverMedia(ctx, to, mediaMessage(kind, up, meta), "owner attachment"); err != nil && first == nil {
			first = err
		}
	}
	return res, first
}
