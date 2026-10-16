package mailer

import (
	"context"
	"fmt"
	"io"
	"time"
	_ "time/tzdata" // Scheduled mail must resolve IANA zones on portable installations.
)

func (s *Service) ListOutbox(ctx context.Context) ([]OutboxItem, error) {
	items, err := s.store.ListOutbox(ctx)
	if err != nil {
		return nil, err
	}
	allowed, err := s.allowedAccountIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]OutboxItem, 0, len(items))
	for _, item := range items {
		if contains(allowed, item.AccountID) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *Service) QueueSend(ctx context.Context, req SendRequest) (OutboxItem, error) {
	ctx, cancel, err := s.begin(ctx, true)
	if err != nil {
		return OutboxItem{}, err
	}
	defer cancel()
	d, err := s.store.GetDraft(ctx, req.DraftID)
	if err != nil {
		return OutboxItem{}, err
	}
	if d.Revision != req.Revision {
		return OutboxItem{}, ErrConflict
	}
	if err := validateDraft(d); err != nil {
		return OutboxItem{}, err
	}
	if len(d.To)+len(d.CC)+len(d.BCC) == 0 {
		return OutboxItem{}, fmt.Errorf("%w: sending requires a recipient", ErrInvalid)
	}
	if err := validateOutgoingSize(d); err != nil {
		return OutboxItem{}, err
	}
	if req.Timezone == "" {
		req.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil {
		return OutboxItem{}, fmt.Errorf("%w: invalid schedule timezone", ErrInvalid)
	}
	if _, _, err := s.sendAccount(ctx, d.AccountID); err != nil {
		return OutboxItem{}, err
	}
	for _, a := range d.Attachments {
		if !s.store.BlobOwnedBy(ctx, a.ID, "draft", d.ID) {
			return OutboxItem{}, fmt.Errorf("attachment %q is not owned by this draft", a.ID)
		}
	}
	item, err := s.store.QueueSend(ctx, req, d)
	if err != nil {
		return OutboxItem{}, err
	}
	s.emit(Event{Type: "outbox", AccountID: item.AccountID, ID: item.ID, Status: item.Status})
	return item, nil
}

func (s *Service) CancelSend(ctx context.Context, id string) error {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return err
	}
	defer cancel()
	items, err := s.store.ListOutbox(ctx)
	if err != nil {
		return err
	}
	var found *OutboxItem
	for i := range items {
		if items[i].ID == id {
			found = &items[i]
			break
		}
	}
	if found == nil {
		return ErrNotFound
	}
	account, err := s.authorizedAccount(ctx, found.AccountID)
	if err != nil {
		return err
	}
	if account.Disabled {
		return ErrDisabled
	}
	if account.ReadOnly {
		return ErrReadOnly
	}
	if err := s.store.CancelSend(ctx, id); err != nil {
		return err
	}
	s.emit(Event{Type: "outbox", ID: id, Status: "cancelled"})
	return nil
}

func (s *Service) RetrySend(ctx context.Context, id string) (OutboxItem, error) {
	ctx, cancel, err := s.begin(ctx, true)
	if err != nil {
		return OutboxItem{}, err
	}
	defer cancel()
	items, err := s.store.ListOutbox(ctx)
	if err != nil {
		return OutboxItem{}, err
	}
	var found *OutboxItem
	for i := range items {
		if items[i].ID == id {
			found = &items[i]
			break
		}
	}
	if found == nil {
		return OutboxItem{}, ErrNotFound
	}
	if _, _, err := s.sendAccount(ctx, found.AccountID); err != nil {
		return OutboxItem{}, err
	}
	item, err := s.store.RetrySend(ctx, id)
	if err != nil {
		return OutboxItem{}, err
	}
	s.emit(Event{Type: "outbox", AccountID: item.AccountID, ID: item.ID, Status: item.Status})
	return item, nil
}

func (s *Service) sendAccount(ctx context.Context, id string) (Adapter, Account, error) {
	adapter, account, err := s.resolve(ctx, id)
	if err != nil {
		return nil, Account{}, err
	}
	if account.Disabled {
		return nil, Account{}, ErrDisabled
	}
	if account.ReadOnly {
		return nil, Account{}, ErrReadOnly
	}
	if !hasCapability(account.Capabilities, "send") {
		return nil, Account{}, ErrCapability
	}
	return adapter, account, nil
}

func (s *Service) outboxLoop(ctx context.Context) {
	defer s.lifeWG.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.dispatchDue(ctx)
		}
	}
}

func (s *Service) dispatchDue(parent context.Context) {
	if !s.outboxMu.TryLock() {
		return
	}
	defer s.outboxMu.Unlock()
	p := s.runtimePolicy()
	if !p.Enabled || p.ReadOnly {
		return
	}
	settings, err := s.store.GetSettings(parent)
	if err != nil || !settings.Enabled {
		return
	}
	ctx, cancel, err := s.begin(parent, true)
	if err != nil {
		return
	}
	defer cancel()
	item, draft, err := s.store.ClaimDue(ctx, time.Now())
	if err != nil {
		return
	}
	adapter, account, err := s.sendAccount(ctx, item.AccountID)
	if err != nil {
		status := "failed"
		_ = s.store.FinishSend(context.WithoutCancel(ctx), item.ID, status, "account_unavailable", "")
		s.emit(Event{Type: "outbox", AccountID: item.AccountID, ID: item.ID, Status: status})
		return
	}
	if err := validateOutgoingSize(draft); err != nil {
		_ = s.store.FinishSend(context.WithoutCancel(ctx), item.ID, "failed", "message_too_large", "")
		s.emit(Event{Type: "outbox", AccountID: item.AccountID, ID: item.ID, Status: "failed"})
		return
	}
	for i := range draft.Attachments {
		if !s.store.BlobOwnedBy(ctx, draft.Attachments[i].ID, "draft", draft.ID) {
			_ = s.store.FinishSend(context.WithoutCancel(ctx), item.ID, "failed", "attachment_unavailable", "")
			s.emit(Event{Type: "outbox", AccountID: item.AccountID, ID: item.ID, Status: "failed"})
			return
		}
		_, f, openErr := s.blobs.Open(ctx, draft.Attachments[i].ID)
		if openErr != nil {
			_ = s.store.FinishSend(context.WithoutCancel(ctx), item.ID, "failed", "attachment_unavailable", "")
			s.emit(Event{Type: "outbox", AccountID: item.AccountID, ID: item.ID, Status: "failed"})
			return
		}
		data, readErr := io.ReadAll(io.LimitReader(f, s.blobs.maxFile+1))
		closeErr := f.Close()
		if readErr != nil || closeErr != nil || int64(len(data)) > s.blobs.maxFile {
			_ = s.store.FinishSend(context.WithoutCancel(ctx), item.ID, "failed", "attachment_unavailable", "")
			s.emit(Event{Type: "outbox", AccountID: item.AccountID, ID: item.ID, Status: "failed"})
			return
		}
		draft.Attachments[i].Data = data
	}
	result, sendErr := adapter.Send(ctx, Outgoing{Draft: draft})
	if sendErr != nil {
		status := "unknown"
		if result.Rejected {
			status = "failed"
		}
		code := "transport_unknown"
		if status == "failed" {
			code = "provider_rejected"
		}
		_ = s.store.FinishSend(context.WithoutCancel(ctx), item.ID, status, code, result.RemoteID)
		s.emit(Event{Type: "outbox", AccountID: item.AccountID, ID: item.ID, Status: status})
		return
	}
	if !result.Confirmed {
		status := "unknown"
		if result.Rejected {
			status = "failed"
		}
		code := "unconfirmed_send"
		if status == "failed" {
			code = "provider_rejected"
		}
		_ = s.store.FinishSend(context.WithoutCancel(ctx), item.ID, status, code, result.RemoteID)
		s.emit(Event{Type: "outbox", AccountID: item.AccountID, ID: item.ID, Status: status})
		return
	}
	if err := s.store.FinishSend(context.WithoutCancel(ctx), item.ID, "sent", "", result.RemoteID); err != nil {
		return
	}
	appendErr := error(nil)
	accountSettings, _ := s.store.AccountSettings(ctx, account.ID)
	if mappedID := accountSettings.FolderMap["sent"]; mappedID != "" {
		if appender, ok := adapter.(MappedSentAppender); ok {
			var mapped Folder
			folders, _ := s.store.Folders(ctx, account.ID)
			for _, folder := range folders {
				if folder.ID == mappedID {
					mapped = folder
					break
				}
			}
			if mapped.ID == "" {
				appendErr = ErrCapability
			} else {
				appendErr = appender.AppendSentTo(ctx, Outgoing{Draft: draft}, result, mapped)
			}
		} else if appender, ok := adapter.(SentAppender); ok {
			appendErr = appender.AppendSent(ctx, Outgoing{Draft: draft}, result)
		}
	} else if appender, ok := adapter.(SentAppender); ok {
		appendErr = appender.AppendSent(ctx, Outgoing{Draft: draft}, result)
	}
	if appendErr != nil {
		_ = s.store.SetSentAppendError(context.WithoutCancel(ctx), item.ID, "sent_copy_failed")
	}
	if result.Message != nil {
		m := *result.Message
		if m.AccountID == "" {
			m.AccountID = account.ID
		}
		if m.FolderID == "" {
			if folders, e := adapter.Folders(ctx); e == nil {
				for _, f := range folders {
					if f.Kind == "sent" || f.Kind == "Sent" {
						m.FolderID = f.ID
						break
					}
				}
			}
		}
		if m.RemoteID != "" && m.FolderID != "" {
			_, _ = s.store.PutMessages(context.WithoutCancel(ctx), []Message{m})
		}
	}
	s.emit(Event{Type: "outbox", AccountID: item.AccountID, ID: item.ID, Status: "sent"})
}

func validateOutgoingSize(draft Draft) error {
	var total int64
	for _, attachment := range draft.Attachments {
		if attachment.Size < 0 || attachment.Size > maxOutgoingAttachmentBytes-total {
			return fmt.Errorf("%w: outgoing attachments exceed size limit", ErrInvalid)
		}
		total += attachment.Size
	}
	if int64(len(draft.Text))+int64(len(draft.HTML))+total > maxOutgoingMessageBytes {
		return fmt.Errorf("%w: outgoing message exceeds size limit", ErrInvalid)
	}
	return nil
}

func (s *Service) syncLoop(ctx context.Context) {
	defer s.lifeWG.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	s.syncAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncAll(ctx)
		}
	}
}

func (s *Service) ensureStarted() bool {
	s.lifeMu.Lock()
	defer s.lifeMu.Unlock()
	return s.lifeCancel != nil
}
