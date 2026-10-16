package mailer

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current int64
	if err := tx.QueryRowContext(ctx, "SELECT revision FROM mailer_drafts WHERE id=?", id).Scan(&current); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if current != revision {
		return ErrConflict
	}
	var pending int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM mailer_outbox WHERE draft_id=? AND status IN ('scheduled','sending','unknown') LIMIT 1", id).Scan(&pending); err == nil {
		return ErrConflict
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	res, err := tx.ExecContext(ctx, "DELETE FROM mailer_drafts WHERE id=? AND revision=?", id, revision)
	if err != nil {
		return fmt.Errorf("delete mail draft: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit mail draft deletion: %w", err)
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
	rows, err := s.db.QueryContext(ctx, "SELECT id,account_id,draft_id,revision,timezone,send_at,undo_until,status,error,remote_id,append_error,created_at,updated_at FROM mailer_outbox ORDER BY send_at DESC,created_at DESC,id DESC")
	if err != nil {
		return nil, fmt.Errorf("list mail outbox: %w", err)
	}
	defer rows.Close()
	items := make([]OutboxItem, 0)
	for rows.Next() {
		v, err := scanOutboxItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (s *Store) GetOutbox(ctx context.Context, id string) (OutboxItem, error) {
	row := s.db.QueryRowContext(ctx, "SELECT id,account_id,draft_id,revision,timezone,send_at,undo_until,status,error,remote_id,append_error,created_at,updated_at FROM mailer_outbox WHERE id=?", id)
	item, err := scanOutboxItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return OutboxItem{}, ErrNotFound
	}
	return item, err
}

func (s *Store) ListOutboxPage(ctx context.Context, cursor string, limit int, allowedAccountIDs []string) (OutboxPage, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	if len(allowedAccountIDs) == 0 {
		return OutboxPage{Items: []OutboxItem{}}, nil
	}
	var point outboxCursor
	if cursor != "" {
		data, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || json.Unmarshal(data, &point) != nil || point.ID == "" || point.SendAt == 0 || point.CreatedAt == 0 {
			return OutboxPage{}, fmt.Errorf("%w: invalid outbox cursor", ErrInvalid)
		}
	}
	query := "SELECT id,account_id,draft_id,revision,timezone,send_at,undo_until,status,error,remote_id,append_error,created_at,updated_at FROM mailer_outbox WHERE account_id IN (" + strings.TrimRight(strings.Repeat("?,", len(allowedAccountIDs)), ",") + ")"
	args := make([]any, 0, len(allowedAccountIDs)+4)
	for _, id := range allowedAccountIDs {
		args = append(args, id)
	}
	if cursor != "" {
		query += " AND (send_at,created_at,id) < (?,?,?)"
		args = append(args, point.SendAt, point.CreatedAt, point.ID)
	}
	query += " ORDER BY send_at DESC,created_at DESC,id DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return OutboxPage{}, fmt.Errorf("list mail outbox page: %w", err)
	}
	defer rows.Close()
	items := make([]OutboxItem, 0, limit+1)
	for rows.Next() {
		item, err := scanOutboxItem(rows)
		if err != nil {
			return OutboxPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return OutboxPage{}, err
	}
	page := OutboxPage{Items: items}
	if len(items) > limit {
		page.HasMore = true
		page.Items = items[:limit]
		last := page.Items[len(page.Items)-1]
		encoded, err := json.Marshal(outboxCursor{SendAt: timeNS(last.SendAt), CreatedAt: timeNS(last.CreatedAt), ID: last.ID})
		if err != nil {
			return OutboxPage{}, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	return page, nil
}

type outboxCursor struct {
	SendAt    int64  `json:"s"`
	CreatedAt int64  `json:"c"`
	ID        string `json:"i"`
}

type outboxScanner interface {
	Scan(...any) error
}

func scanOutboxItem(row outboxScanner) (OutboxItem, error) {
	var item OutboxItem
	var sendAt, undo, created, updated int64
	if err := row.Scan(&item.ID, &item.AccountID, &item.DraftID, &item.Revision, &item.Timezone, &sendAt, &undo, &item.Status, &item.Error, &item.RemoteID, &item.AppendError, &created, &updated); err != nil {
		return OutboxItem{}, err
	}
	item.SendAt, item.UndoUntil, item.CreatedAt, item.UpdatedAt = fromNS(sendAt), fromNS(undo), fromNS(created), fromNS(updated)
	return item, nil
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
	return s.GetOutbox(ctx, id)
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `DELETE FROM mailer_blobs WHERE id=? AND NOT EXISTS (
SELECT 1 FROM mailer_outbox_blobs b JOIN mailer_outbox o ON o.id=b.outbox_id
WHERE b.blob_id=mailer_blobs.id AND o.status IN ('scheduled','sending','unknown'))`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM mailer_blobs WHERE id=?", id).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		return ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit mail attachment removal: %w", err)
	}
	return nil
}

func (s *Store) BlobHasPendingSend(ctx context.Context, blobID string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM mailer_outbox_blobs b JOIN mailer_outbox o ON o.id=b.outbox_id
WHERE b.blob_id=? AND o.status IN ('scheduled','sending','unknown') LIMIT 1`, blobID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) DraftHasPendingSend(ctx context.Context, draftID string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM mailer_outbox WHERE draft_id=? AND status IN ('scheduled','sending','unknown') LIMIT 1", draftID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) RemoveDraftAttachment(ctx context.Context, draftID string, revision int64, blobID string) (Draft, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Draft{}, err
	}
	defer tx.Rollback()
	var body []byte
	if err := tx.QueryRowContext(ctx, "SELECT body FROM mailer_drafts WHERE id=?", draftID).Scan(&body); errors.Is(err, sql.ErrNoRows) {
		return Draft{}, ErrNotFound
	} else if err != nil {
		return Draft{}, err
	}
	var draft Draft
	if err := unmarshal(body, &draft); err != nil {
		return Draft{}, err
	}
	if draft.Revision != revision {
		return Draft{}, ErrConflict
	}
	attachments := make([]Attachment, 0, len(draft.Attachments))
	found := false
	for _, attachment := range draft.Attachments {
		if attachment.ID == blobID {
			found = true
			continue
		}
		attachments = append(attachments, attachment)
	}
	if !found {
		return Draft{}, ErrNotFound
	}
	draft.Attachments = attachments
	draft.Revision++
	draft.UpdatedAt = time.Now().UTC()
	updated, err := marshal(draft)
	if err != nil {
		return Draft{}, err
	}
	res, err := tx.ExecContext(ctx, `UPDATE mailer_drafts SET revision=?,body=?,updated_at=? WHERE id=? AND revision=? AND NOT EXISTS (
SELECT 1 FROM mailer_outbox WHERE draft_id=? AND status IN ('scheduled','sending','unknown'))`, draft.Revision, updated, timeNS(draft.UpdatedAt), draftID, revision, draftID)
	if err != nil {
		return Draft{}, fmt.Errorf("update mail draft attachments: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var pending int
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM mailer_outbox WHERE draft_id=? AND status IN ('scheduled','sending','unknown') LIMIT 1", draftID).Scan(&pending); err == nil {
			return Draft{}, ErrConflict
		} else if !errors.Is(err, sql.ErrNoRows) {
			return Draft{}, err
		}
		return Draft{}, ErrConflict
	}
	res, err = tx.ExecContext(ctx, `DELETE FROM mailer_blobs WHERE id=? AND owner_kind='draft' AND owner_id=? AND NOT EXISTS (
SELECT 1 FROM mailer_outbox_blobs b JOIN mailer_outbox o ON o.id=b.outbox_id
WHERE b.blob_id=mailer_blobs.id AND o.status IN ('scheduled','sending','unknown'))`, blobID, draftID)
	if err != nil {
		return Draft{}, fmt.Errorf("remove mail attachment index: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Draft{}, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return Draft{}, fmt.Errorf("commit mail draft attachment removal: %w", err)
	}
	return draft, nil
}
