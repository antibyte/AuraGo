package mailer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
)

func (s *Service) GetMessage(ctx context.Context, id string) (Message, error) {
	m, err := s.store.GetMessage(ctx, id)
	if err != nil {
		return Message{}, err
	}
	adapter, account, err := s.resolve(ctx, m.AccountID)
	if err != nil {
		return Message{}, err
	}
	if account.Disabled {
		return Message{}, ErrDisabled
	}
	if !hasCapability(account.Capabilities, "read") {
		return Message{}, ErrCapability
	}
	if m.Text != "" || m.HTML != "" || len(m.Raw) > 0 {
		return m, nil
	}
	if !s.runtimePolicy().Enabled {
		return m, nil
	}
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return Message{}, err
	}
	if !settings.Enabled {
		return m, nil
	}
	hydrated, err := adapter.Read(ctx, m.FolderID, m.RemoteID)
	if err != nil {
		return Message{}, err
	}
	mergeHydratedMessage(&hydrated, m)
	if err := ctx.Err(); err != nil {
		return Message{}, err
	}
	if err := s.store.UpdateMessage(ctx, hydrated); err != nil {
		return Message{}, err
	}
	return hydrated, nil
}

func mergeHydratedMessage(hydrated *Message, indexed Message) {
	hydrated.ID = indexed.ID
	hydrated.AccountID = indexed.AccountID
	hydrated.FolderID = indexed.FolderID
	hydrated.RemoteID = indexed.RemoteID
	hydrated.ThreadID = indexed.ThreadID
	if hydrated.Date.IsZero() {
		hydrated.Date = indexed.Date
	}
	if hydrated.Subject == "" {
		hydrated.Subject = indexed.Subject
	}
	if hydrated.From == "" {
		hydrated.From = indexed.From
	}
	if hydrated.ReplyTo == nil {
		hydrated.ReplyTo = indexed.ReplyTo
	}
	if hydrated.To == nil {
		hydrated.To = indexed.To
	}
	if hydrated.CC == nil {
		hydrated.CC = indexed.CC
	}
	if hydrated.BCC == nil {
		hydrated.BCC = indexed.BCC
	}
	if hydrated.MessageID == "" {
		hydrated.MessageID = indexed.MessageID
	}
	if hydrated.InReplyTo == "" {
		hydrated.InReplyTo = indexed.InReplyTo
	}
	if hydrated.References == nil {
		hydrated.References = indexed.References
	}
	if hydrated.Attachments == nil {
		hydrated.Attachments = indexed.Attachments
	}
	if len(hydrated.Raw) == 0 {
		hydrated.Raw = indexed.Raw
	}
	hydrated.Unread = indexed.Unread
	hydrated.Starred = indexed.Starred
	hydrated.Labels = indexed.Labels
	hydrated.SnoozedUntil = indexed.SnoozedUntil
}

func (s *Service) GetThread(ctx context.Context, id string) (Thread, error) {
	messages, err := s.store.GetThread(ctx, id)
	if err != nil {
		return Thread{}, err
	}
	thread := buildThread(messages)
	for i := range thread.Messages {
		message, err := s.GetMessage(ctx, thread.Messages[i].ID)
		if err != nil {
			kept := thread.Messages[i]
			kept.ReadError = "message_unavailable"
			thread.Messages[i] = kept
			continue
		}
		thread.Messages[i] = message
	}
	return buildThread(thread.Messages), nil
}

func (s *Service) RawMessage(ctx context.Context, id string) ([]byte, error) {
	m, err := s.store.GetMessage(ctx, id)
	if err != nil {
		return nil, err
	}
	adapter, account, err := s.resolve(ctx, m.AccountID)
	if err != nil {
		return nil, err
	}
	if account.Disabled {
		return nil, ErrDisabled
	}
	if !hasCapability(account.Capabilities, "read") {
		return nil, ErrCapability
	}
	if !hasCapability(account.Capabilities, "read") {
		return nil, ErrCapability
	}
	limit := s.blobs.maxFile * 2
	if len(m.Raw) > 0 {
		if int64(len(m.Raw)) > limit {
			return nil, errors.New("raw message exceeds configured size limit")
		}
		return append([]byte(nil), m.Raw...), nil
	}
	if !s.runtimePolicy().Enabled {
		return nil, ErrDisabled
	}
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, ErrDisabled
	}
	raw, err := adapter.Raw(ctx, m.FolderID, m.RemoteID)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errors.New("raw message exceeds configured size limit")
	}
	m.Raw = append([]byte(nil), raw...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.store.UpdateMessage(ctx, m); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Service) MessageAttachment(ctx context.Context, messageID, attachmentID string) (Attachment, io.ReadCloser, error) {
	m, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		return Attachment{}, nil, err
	}
	adapter, account, err := s.resolve(ctx, m.AccountID)
	if err != nil {
		return Attachment{}, nil, err
	}
	if account.Disabled {
		return Attachment{}, nil, ErrDisabled
	}
	if !hasCapability(account.Capabilities, "read") {
		return Attachment{}, nil, ErrCapability
	}
	meta, ok := attachmentMetadata(m.Attachments, attachmentID)
	if !ok {
		return Attachment{}, nil, ErrNotFound
	}
	if len(m.Raw) > 0 {
		_, _, _, _, _, _, attachments, _, err := ParseMIME(m.Raw, int64(len(m.Raw)))
		if err != nil {
			return Attachment{}, nil, err
		}
		for _, attachment := range attachments {
			if attachment.ID == attachmentID {
				return attachment, io.NopCloser(bytes.NewReader(attachment.Data)), nil
			}
		}
		return Attachment{}, nil, ErrNotFound
	}
	if !s.runtimePolicy().Enabled {
		return Attachment{}, nil, ErrDisabled
	}
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return Attachment{}, nil, err
	}
	if !settings.Enabled {
		return Attachment{}, nil, ErrDisabled
	}
	reader, err := adapter.Attachment(ctx, m.FolderID, m.RemoteID, attachmentID)
	if err != nil {
		return Attachment{}, nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, s.blobs.maxFile+1))
	closeErr := reader.Close()
	if readErr != nil {
		return Attachment{}, nil, readErr
	}
	if closeErr != nil {
		return Attachment{}, nil, closeErr
	}
	if int64(len(data)) > s.blobs.maxFile {
		return Attachment{}, nil, fmt.Errorf("mail attachment exceeds configured size limit")
	}
	meta.Size = int64(len(data))
	return meta, io.NopCloser(bytes.NewReader(data)), nil
}

func attachmentMetadata(attachments []Attachment, id string) (Attachment, bool) {
	for _, attachment := range attachments {
		if attachment.ID == id {
			return attachment, true
		}
	}
	return Attachment{}, false
}
