package mailer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	defaultMailCacheBytes = int64(2 << 30)
	maxRawSyncPageBytes   = int64(64 << 20)
)

func messageID(accountID, folderID, remoteID string) string {
	sum := sha256.Sum256([]byte(accountID + "\x00" + folderID + "\x00" + remoteID))
	return hex.EncodeToString(sum[:16])
}

// PutMessages upserts the provider metadata and returns only records that were
// new to the local index, which is the set eligible for automatic rules.
func (s *Store) PutMessages(ctx context.Context, messages []Message) ([]Message, error) {
	return s.PutMessagesWithLimit(ctx, messages, defaultMailCacheBytes)
}

func (s *Store) PutMessagesWithLimit(ctx context.Context, messages []Message, maxRawBytes int64) ([]Message, error) {
	return s.putMessagesWithLimits(ctx, messages, maxRawBytes, maxRawSyncPageBytes)
}

func (s *Store) putMessagesWithLimits(ctx context.Context, messages []Message, maxRawBytes, maxPageBytes int64) ([]Message, error) {
	if maxRawBytes <= 0 {
		maxRawBytes = defaultMailCacheBytes
	}
	if maxPageBytes <= 0 {
		maxPageBytes = maxRawSyncPageBytes
	}
	var pageBytes int64
	for _, message := range messages {
		pageBytes += int64(len(message.Raw))
		if pageBytes > maxPageBytes {
			return nil, ErrStorageFull
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin message sync: %w", err)
	}
	defer tx.Rollback()
	var usedCacheBytes int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(length(raw)+length(body)),0) FROM mailer_messages").Scan(&usedCacheBytes); err != nil {
		return nil, fmt.Errorf("check mail cache quota: %w", err)
	}
	created := make([]Message, 0)
	for _, m := range messages {
		if m.AccountID == "" || m.FolderID == "" || m.RemoteID == "" {
			return nil, errors.New("synced mail message is missing its account, folder, or remote ID")
		}
		if m.ID == "" {
			m.ID = messageID(m.AccountID, m.FolderID, m.RemoteID)
		}
		sanitizeMessageHeaders(&m)
		if m.Date.IsZero() {
			m.Date = time.Now().UTC()
		}
		protocol := ""
		_ = tx.QueryRowContext(ctx, "SELECT protocol FROM mailer_accounts WHERE id=?", m.AccountID).Scan(&protocol)
		m.ThreadID = resolveThreadID(ctx, tx, m, protocol)
		var oldRaw, oldRFC822 []byte
		err := tx.QueryRowContext(ctx, "SELECT body,raw FROM mailer_messages WHERE account_id=? AND folder_id=? AND remote_id=?", m.AccountID, m.FolderID, m.RemoteID).Scan(&oldRaw, &oldRFC822)
		// POP3 has one remote mailbox and the local folder is a client-side
		// organization. Re-indexing an archived message must not move it back.
		if errors.Is(err, sql.ErrNoRows) {
			var source string
			if e := tx.QueryRowContext(ctx, "SELECT protocol FROM mailer_accounts WHERE id=?", m.AccountID).Scan(&source); e == nil && strings.EqualFold(source, "pop3") {
				err = tx.QueryRowContext(ctx, "SELECT id,folder_id,body,raw FROM mailer_messages WHERE account_id=? AND remote_id=? LIMIT 1", m.AccountID, m.RemoteID).Scan(&m.ID, &m.FolderID, &oldRaw, &oldRFC822)
			}
		}
		isNew := errors.Is(err, sql.ErrNoRows)
		if err == nil {
			var old Message
			if err := unmarshal(oldRaw, &old); err != nil {
				return nil, err
			}
			if m.Text == "" {
				m.Text = old.Text
			}
			if m.HTML == "" {
				m.HTML = old.HTML
			}
			if m.Attachments == nil {
				m.Attachments = old.Attachments
			}
			if m.Labels == nil {
				m.Labels = old.Labels
			}
			if strings.EqualFold(protocol, "pop3") {
				// POP3 has no server-side read or star state; these flags belong
				// to the local cache and must survive every mailbox refresh.
				m.Unread = old.Unread
				m.Starred = old.Starred
			}
			if old.SnoozedUntil.After(m.SnoozedUntil) {
				m.SnoozedUntil = old.SnoozedUntil
			}
			if len(m.Raw) == 0 {
				m.Raw = oldRFC822
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("read indexed message: %w", err)
		}
		labels, err := marshal(m.Labels)
		if err != nil {
			return nil, err
		}
		body, err := marshal(m)
		if err != nil {
			return nil, err
		}
		pageBytes += int64(len(body))
		if pageBytes > maxPageBytes {
			return nil, ErrStorageFull
		}
		delta := int64(len(m.Raw) + len(body) - len(oldRFC822) - len(oldRaw))
		if delta > 0 && usedCacheBytes+delta > maxRawBytes {
			return nil, ErrStorageFull
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO mailer_messages(id,account_id,folder_id,remote_id,thread_id,message_id,in_reply_to,subject,sender,date,unread,starred,labels,snoozed_until,body,raw)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(account_id,folder_id,remote_id) DO UPDATE SET thread_id=excluded.thread_id,message_id=excluded.message_id,in_reply_to=excluded.in_reply_to,subject=excluded.subject,sender=excluded.sender,date=excluded.date,unread=excluded.unread,starred=excluded.starred,labels=excluded.labels,snoozed_until=CASE WHEN mailer_messages.snoozed_until>excluded.snoozed_until THEN mailer_messages.snoozed_until ELSE excluded.snoozed_until END,body=excluded.body,raw=CASE WHEN length(excluded.raw)>0 THEN excluded.raw ELSE mailer_messages.raw END`,
			m.ID, m.AccountID, m.FolderID, m.RemoteID, m.ThreadID, normalizeMessageID(m.MessageID), normalizeMessageID(m.InReplyTo), m.Subject, m.From, timeNS(m.Date), m.Unread, m.Starred, labels, timeNS(m.SnoozedUntil), body, nullIfEmpty(m.Raw))
		if err != nil {
			return nil, fmt.Errorf("index mail message: %w", err)
		}
		usedCacheBytes += delta
		if err := replaceMessageReferences(ctx, tx, m); err != nil {
			return nil, err
		}
		if isNew {
			created = append(created, m)
		}
		if isNew && m.MessageID != "" {
			if childThread, err := referencedChildThread(ctx, tx, m.AccountID, normalizeMessageID(m.MessageID)); err == nil && childThread != "" && childThread != m.ThreadID {
				m.ThreadID = childThread
				updated, _ := marshal(m)
				_, _ = tx.ExecContext(ctx, "UPDATE mailer_messages SET thread_id=?,body=? WHERE id=?", childThread, updated, m.ID)
				_, _ = tx.ExecContext(ctx, "UPDATE mailer_messages SET thread_id=? WHERE account_id=? AND id IN (SELECT message_id FROM mailer_message_refs WHERE account_id=? AND referenced_id=?)", childThread, m.AccountID, m.AccountID, normalizeMessageID(m.MessageID))
				if len(created) > 0 && created[len(created)-1].ID == m.ID {
					created[len(created)-1].ThreadID = childThread
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit message sync: %w", err)
	}
	return created, nil
}

func backfillMessageHeaders(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT account_id,id,body FROM mailer_messages")
	if err != nil {
		return err
	}
	type entry struct {
		accountID string
		id        string
		message   Message
	}
	entries := make([]entry, 0)
	for rows.Next() {
		var item entry
		var body []byte
		if err := rows.Scan(&item.accountID, &item.id, &body); err != nil {
			rows.Close()
			return err
		}
		if err := unmarshal(body, &item.message); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range entries {
		if _, err := tx.ExecContext(ctx, "UPDATE mailer_messages SET message_id=?,in_reply_to=? WHERE id=?", normalizeMessageID(item.message.MessageID), normalizeMessageID(item.message.InReplyTo), item.id); err != nil {
			return err
		}
		item.message.AccountID = item.accountID
		item.message.ID = item.id
		if err := replaceMessageReferences(ctx, tx, item.message); err != nil {
			return err
		}
	}
	return nil
}

func normalizeMessageID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "<>")
	return strings.ToLower(strings.TrimSpace(value))
}

func sanitizeMessageHeaders(message *Message) {
	message.MessageID = cleanMessageID(message.MessageID)
	message.InReplyTo = cleanMessageID(message.InReplyTo)
	refs := make([]string, 0, min(100, len(message.References)))
	for _, raw := range message.References {
		if len(refs) == 100 {
			break
		}
		if value := cleanMessageID(raw); value != "" {
			refs = append(refs, value)
		}
	}
	message.References = refs
}

func cleanMessageID(value string) string {
	value = normalizeMessageID(value)
	if len(value) == 0 || len(value) > 998 || strings.ContainsAny(value, " \t\r\n\x00<>") {
		return ""
	}
	return value
}

func resolveThreadID(ctx context.Context, tx *sql.Tx, message Message, protocol string) string {
	accountID := message.AccountID
	if strings.EqualFold(protocol, "imap") || strings.EqualFold(protocol, "pop3") || strings.EqualFold(protocol, "agentmail") {
		root := ""
		for _, ref := range message.References {
			if id := normalizeMessageID(ref); id != "" {
				root = id
				break
			}
		}
		if root == "" {
			root = normalizeMessageID(message.InReplyTo)
		}
		if root != "" {
			var threadID string
			err := tx.QueryRowContext(ctx, "SELECT thread_id FROM mailer_messages WHERE account_id=? AND message_id=? ORDER BY date,id LIMIT 1", accountID, root).Scan(&threadID)
			if err == nil && threadID != "" {
				return qualifyThreadID(accountID, threadID)
			}
			if messageID := normalizeMessageID(message.MessageID); messageID != "" {
				if childThread, childErr := referencedChildThread(ctx, tx, accountID, messageID); childErr == nil && childThread != "" {
					return qualifyThreadID(accountID, childThread)
				}
			}
			key := sha256.Sum256([]byte(accountID + "\x00" + root))
			return accountID + ":rfc:" + hex.EncodeToString(key[:12])
		}
	}
	if message.ThreadID == "" {
		return qualifyThreadID(accountID, messageID(accountID, message.FolderID, message.RemoteID))
	}
	return qualifyThreadID(accountID, message.ThreadID)
}

func qualifyThreadID(accountID, threadID string) string {
	if threadID == "" {
		return accountID + ":unknown"
	}
	if strings.HasPrefix(threadID, accountID+":") {
		return threadID
	}
	return accountID + ":" + threadID
}

func referencedChildThread(ctx context.Context, tx *sql.Tx, accountID, messageID string) (string, error) {
	var threadID string
	err := tx.QueryRowContext(ctx, `SELECT m.thread_id FROM mailer_message_refs r
JOIN mailer_messages m ON m.account_id=r.account_id AND m.id=r.message_id
WHERE r.account_id=? AND r.referenced_id=? ORDER BY m.date,m.id LIMIT 1`, accountID, messageID).Scan(&threadID)
	return threadID, err
}

func replaceMessageReferences(ctx context.Context, tx *sql.Tx, message Message) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM mailer_message_refs WHERE account_id=? AND message_id=?", message.AccountID, message.ID); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(message.References)+1)
	values := append(append([]string(nil), message.References...), message.InReplyTo)
	for _, raw := range values {
		ref := cleanMessageID(raw)
		if ref == "" || ref == normalizeMessageID(message.MessageID) {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO mailer_message_refs(account_id,referenced_id,message_id) VALUES(?,?,?)", message.AccountID, ref, message.ID); err != nil {
			return fmt.Errorf("index mail references: %w", err)
		}
	}
	return nil
}

func (s *Store) GetMessage(ctx context.Context, id string) (Message, error) {
	var body, raw []byte
	var threadID string
	err := s.db.QueryRowContext(ctx, "SELECT body,raw,thread_id FROM mailer_messages WHERE id=?", id).Scan(&body, &raw, &threadID)
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, ErrNotFound
	}
	if err != nil {
		return Message{}, fmt.Errorf("read mail message: %w", err)
	}
	var m Message
	if err := unmarshal(body, &m); err != nil {
		return Message{}, err
	}
	m.Raw = raw
	m.ThreadID = threadID
	return m, nil
}

func (s *Store) GetThread(ctx context.Context, threadID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT body,thread_id FROM mailer_messages WHERE thread_id=? ORDER BY date,id", threadID)
	if err != nil {
		return nil, fmt.Errorf("read mail thread: %w", err)
	}
	defer rows.Close()
	messages := make([]Message, 0)
	for rows.Next() {
		var b []byte
		var m Message
		var storedThreadID string
		if err := rows.Scan(&b, &storedThreadID); err != nil {
			return nil, err
		}
		if err := unmarshal(b, &m); err != nil {
			return nil, err
		}
		m.ThreadID = storedThreadID
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, ErrNotFound
	}
	return messages, nil
}

func (s *Store) QueryMessages(ctx context.Context, q MessageQuery) (MessagePage, error) {
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	where, args := messageWhere(q)
	var cursor messagePageCursor
	if q.Cursor != "" {
		if err := decodePageCursor(q.Cursor, &cursor); err != nil || cursor.ID == "" {
			return MessagePage{}, fmt.Errorf("%w: invalid message cursor", ErrInvalid)
		}
	}
	var total int
	join := " FROM mailer_messages m LEFT JOIN mailer_folders f ON f.account_id=m.account_id AND f.id=m.folder_id WHERE " + where
	if err := s.db.QueryRowContext(ctx, "SELECT count(*)"+join, args...).Scan(&total); err != nil {
		return MessagePage{}, fmt.Errorf("count mail messages: %w", err)
	}
	query := "SELECT m.body,m.thread_id" + join
	queryArgs := append([]any(nil), args...)
	if q.Cursor != "" {
		query += " AND (m.date<? OR (m.date=? AND m.id<?))"
		queryArgs = append(queryArgs, cursor.Date, cursor.Date, cursor.ID)
		q.Offset = 0
	}
	query += " ORDER BY m.date DESC,m.id DESC LIMIT ? OFFSET ?"
	queryArgs = append(queryArgs, q.Limit+1, q.Offset)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return MessagePage{}, fmt.Errorf("list mail messages: %w", err)
	}
	defer rows.Close()
	page := MessagePage{Messages: make([]Message, 0), Total: total, Limit: q.Limit, Offset: q.Offset}
	for rows.Next() {
		var b []byte
		var m Message
		var threadID string
		if err := rows.Scan(&b, &threadID); err != nil {
			return MessagePage{}, err
		}
		if err := unmarshal(b, &m); err != nil {
			return MessagePage{}, err
		}
		m.ThreadID = threadID
		page.Messages = append(page.Messages, m)
	}
	if err := rows.Err(); err != nil {
		return MessagePage{}, err
	}
	if len(page.Messages) > q.Limit {
		page.Messages = page.Messages[:q.Limit]
		last := page.Messages[len(page.Messages)-1]
		page.NextCursor = encodePageCursor(messagePageCursor{Date: timeNS(last.Date), ID: last.ID})
	}
	return page, nil
}

type messagePageCursor struct {
	Date int64  `json:"d"`
	ID   string `json:"i"`
}

type threadPageCursor struct {
	Date      int64  `json:"d"`
	AccountID string `json:"a"`
	ThreadID  string `json:"t"`
}

func encodePageCursor(value any) string {
	body, err := marshal(value)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(body)
}

func decodePageCursor(value string, target any) error {
	if len(value) > 8192 {
		return errors.New("page cursor exceeds its size limit")
	}
	body, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return unmarshal(body, target)
}

func messageWhere(q MessageQuery) (string, []any) {
	parts := []string{"1=1"}
	args := make([]any, 0, 8)
	if q.AllowedAccountIDs != nil {
		if len(q.AllowedAccountIDs) == 0 {
			return "0=1", args
		}
		marks := strings.TrimRight(strings.Repeat("?,", len(q.AllowedAccountIDs)), ",")
		parts = append(parts, "m.account_id IN ("+marks+")")
		for _, id := range q.AllowedAccountIDs {
			args = append(args, id)
		}
	}
	if q.AccountID != "" {
		parts = append(parts, "m.account_id=?")
		args = append(args, q.AccountID)
	}
	if q.FolderID != "" {
		parts = append(parts, "m.folder_id=?")
		args = append(args, q.FolderID)
	}
	if q.Query != "" {
		parts = append(parts, `(m.subject LIKE ? ESCAPE '\' OR m.sender LIKE ? ESCAPE '\')`)
		v := "%" + escapeLike(q.Query) + "%"
		args = append(args, v, v)
	}
	if q.Unread || strings.EqualFold(q.View, "unread") {
		parts = append(parts, "m.unread=1")
	}
	if q.Starred || strings.EqualFold(q.View, "starred") {
		parts = append(parts, "m.starred=1")
	}
	if q.Snoozed || strings.EqualFold(q.View, "snoozed") {
		parts = append(parts, "m.snoozed_until>?")
		args = append(args, timeNS(time.Now()))
	} else if !strings.EqualFold(q.View, "all") {
		parts = append(parts, "m.snoozed_until<=?")
		args = append(args, timeNS(time.Now()))
	}
	if strings.EqualFold(q.View, "inbox") {
		parts = append(parts, "lower(f.kind)='inbox'")
	}
	return strings.Join(parts, " AND "), args
}

func escapeLike(s string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(s)
}

func (s *Store) QueryThreads(ctx context.Context, q MessageQuery) ([]Thread, int, error) {
	page, err := s.QueryThreadPage(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	return page.Threads, page.Total, nil
}

func (s *Store) QueryThreadPage(ctx context.Context, q MessageQuery) (ThreadPage, error) {
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	where, args := messageWhere(q)
	join := " FROM mailer_messages m LEFT JOIN mailer_folders f ON f.account_id=m.account_id AND f.id=m.folder_id WHERE " + where
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM (SELECT m.account_id,m.thread_id"+join+" GROUP BY m.account_id,m.thread_id)", args...).Scan(&total); err != nil {
		return ThreadPage{}, fmt.Errorf("count mail threads: %w", err)
	}
	page := ThreadPage{Threads: make([]Thread, 0), Total: total, Limit: q.Limit, Offset: q.Offset}
	query := "SELECT m.account_id,m.thread_id,COUNT(*),SUM(m.unread),MAX(m.date)" + join + " GROUP BY m.account_id,m.thread_id"
	queryArgs := append([]any(nil), args...)
	if q.Cursor != "" {
		var cursor threadPageCursor
		if err := decodePageCursor(q.Cursor, &cursor); err != nil || cursor.AccountID == "" || cursor.ThreadID == "" {
			return ThreadPage{}, fmt.Errorf("%w: invalid thread cursor", ErrInvalid)
		}
		query += " HAVING (MAX(m.date)<? OR (MAX(m.date)=? AND (m.account_id>? OR (m.account_id=? AND m.thread_id>?))))"
		queryArgs = append(queryArgs, cursor.Date, cursor.Date, cursor.AccountID, cursor.AccountID, cursor.ThreadID)
		q.Offset = 0
	}
	query += " ORDER BY MAX(m.date) DESC,m.account_id,m.thread_id LIMIT ? OFFSET ?"
	queryArgs = append(queryArgs, q.Limit+1, q.Offset)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return ThreadPage{}, fmt.Errorf("list mail threads: %w", err)
	}
	type ref struct{ account, id string }
	refs := make([]ref, 0)
	for rows.Next() {
		var t Thread
		var unread int
		var date int64
		if err := rows.Scan(&t.AccountID, &t.ID, &t.Count, &unread, &date); err != nil {
			rows.Close()
			return ThreadPage{}, err
		}
		t.UnreadCount = unread
		t.Unread = unread > 0
		t.LastMessageAt = fromNS(date)
		refs = append(refs, ref{t.AccountID, t.ID})
		page.Threads = append(page.Threads, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ThreadPage{}, err
	}
	_ = rows.Close()
	if len(page.Threads) > q.Limit {
		page.Threads = page.Threads[:q.Limit]
		last := page.Threads[len(page.Threads)-1]
		page.NextCursor = encodePageCursor(threadPageCursor{Date: timeNS(last.LastMessageAt), AccountID: last.AccountID, ThreadID: last.ID})
		refs = refs[:q.Limit]
	}
	for i, r := range refs {
		messages, err := s.getThreadByKey(ctx, r.account, r.id)
		if err != nil {
			return ThreadPage{}, err
		}
		t := &page.Threads[i]
		if len(messages) > 0 {
			latest := messages[len(messages)-1]
			t.Subject = latest.Subject
			t.Preview = messagePreview(latest)
			t.Starred = latest.Starred
		}
		for _, m := range messages {
			for _, p := range append(append(append([]string{}, m.To...), m.CC...), m.From) {
				if p != "" && !contains(t.Participants, p) {
					t.Participants = append(t.Participants, p)
				}
			}
		}
	}
	return page, nil
}

func (s *Store) getThreadByKey(ctx context.Context, accountID, threadID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT body,thread_id FROM mailer_messages WHERE account_id=? AND thread_id=? ORDER BY date,id", accountID, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]Message, 0)
	for rows.Next() {
		var b []byte
		var m Message
		var storedThreadID string
		if err := rows.Scan(&b, &storedThreadID); err != nil {
			return nil, err
		}
		if err := unmarshal(b, &m); err != nil {
			return nil, err
		}
		m.ThreadID = storedThreadID
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func messagePreview(m Message) string {
	v := strings.TrimSpace(m.Text)
	if v == "" {
		v = strings.TrimSpace(m.Subject)
	}
	r := []rune(v)
	if len(r) > 180 {
		return string(r[:180])
	}
	return v
}

func (s *Store) SetSnooze(ctx context.Context, id string, until time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var b []byte
	if err := tx.QueryRowContext(ctx, "SELECT body FROM mailer_messages WHERE id=?", id).Scan(&b); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	var m Message
	if err := unmarshal(b, &m); err != nil {
		return err
	}
	m.SnoozedUntil = until.UTC()
	b, err = marshal(m)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE mailer_messages SET snoozed_until=?,body=? WHERE id=?", timeNS(until), b, id); err != nil {
		return fmt.Errorf("update snooze state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) SetLabels(ctx context.Context, id string, labels []string) error {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT body FROM mailer_messages WHERE id=?", id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var m Message
	if err := unmarshal(b, &m); err != nil {
		return err
	}
	m.Labels = labels
	labelsBlob, err := marshal(labels)
	if err != nil {
		return err
	}
	b, err = marshal(m)
	if err != nil {
		return err
	}
	if _, err = s.db.ExecContext(ctx, "UPDATE mailer_messages SET labels=?,body=? WHERE id=?", labelsBlob, b, id); err != nil {
		return fmt.Errorf("save mail labels: %w", err)
	}
	return nil
}

func (s *Store) SetMessageFlags(ctx context.Context, id string, unread, starred bool) error {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT body FROM mailer_messages WHERE id=?", id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var m Message
	if err := unmarshal(b, &m); err != nil {
		return err
	}
	m.Unread = unread
	m.Starred = starred
	b, err = marshal(m)
	if err != nil {
		return err
	}
	if _, err = s.db.ExecContext(ctx, "UPDATE mailer_messages SET unread=?,starred=?,body=? WHERE id=?", unread, starred, b, id); err != nil {
		return fmt.Errorf("save mail flags: %w", err)
	}
	return nil
}

func (s *Store) UpdateMessage(ctx context.Context, m Message) error {
	labels, err := marshal(m.Labels)
	if err != nil {
		return err
	}
	body, err := marshal(m)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, "UPDATE mailer_messages SET folder_id=?,thread_id=?,subject=?,sender=?,date=?,unread=?,starred=?,labels=?,snoozed_until=?,body=?,raw=CASE WHEN length(?)>0 THEN ? ELSE raw END WHERE id=?", m.FolderID, m.ThreadID, m.Subject, m.From, timeNS(m.Date), m.Unread, m.Starred, labels, timeNS(m.SnoozedUntil), body, nullIfEmpty(m.Raw), nullIfEmpty(m.Raw), m.ID)
	if err != nil {
		return fmt.Errorf("update indexed message: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteMessage(ctx context.Context, id string) error {
	_, _ = s.db.ExecContext(ctx, "DELETE FROM mailer_message_refs WHERE message_id=?", id)
	res, err := s.db.ExecContext(ctx, "DELETE FROM mailer_messages WHERE id=?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteRemoteMessages(ctx context.Context, accountID, folderID string, remoteIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range remoteIDs {
		if _, err := tx.ExecContext(ctx, "DELETE FROM mailer_message_refs WHERE account_id=? AND message_id IN (SELECT id FROM mailer_messages WHERE account_id=? AND folder_id=? AND remote_id=?)", accountID, accountID, folderID, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM mailer_messages WHERE account_id=? AND folder_id=? AND remote_id=?", accountID, folderID, id); err != nil {
			return fmt.Errorf("remove deleted remote message from index: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteFolderMessages(ctx context.Context, accountID, folderID string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM mailer_message_refs WHERE account_id=? AND message_id IN (SELECT id FROM mailer_messages WHERE account_id=? AND folder_id=?)", accountID, accountID, folderID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM mailer_messages WHERE account_id=? AND folder_id=?", accountID, folderID); err != nil {
		return fmt.Errorf("clear stale remote folder index: %w", err)
	}
	return nil
}

func (s *Store) BeginRemoteSnapshot(ctx context.Context, accountID, folderID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM mailer_snapshot_seen WHERE account_id=? AND folder_id=?", accountID, folderID); err != nil {
		return fmt.Errorf("start remote folder snapshot: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO mailer_snapshots(account_id,folder_id,active) VALUES(?,?,1) ON CONFLICT(account_id,folder_id) DO UPDATE SET active=1", accountID, folderID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RemoteSnapshotActive(ctx context.Context, accountID, folderID string) (bool, error) {
	var active bool
	err := s.db.QueryRowContext(ctx, "SELECT active FROM mailer_snapshots WHERE account_id=? AND folder_id=?", accountID, folderID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return active, err
}

func (s *Store) MarkRemoteSnapshotSeen(ctx context.Context, accountID, folderID string, remoteIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, remoteID := range remoteIDs {
		if remoteID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO mailer_snapshot_seen(account_id,folder_id,remote_id) VALUES(?,?,?)", accountID, folderID, remoteID); err != nil {
			return fmt.Errorf("record remote folder snapshot item: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) FinishRemoteSnapshot(ctx context.Context, accountID, folderID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM mailer_messages AS m WHERE m.account_id=? AND m.folder_id=? AND NOT EXISTS (
SELECT 1 FROM mailer_snapshot_seen AS seen WHERE seen.account_id=m.account_id AND seen.folder_id=m.folder_id AND seen.remote_id=m.remote_id)`, accountID, folderID); err != nil {
		return fmt.Errorf("reconcile remote folder snapshot: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM mailer_snapshot_seen WHERE account_id=? AND folder_id=?", accountID, folderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM mailer_snapshots WHERE account_id=? AND folder_id=?", accountID, folderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM mailer_message_refs WHERE account_id=? AND NOT EXISTS (SELECT 1 FROM mailer_messages m WHERE m.id=mailer_message_refs.message_id)", accountID); err != nil {
		return err
	}
	return tx.Commit()
}

func nullIfEmpty(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
