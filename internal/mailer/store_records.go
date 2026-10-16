package mailer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) SaveDraft(ctx context.Context, d Draft) (Draft, error) {
	if d.ID == "" {
		id, err := newID("draft_")
		if err != nil {
			return Draft{}, err
		}
		d.ID = id
	}
	now := time.Now().UTC()
	d.UpdatedAt = now
	var err error
	if d.Revision == 0 {
		d.Revision = 1
		var b []byte
		b, err = marshal(d)
		if err != nil {
			return Draft{}, err
		}
		_, err = s.db.ExecContext(ctx, "INSERT INTO mailer_drafts(id,account_id,revision,body,updated_at) VALUES(?,?,?,?,?)", d.ID, d.AccountID, d.Revision, b, timeNS(now))
		if err != nil {
			return Draft{}, fmt.Errorf("create mail draft: %w", err)
		}
		return d, nil
	}
	expected := d.Revision
	d.Revision++
	var b []byte
	b, err = marshal(d)
	if err != nil {
		return Draft{}, err
	}
	res, err := s.db.ExecContext(ctx, "UPDATE mailer_drafts SET account_id=?,revision=?,body=?,updated_at=? WHERE id=? AND revision=? AND (account_id=? OR NOT EXISTS (SELECT 1 FROM mailer_outbox WHERE draft_id=? AND status IN ('scheduled','sending','unknown')))", d.AccountID, d.Revision, b, timeNS(now), d.ID, expected, d.AccountID, d.ID)
	if err != nil {
		return Draft{}, fmt.Errorf("update mail draft: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Draft{}, ErrConflict
	}
	return d, nil
}

func (s *Store) GetDraft(ctx context.Context, id string) (Draft, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT body FROM mailer_drafts WHERE id=?", id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return Draft{}, ErrNotFound
	}
	if err != nil {
		return Draft{}, fmt.Errorf("read mail draft: %w", err)
	}
	var d Draft
	if err := unmarshal(b, &d); err != nil {
		return Draft{}, err
	}
	return d, nil
}

func (s *Store) ListDrafts(ctx context.Context) ([]Draft, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT d.body FROM mailer_drafts d WHERE NOT EXISTS (SELECT 1 FROM mailer_outbox o WHERE o.draft_id=d.id AND o.revision=d.revision AND o.status='sent') ORDER BY d.updated_at DESC,d.id")
	if err != nil {
		return nil, fmt.Errorf("list mail drafts: %w", err)
	}
	defer rows.Close()
	drafts := make([]Draft, 0)
	for rows.Next() {
		var b []byte
		var d Draft
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		if err := unmarshal(b, &d); err != nil {
			return nil, err
		}
		drafts = append(drafts, d)
	}
	return drafts, rows.Err()
}

func (s *Store) DeleteDraft(ctx context.Context, id string, revision int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM mailer_drafts WHERE id=? AND revision=?", id, revision)
	if err != nil {
		return fmt.Errorf("delete mail draft: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Store) SaveTemplate(ctx context.Context, t Template) (Template, error) {
	if t.ID == "" {
		id, err := newID("template_")
		if err != nil {
			return Template{}, err
		}
		t.ID = id
	}
	if t.Kind != "template" && t.Kind != "signature" {
		return Template{}, errors.New("template kind must be template or signature")
	}
	now := time.Now().UTC()
	t.UpdatedAt = now
	expected := t.Revision
	if expected == 0 {
		t.Revision = 1
		b, err := marshal(t)
		if err != nil {
			return Template{}, err
		}
		_, err = s.db.ExecContext(ctx, "INSERT INTO mailer_templates(id,account_id,kind,name,revision,body,updated_at) VALUES(?,?,?,?,?,?,?)", t.ID, t.AccountID, t.Kind, t.Name, t.Revision, b, timeNS(now))
		if err != nil {
			return Template{}, fmt.Errorf("create mail template: %w", err)
		}
		return t, nil
	}
	t.Revision++
	b, err := marshal(t)
	if err != nil {
		return Template{}, err
	}
	res, err := s.db.ExecContext(ctx, "UPDATE mailer_templates SET account_id=?,kind=?,name=?,revision=?,body=?,updated_at=? WHERE id=? AND revision=?", t.AccountID, t.Kind, t.Name, t.Revision, b, timeNS(now), t.ID, expected)
	if err != nil {
		return Template{}, fmt.Errorf("update mail template: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Template{}, ErrConflict
	}
	return t, nil
}

func (s *Store) ListTemplates(ctx context.Context) ([]Template, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT body FROM mailer_templates ORDER BY kind,name,id")
	if err != nil {
		return nil, fmt.Errorf("list mail templates: %w", err)
	}
	defer rows.Close()
	values := make([]Template, 0)
	for rows.Next() {
		var b []byte
		var v Template
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		if err := unmarshal(b, &v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func (s *Store) GetTemplate(ctx context.Context, id string) (Template, error) {
	var body []byte
	err := s.db.QueryRowContext(ctx, "SELECT body FROM mailer_templates WHERE id=?", id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return Template{}, ErrNotFound
	}
	if err != nil {
		return Template{}, err
	}
	var value Template
	if err := unmarshal(body, &value); err != nil {
		return Template{}, err
	}
	return value, nil
}

func (s *Store) DeleteTemplate(ctx context.Context, id string, revision int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM mailer_templates WHERE id=? AND revision=?", id, revision)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SaveRule(ctx context.Context, r Rule) (Rule, error) {
	if r.ID == "" {
		id, err := newID("rule_")
		if err != nil {
			return Rule{}, err
		}
		r.ID = id
	}
	now := time.Now().UTC()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	expected := r.Revision
	if expected == 0 {
		r.Revision = 1
		b, err := marshal(r)
		if err != nil {
			return Rule{}, err
		}
		_, err = s.db.ExecContext(ctx, "INSERT INTO mailer_rules(id,account_id,revision,enabled,body,created_at,updated_at) VALUES(?,?,?,?,?,?,?)", r.ID, r.AccountID, r.Revision, r.Enabled, b, timeNS(r.CreatedAt), timeNS(now))
		if err != nil {
			return Rule{}, fmt.Errorf("create mail rule: %w", err)
		}
		return r, nil
	}
	r.Revision++
	b, err := marshal(r)
	if err != nil {
		return Rule{}, err
	}
	res, err := s.db.ExecContext(ctx, "UPDATE mailer_rules SET account_id=?,revision=?,enabled=?,body=?,updated_at=? WHERE id=? AND revision=?", r.AccountID, r.Revision, r.Enabled, b, timeNS(now), r.ID, expected)
	if err != nil {
		return Rule{}, fmt.Errorf("update mail rule: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Rule{}, ErrConflict
	}
	return r, nil
}

func (s *Store) GetRule(ctx context.Context, id string) (Rule, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT body FROM mailer_rules WHERE id=?", id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return Rule{}, ErrNotFound
	}
	if err != nil {
		return Rule{}, err
	}
	var r Rule
	if err := unmarshal(b, &r); err != nil {
		return Rule{}, err
	}
	return r, nil
}

func (s *Store) ListRules(ctx context.Context) ([]Rule, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT body FROM mailer_rules ORDER BY created_at,id")
	if err != nil {
		return nil, fmt.Errorf("list mail rules: %w", err)
	}
	defer rows.Close()
	values := make([]Rule, 0)
	for rows.Next() {
		var b []byte
		var r Rule
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		if err := unmarshal(b, &r); err != nil {
			return nil, err
		}
		values = append(values, r)
	}
	return values, rows.Err()
}

func (s *Store) DeleteRule(ctx context.Context, id string, revision int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM mailer_rules WHERE id=? AND revision=?", id, revision)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

type storedOutgoing struct {
	Draft Draft `json:"draft"`
}

func (s *Store) QueueSend(ctx context.Context, req SendRequest, d Draft) (OutboxItem, error) {
	id, err := newID("send_")
	if err != nil {
		return OutboxItem{}, err
	}
	now := time.Now().UTC()
	sendAt := req.SendAt.UTC()
	if sendAt.IsZero() || sendAt.Before(now.Add(10*time.Second)) {
		sendAt = now.Add(10 * time.Second)
	}
	item := OutboxItem{ID: id, AccountID: d.AccountID, DraftID: d.ID, Revision: d.Revision, Timezone: req.Timezone, SendAt: sendAt, UndoUntil: now.Add(10 * time.Second), Status: "scheduled", CreatedAt: now, UpdatedAt: now}
	b, err := marshal(storedOutgoing{Draft: d})
	if err != nil {
		return OutboxItem{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OutboxItem{}, err
	}
	defer tx.Rollback()
	var existing OutboxItem
	var oldSend, oldUndo, oldCreated, oldUpdated int64
	err = tx.QueryRowContext(ctx, "SELECT id,account_id,draft_id,revision,timezone,send_at,undo_until,status,error,remote_id,append_error,created_at,updated_at FROM mailer_outbox WHERE draft_id=? AND revision=? AND status!='cancelled' ORDER BY created_at DESC LIMIT 1", d.ID, d.Revision).Scan(&existing.ID, &existing.AccountID, &existing.DraftID, &existing.Revision, &existing.Timezone, &oldSend, &oldUndo, &existing.Status, &existing.Error, &existing.RemoteID, &existing.AppendError, &oldCreated, &oldUpdated)
	if err == nil {
		existing.SendAt, existing.UndoUntil, existing.CreatedAt, existing.UpdatedAt = fromNS(oldSend), fromNS(oldUndo), fromNS(oldCreated), fromNS(oldUpdated)
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return OutboxItem{}, err
	}
	var currentRevision int64
	if err := tx.QueryRowContext(ctx, "SELECT revision FROM mailer_drafts WHERE id=?", d.ID).Scan(&currentRevision); err != nil {
		return OutboxItem{}, ErrNotFound
	}
	if currentRevision != d.Revision {
		return OutboxItem{}, ErrConflict
	}
	var unsafe int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM mailer_outbox WHERE draft_id=? AND status IN ('sending','sent','unknown')", d.ID).Scan(&unsafe); err != nil {
		return OutboxItem{}, err
	}
	if unsafe > 0 {
		return OutboxItem{}, ErrConflict
	}
	if _, err := tx.ExecContext(ctx, "UPDATE mailer_outbox SET status='cancelled',updated_at=? WHERE draft_id=? AND status='scheduled'", timeNS(now), d.ID); err != nil {
		return OutboxItem{}, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO mailer_outbox(id,account_id,draft_id,revision,timezone,send_at,undo_until,status,body,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", item.ID, item.AccountID, item.DraftID, item.Revision, item.Timezone, timeNS(item.SendAt), timeNS(item.UndoUntil), item.Status, b, timeNS(now), timeNS(now)); err != nil {
		return OutboxItem{}, fmt.Errorf("queue mail send: %w", err)
	}
	for _, attachment := range d.Attachments {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM mailer_blobs WHERE id=? AND owner_kind='draft' AND owner_id=?", attachment.ID, d.ID).Scan(&exists); err != nil {
			return OutboxItem{}, ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO mailer_outbox_blobs(outbox_id,blob_id) VALUES(?,?)", item.ID, attachment.ID); err != nil {
			return OutboxItem{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return OutboxItem{}, fmt.Errorf("commit queued mail send: %w", err)
	}
	return item, nil
}

func (s *Store) ListOutbox(ctx context.Context) ([]OutboxItem, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,account_id,draft_id,revision,timezone,send_at,undo_until,status,error,remote_id,append_error,created_at,updated_at FROM mailer_outbox ORDER BY send_at DESC,created_at DESC LIMIT 500")
	if err != nil {
		return nil, fmt.Errorf("list mail outbox: %w", err)
	}
	defer rows.Close()
	items := make([]OutboxItem, 0)
	for rows.Next() {
		var v OutboxItem
		var sendAt, undo, created, updated int64
		var appendError string
		if err := rows.Scan(&v.ID, &v.AccountID, &v.DraftID, &v.Revision, &v.Timezone, &sendAt, &undo, &v.Status, &v.Error, &v.RemoteID, &appendError, &created, &updated); err != nil {
			return nil, err
		}
		v.SendAt = fromNS(sendAt)
		v.UndoUntil = fromNS(undo)
		v.CreatedAt = fromNS(created)
		v.UpdatedAt = fromNS(updated)
		v.AppendError = appendError
		items = append(items, v)
	}
	return items, rows.Err()
}

func (s *Store) ClaimDue(ctx context.Context, now time.Time) (OutboxItem, Draft, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OutboxItem{}, Draft{}, err
	}
	defer tx.Rollback()
	var item OutboxItem
	var sendAt, undo, created, updated int64
	var b []byte
	err = tx.QueryRowContext(ctx, "SELECT id,account_id,draft_id,revision,timezone,send_at,undo_until,body,created_at,updated_at FROM mailer_outbox WHERE status='scheduled' AND send_at<=? ORDER BY send_at,created_at LIMIT 1", timeNS(now)).Scan(&item.ID, &item.AccountID, &item.DraftID, &item.Revision, &item.Timezone, &sendAt, &undo, &b, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return OutboxItem{}, Draft{}, ErrNotFound
	}
	if err != nil {
		return OutboxItem{}, Draft{}, err
	}
	var currentRevision int64
	if err := tx.QueryRowContext(ctx, "SELECT revision FROM mailer_drafts WHERE id=?", item.DraftID).Scan(&currentRevision); errors.Is(err, sql.ErrNoRows) || err == nil && currentRevision != item.Revision {
		if _, e := tx.ExecContext(ctx, "UPDATE mailer_outbox SET status='failed',error='draft_changed',updated_at=? WHERE id=?", timeNS(now), item.ID); e != nil {
			return OutboxItem{}, Draft{}, e
		}
		if e := tx.Commit(); e != nil {
			return OutboxItem{}, Draft{}, e
		}
		return OutboxItem{}, Draft{}, ErrConflict
	} else if err != nil {
		return OutboxItem{}, Draft{}, err
	}
	res, err := tx.ExecContext(ctx, "UPDATE mailer_outbox SET status='sending',updated_at=? WHERE id=? AND status='scheduled'", timeNS(now), item.ID)
	if err != nil {
		return OutboxItem{}, Draft{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return OutboxItem{}, Draft{}, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return OutboxItem{}, Draft{}, err
	}
	var snapshot storedOutgoing
	if err := unmarshal(b, &snapshot); err != nil {
		return OutboxItem{}, Draft{}, err
	}
	item.SendAt = fromNS(sendAt)
	item.UndoUntil = fromNS(undo)
	item.Status = "sending"
	item.CreatedAt = fromNS(created)
	item.UpdatedAt = now.UTC()
	return item, snapshot.Draft, nil
}

func (s *Store) FinishSend(ctx context.Context, id, status, message, remoteID string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE mailer_outbox SET status=?,error=?,remote_id=?,updated_at=? WHERE id=? AND status='sending'", status, message, remoteID, timeNS(time.Now()), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Store) SetSentAppendError(ctx context.Context, id, message string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE mailer_outbox SET append_error=?,updated_at=? WHERE id=? AND status='sent'", message, timeNS(time.Now()), id)
	return err
}

func (s *Store) CancelSend(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE mailer_outbox SET status='cancelled',updated_at=? WHERE id=? AND status='scheduled'", timeNS(time.Now()), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Store) RetrySend(ctx context.Context, id string) (OutboxItem, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, "UPDATE mailer_outbox SET status='scheduled',send_at=?,undo_until=?,error='',updated_at=? WHERE id=? AND status='failed'", timeNS(now.Add(10*time.Second)), timeNS(now.Add(10*time.Second)), timeNS(now), id)
	if err != nil {
		return OutboxItem{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return OutboxItem{}, ErrConflict
	}
	items, err := s.ListOutbox(ctx)
	if err != nil {
		return OutboxItem{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return OutboxItem{}, ErrNotFound
}

func (s *Store) RecoverSending(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "UPDATE mailer_outbox SET status='unknown',error='send outcome unknown after restart',updated_at=? WHERE status='sending'", timeNS(time.Now()))
	if err != nil {
		return fmt.Errorf("reconcile mail outbox after restart: %w", err)
	}
	return nil
}

func (s *Store) SetBlob(ctx context.Context, a Attachment, digest, ownerKind, ownerID string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO mailer_blobs(id,filename,content_type,size,digest,owner_kind,owner_id,created_at) VALUES(?,?,?,?,?,?,?,?)", a.ID, a.Filename, a.ContentType, a.Size, digest, ownerKind, ownerID, timeNS(time.Now()))
	if err != nil {
		return fmt.Errorf("index mail attachment: %w", err)
	}
	return nil
}

func (s *Store) Blob(ctx context.Context, id string) (Attachment, string, error) {
	var a Attachment
	var digest string
	err := s.db.QueryRowContext(ctx, "SELECT id,filename,content_type,size,digest FROM mailer_blobs WHERE id=?", id).Scan(&a.ID, &a.Filename, &a.ContentType, &a.Size, &digest)
	if errors.Is(err, sql.ErrNoRows) {
		return Attachment{}, "", ErrNotFound
	}
	return a, digest, err
}

func (s *Store) BlobOwnedBy(ctx context.Context, id, ownerKind, ownerID string) bool {
	var exists int
	return s.db.QueryRowContext(ctx, "SELECT 1 FROM mailer_blobs WHERE id=? AND owner_kind=? AND owner_id=?", id, ownerKind, ownerID).Scan(&exists) == nil
}

func (s *Store) BlobOwner(ctx context.Context, id string) (string, string, error) {
	var kind, ownerID string
	err := s.db.QueryRowContext(ctx, "SELECT owner_kind,owner_id FROM mailer_blobs WHERE id=?", id).Scan(&kind, &ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return kind, ownerID, err
}

func (s *Store) DeleteBlob(ctx context.Context, id string) error {
	pending, err := s.BlobHasPendingSend(ctx, id)
	if err != nil {
		return err
	}
	if pending {
		return ErrConflict
	}
	res, err := s.db.ExecContext(ctx, "DELETE FROM mailer_blobs WHERE id=?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) BlobHasPendingSend(ctx context.Context, blobID string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM mailer_outbox_blobs b JOIN mailer_outbox o ON o.id=b.outbox_id
WHERE b.blob_id=? AND o.status IN ('scheduled','sending') LIMIT 1`, blobID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) DraftHasPendingSend(ctx context.Context, draftID string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM mailer_outbox WHERE draft_id=? AND status IN ('scheduled','sending') LIMIT 1", draftID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
