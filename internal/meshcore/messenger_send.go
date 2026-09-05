package meshcore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type sendOrigin byte

func (m *Manager) ChannelTextLimit() int {
	st := m.Status()
	return min(133, 160-max(len(st.Name), st.nameBytes)-2)
}

const (
	sendAgent sendOrigin = iota
	sendReply
	sendHuman
)

// SendManual is administrative application input, deliberately absent from the
// agent tool. The target is resolved from a persisted conversation, not a name.
func (m *Manager) SendManual(ctx context.Context, id, conversation, text string) (string, error) {
	if !validRequestID(id) || !ValidKey(conversation) {
		return "", fmt.Errorf("invalid_request")
	}
	text = m.scrub(strings.TrimSpace(text))
	if _, err := splitText(text, 133); err != nil {
		return "", fmt.Errorf("invalid_text")
	}
	id = "manual-" + id
	h := sha256.Sum256([]byte(conversation + "\x00" + text))
	fingerprint := hex.EncodeToString(h[:])
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	var previous string
	err := m.store.db.QueryRow("SELECT fingerprint FROM meshcore_send_keys WHERE id=?", id).Scan(&previous)
	if err == nil {
		if previous != fingerprint {
			return "", fmt.Errorf("idempotency_conflict")
		}
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	m.mu.Lock()
	c, cfg, closed := m.conn, m.cfg, m.closed
	m.mu.Unlock()
	if closed || c == nil || !cfg.Enabled {
		return "", fmt.Errorf("not_connected")
	}
	st, err := m.refresh(ctx, c)
	if err != nil || st.State != "connected" {
		return "", fmt.Errorf("binding_required")
	}
	conv, err := m.store.conversation(conversation)
	if err != nil {
		return "", fmt.Errorf("invalid_target")
	}
	if conv.IdentityKey != st.IdentityKey {
		return "", fmt.Errorf("binding_required")
	}
	if conv.Kind == "direct" {
		contact, ok := uniqueContact(st, conv.Target)
		if !ok || contact.Key != conv.Target || contact.Type != 1 || contact.Key == st.IdentityKey {
			return "", fmt.Errorf("invalid_target")
		}
	} else if conv.Kind != "channel" || !channelMatches(st, ChannelRule{Index: conv.Channel, Binding: conv.Target}) {
		return "", fmt.Errorf("invalid_target")
	}
	limit := 133
	if conv.Kind == "channel" {
		limit = min(limit, 160-max(len(st.Name), st.nameBytes)-2)
	}
	if _, err = splitText(text, limit); err != nil {
		return "", fmt.Errorf("invalid_text")
	}
	select {
	case m.manualSlots <- struct{}{}:
	default:
		return "", fmt.Errorf("busy")
	}
	reserved := false
	defer func() {
		if !reserved {
			<-m.manualSlots
		}
	}()
	msg := Message{ID: id, IdentityKey: st.IdentityKey, Kind: conv.Kind, Sender: conv.Target, PeerKey: conv.Target, Channel: conv.Channel, Binding: conv.Target, Text: text, Direction: "outgoing", Origin: "manual", ReceivedAt: time.Now().Unix(), State: "sending", SendState: "sending"}
	b, _ := json.Marshal(msg)
	tx, err := m.store.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRow("SELECT COUNT(*) FROM meshcore_send_keys").Scan(&count); err != nil {
		return "", err
	}
	if count >= 65536 {
		// ponytail: permanent idempotency tombstones have a 65K safety ceiling;
		// add archival maintenance if an installation reaches this volume.
		return "", fmt.Errorf("send_ledger_full")
	}
	if _, err = tx.Exec("INSERT INTO meshcore_send_keys(id,fingerprint,created) VALUES(?,?,?)", id, fingerprint, msg.ReceivedAt); err != nil {
		return "", err
	}
	if _, err = tx.Exec("INSERT INTO meshcore_messages(id,received,state,data,binding) VALUES(?,?,?,?,?)", id, msg.ReceivedAt, msg.State, b, msg.Binding); err != nil {
		return "", err
	}
	if err = projectChat(tx, msg); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	reserved = true
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() { <-m.manualSlots }()
		ctx, cancel := context.WithTimeout(m.root, 4*time.Minute)
		defer cancel()
		state, _ := m.sendMessage(ctx, msg, text, sendHuman, c)
		msg.SendState = state
		msg.State = "completed"
		if state == "outcome_unknown" {
			msg.State = state
		}
		m.save(msg)
		m.mu.Lock()
		cfg := m.cfg
		m.mu.Unlock()
		if err := m.store.prune(cfg); err != nil {
			m.issue("persistence", true)
		}
	}()
	m.chatChanged(msg, false)
	return id, nil
}

func (m *Manager) prepareParts(id, identity string, c *companion, parts []string) error {
	tx, err := m.store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i := range parts {
		if _, err = tx.Exec("INSERT OR IGNORE INTO meshcore_send_parts(message,number,identity,session,created,state) VALUES(?,?,?,?,?,'not_sent')", id, i+1, identity, c.session, time.Now().Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (m *Manager) recordPart(id string, number int, state string, tag uint32, c *companion) error {
	_, err := m.store.db.Exec("UPDATE meshcore_send_parts SET state=CASE WHEN state='delivered' THEN state ELSE ? END,tag=? WHERE message=? AND number=? AND session=?", state, tag, id, number, c.session)
	if err == nil {
		m.partChanged(id)
	}
	return err
}

func (m *Manager) partChanged(id string) {
	var conversation string
	if err := m.store.db.QueryRow("SELECT conversation FROM meshcore_chat WHERE id=?", id).Scan(&conversation); err == nil {
		m.changed(Change{ConversationID: conversation, MessageID: id})
	}
}

func (m *Manager) lateACK(c *companion, tag uint32) {
	var id string
	var number, count int
	err := m.store.db.QueryRow("SELECT COALESCE(MIN(message),''),COALESCE(MIN(number),0),COUNT(*) FROM meshcore_send_parts WHERE session=? AND tag=? AND created>=? AND state IN ('device_accepted','outcome_unknown')", c.session, tag, time.Now().Add(-10*time.Minute).Unix()).Scan(&id, &number, &count)
	if err == nil && count == 1 {
		_ = m.recordPart(id, number, "delivered", tag, c)
	}
}

func (s *store) loadParts(msg *ChatMessage) error {
	rows, err := s.db.Query("SELECT number,state FROM meshcore_send_parts WHERE message=? ORDER BY number", msg.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	parts := []SendPart{}
	for rows.Next() {
		var p SendPart
		if err = rows.Scan(&p.Number, &p.State); err != nil {
			return err
		}
		parts = append(parts, p)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(parts) == 0 {
		return nil
	}
	msg.Parts = parts
	if msg.SendState == "sending" {
		return nil
	}
	allDelivered, allAccepted, anySent := true, true, false
	for _, p := range parts {
		allDelivered = allDelivered && p.State == "delivered"
		accepted := p.State == "delivered" || p.State == "device_accepted"
		allAccepted = allAccepted && accepted
		anySent = anySent || accepted || p.State == "sending" || p.State == "outcome_unknown"
	}
	if allDelivered {
		msg.SendState = "delivered"
	} else if allAccepted {
		msg.SendState = "device_accepted"
	} else if anySent {
		msg.SendState = "outcome_unknown"
	} else {
		msg.SendState = "not_sent"
	}
	return nil
}
