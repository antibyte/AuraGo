package meshcore

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SendPart struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Tag    uint32 `json:"-"`
}

// Change is metadata only; it is safe to broadcast through the desktop hub.
type Change struct {
	ConversationID string `json:"conversation_id,omitempty"`
	MessageID      string `json:"message_id,omitempty"`
	Incoming       bool   `json:"incoming"`
	Muted          bool   `json:"muted"`
}

type Conversation struct {
	ID          string `json:"id"`
	IdentityKey string `json:"identity_key"`
	Kind        string `json:"kind"`
	Target      string `json:"target"`
	Channel     int    `json:"channel"`
	Name        string `json:"name"`
	Type        byte   `json:"type"`
	ChannelKind string `json:"channel_kind,omitempty"`
	Favorite    bool   `json:"favorite"`
	Muted       bool   `json:"muted"`
	Unread      int    `json:"unread"`
	LastSeq     int64  `json:"last_seq"`
	LastAt      int64  `json:"last_at"`
	Preview     string `json:"preview"`
	Protected   bool   `json:"protected"`
	Active      bool   `json:"active"`
	CanSend     bool   `json:"can_send"`
}

type ChatMessage struct {
	ID             string     `json:"id"`
	Seq            int64      `json:"seq"`
	ConversationID string     `json:"conversation_id"`
	Direction      string     `json:"direction"`
	Origin         string     `json:"origin"`
	Text           string     `json:"text"`
	At             int64      `json:"at"`
	Review         string     `json:"review"`
	Protected      bool       `json:"protected"`
	SendState      string     `json:"send_state"`
	Parts          []SendPart `json:"parts"`
}

func conversationID(identity, kind, target string) string {
	h := sha256.Sum256([]byte(identity + "\x00" + kind + "\x00" + target))
	return hex.EncodeToString(h[:])
}

func messageConversation(m Message) Conversation {
	kind, target := m.Kind, m.PeerKey
	if kind == "channel" {
		target = m.Binding
	} else if target == "" && ValidKey(m.Sender) {
		target = m.Sender
	}
	if target == "" {
		kind, target = "unknown", m.Sender
	}
	return Conversation{ID: conversationID(m.IdentityKey, kind, target), IdentityKey: m.IdentityKey, Kind: kind, Target: target, Channel: m.Channel, Name: target}
}

func (s *store) migrateMessenger(dir, version string) error {
	if version != "1" && version != "2" {
		return fmt.Errorf("unsupported meshcore schema")
	}
	if version == "1" {
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM meshcore_messages").Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			backup := filepath.Join(dir, fmt.Sprintf("meshcore-v1-%d.backup.db", time.Now().UnixNano()))
			if _, err := s.db.Exec("VACUUM INTO ?", backup); err != nil {
				return fmt.Errorf("back up meshcore database: %w", err)
			}
			if err := os.Chmod(backup, 0600); err != nil {
				return err
			}
		}
	}
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS meshcore_conversations (id TEXT PRIMARY KEY, data BLOB NOT NULL, favorite INTEGER NOT NULL DEFAULT 0, muted INTEGER NOT NULL DEFAULT 0, read_seq INTEGER NOT NULL DEFAULT 0, cleared_seq INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS meshcore_chat (seq INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL UNIQUE, conversation TEXT NOT NULL, incoming INTEGER NOT NULL, at INTEGER NOT NULL, text TEXT NOT NULL, protected INTEGER NOT NULL, data BLOB NOT NULL);
CREATE INDEX IF NOT EXISTS meshcore_chat_conversation ON meshcore_chat(conversation,seq);
CREATE TABLE IF NOT EXISTS meshcore_send_keys (id TEXT PRIMARY KEY, fingerprint TEXT NOT NULL, created INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS meshcore_send_parts (message TEXT NOT NULL, number INTEGER NOT NULL, identity TEXT NOT NULL, session TEXT NOT NULL, tag INTEGER NOT NULL DEFAULT 0, created INTEGER NOT NULL, state TEXT NOT NULL, PRIMARY KEY(message,number));
`)
	if err != nil {
		return err
	}
	if version == "1" {
		var legacy []Message
		for offset := 0; ; offset += 100 {
			messages, err := s.list(100, offset)
			if err != nil {
				return err
			}
			legacy = append(legacy, messages...)
			if len(messages) < 100 {
				break
			}
		}
		// Sequence order must agree with chronology, including across old pages.
		sort.Slice(legacy, func(i, j int) bool {
			if legacy[i].ReceivedAt == legacy[j].ReceivedAt {
				return legacy[i].ID < legacy[j].ID
			}
			return legacy[i].ReceivedAt < legacy[j].ReceivedAt
		})
		for _, m := range legacy {
			tx, err := s.db.Begin()
			if err != nil {
				return err
			}
			if err = projectChat(tx, m); err != nil {
				tx.Rollback()
				return err
			}
			if err = tx.Commit(); err != nil {
				return err
			}
		}
	}
	_, err = s.db.Exec("UPDATE meshcore_meta SET value='2' WHERE key='version'; UPDATE meshcore_send_parts SET state='outcome_unknown' WHERE state='sending'")
	return err
}

// projectChat commits the sanitized history projection with the security inbox.
// Unreviewed bodies stay exclusively in the short-lived protected inbox.
func projectChat(tx *sql.Tx, m Message) error {
	c := messageConversation(m)
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT OR IGNORE INTO meshcore_conversations(id,data) VALUES(?,?)", c.ID, b); err != nil {
		return err
	}
	origin := m.Origin
	if origin == "" {
		if m.Direction == "outgoing" {
			origin = "agent"
		} else {
			origin = "radio"
		}
	}
	msg := ChatMessage{ID: m.ID, ConversationID: c.ID, Direction: m.Direction, Origin: origin, Text: m.Text, At: m.ReceivedAt, Review: m.Review, SendState: m.SendState, Parts: m.Parts}
	msg.Protected = m.Direction != "outgoing" && m.Review != "safe"
	if msg.Protected {
		msg.Text = ""
	}
	if m.Direction == "outgoing" && (m.State == "sending" || m.State == "queued" || m.State == "outcome_unknown") {
		msg.SendState = m.State
	}
	if err = putChat(tx, msg); err != nil {
		return err
	}
	if m.Reply != "" && (m.SendState != "" || m.State == "sending") {
		msg.ID = m.ID + ":reply"
		msg.Direction = "outgoing"
		msg.Origin = "agent"
		msg.Protected = false
		msg.Text = m.Reply
		msg.Review = ""
		if m.State == "sending" {
			msg.SendState = "sending"
		}
		return putChat(tx, msg)
	}
	return nil
}

func putChat(tx *sql.Tx, m ChatMessage) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO meshcore_chat(id,conversation,incoming,at,text,protected,data) VALUES(?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET text=excluded.text,protected=excluded.protected,data=excluded.data
WHERE meshcore_chat.seq>(SELECT cleared_seq FROM meshcore_conversations WHERE id=meshcore_chat.conversation)`, m.ID, m.ConversationID, m.Direction == "incoming", m.At, m.Text, m.Protected, b)
	return err
}

func (s *store) syncConversations(st Status) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	items := []Conversation{}
	for _, contact := range st.Contacts {
		if contact.Key == st.IdentityKey {
			continue
		}
		items = append(items, Conversation{ID: conversationID(st.IdentityKey, "direct", contact.Key), IdentityKey: st.IdentityKey, Kind: "direct", Target: contact.Key, Channel: -1, Name: contact.Name, Type: contact.Type})
	}
	for _, ch := range st.Channels {
		items = append(items, Conversation{ID: conversationID(st.IdentityKey, "channel", ch.Binding), IdentityKey: st.IdentityKey, Kind: "channel", Target: ch.Binding, Channel: ch.Index, Name: ch.Name, ChannelKind: ch.Kind})
	}
	for _, c := range items {
		b, _ := json.Marshal(c)
		if _, err = tx.Exec("INSERT INTO meshcore_conversations(id,data) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data", c.ID, b); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *store) conversation(id string) (Conversation, error) {
	var c Conversation
	var b []byte
	err := s.db.QueryRow("SELECT data,favorite,muted FROM meshcore_conversations WHERE id=?", id).Scan(&b, &c.Favorite, &c.Muted)
	if err != nil {
		return c, err
	}
	favorite, muted := c.Favorite, c.Muted
	err = json.Unmarshal(b, &c)
	c.Favorite, c.Muted = favorite, muted
	return c, err
}

func (m *Manager) Conversations() ([]Conversation, error) {
	st := m.Status()
	m.mu.Lock()
	cfg := m.cfg
	m.mu.Unlock()
	if cfg.HistoryDays == 0 {
		cfg.HistoryDays = 90
	}
	if cfg.HistoryMessages == 0 {
		cfg.HistoryMessages = 10000
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = 7
	}
	if cfg.MaxMessages == 0 {
		cfg.MaxMessages = 1000
	}
	if err := m.store.prune(cfg); err != nil {
		return nil, err
	}
	rows, err := m.store.db.Query(`SELECT c.data,c.favorite,c.muted,
(SELECT COUNT(*) FROM meshcore_chat h WHERE h.conversation=c.id AND h.incoming=1 AND h.seq>MAX(c.read_seq,c.cleared_seq)),
COALESCE(h.seq,0),COALESCE(h.at,0),COALESCE(h.text,''),COALESCE(h.protected,0)
FROM meshcore_conversations c LEFT JOIN meshcore_chat h ON h.seq=(SELECT MAX(seq) FROM meshcore_chat WHERE conversation=c.id AND seq>c.cleared_seq)
ORDER BY c.favorite DESC,COALESCE(h.seq,0) DESC,c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Conversation{}
	for rows.Next() {
		var c Conversation
		var b []byte
		if err = rows.Scan(&b, &c.Favorite, &c.Muted, &c.Unread, &c.LastSeq, &c.LastAt, &c.Preview, &c.Protected); err != nil {
			return nil, err
		}
		var meta Conversation
		if err = json.Unmarshal(b, &meta); err != nil {
			return nil, err
		}
		c.ID, c.IdentityKey, c.Kind, c.Target, c.Channel, c.Name, c.Type, c.ChannelKind = meta.ID, meta.IdentityKey, meta.Kind, meta.Target, meta.Channel, meta.Name, meta.Type, meta.ChannelKind
		c.Active = c.IdentityKey == st.IdentityKey
		if c.Active && c.Kind == "direct" {
			contact, ok := uniqueContact(st, c.Target)
			c.Active = ok && contact.Key == c.Target
			c.CanSend = c.Active && contact.Type == 1
		}
		if c.Active && c.Kind == "channel" {
			c.Active = channelMatches(st, ChannelRule{Index: c.Channel, Binding: c.Target})
			c.CanSend = c.Active
		}
		c.CanSend = c.CanSend && st.State == "connected"
		if len([]rune(c.Preview)) > 100 {
			c.Preview = string([]rune(c.Preview)[:100]) + "…"
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (m *Manager) ChatMessages(id string, before int64, query string) ([]ChatMessage, error) {
	if !ValidKey(id) || before < 0 || len(query) > 256 {
		return nil, fmt.Errorf("invalid_request")
	}
	rows, err := m.store.db.Query(`SELECT h.seq,h.data FROM meshcore_chat h JOIN meshcore_conversations c ON c.id=h.conversation
WHERE h.conversation=? AND h.seq>c.cleared_seq AND (?=0 OR h.seq<?) AND (?='' OR instr(lower(h.text),lower(?))>0) ORDER BY h.seq DESC LIMIT 50`, id, before, before, query, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ChatMessage{}
	for rows.Next() {
		var msg ChatMessage
		var b []byte
		var seq int64
		if err = rows.Scan(&seq, &b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &msg); err != nil {
			return nil, err
		}
		msg.Seq = seq
		items = append(items, msg)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range items {
		if err = m.store.loadParts(&items[i]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (m *Manager) UpdateConversation(id string, read int64, favorite, muted *bool, clear bool) error {
	if !ValidKey(id) || read < 0 {
		return fmt.Errorf("invalid_request")
	}
	tx, err := m.store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if favorite != nil {
		if _, err = tx.Exec("UPDATE meshcore_conversations SET favorite=? WHERE id=?", *favorite, id); err != nil {
			return err
		}
	}
	if muted != nil {
		if _, err = tx.Exec("UPDATE meshcore_conversations SET muted=? WHERE id=?", *muted, id); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("UPDATE meshcore_conversations SET read_seq=MAX(read_seq,MIN(?,COALESCE((SELECT MAX(seq) FROM meshcore_chat WHERE conversation=?),0))) WHERE id=?", read, id, id); err != nil {
		return err
	}
	if clear {
		if _, err = tx.Exec("UPDATE meshcore_conversations SET cleared_seq=COALESCE((SELECT MAX(seq) FROM meshcore_chat WHERE conversation=?),0) WHERE id=?", id, id); err != nil {
			return err
		}
		// Keep IDs and the cursor so a late review/ACK cannot restore cleared text.
		// The short-lived security inbox and its execution reservations are separate.
		if _, err = tx.Exec("UPDATE meshcore_chat SET text='',data=json_set(data,'$.text','') WHERE conversation=?", id); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err == nil {
		m.changed(Change{ConversationID: id})
	}
	return err
}

func (m *Manager) RevealMessage(id string) (string, error) {
	msg, err := m.store.get(id)
	if err != nil {
		return "", fmt.Errorf("message_unavailable")
	}
	return m.scrub(msg.Text), nil
}

func (s *store) pruneChat(c Config) error {
	_, err := s.db.Exec(`DELETE FROM meshcore_chat WHERE protected=1 AND id NOT IN (SELECT id FROM meshcore_messages);
DELETE FROM meshcore_chat WHERE seq<=(SELECT cleared_seq FROM meshcore_conversations WHERE id=conversation) AND id NOT IN (SELECT id FROM meshcore_messages) AND replace(id,':reply','') NOT IN (SELECT id FROM meshcore_messages);
DELETE FROM meshcore_send_parts WHERE message NOT IN (SELECT id FROM meshcore_chat);`)
	if err != nil {
		return err
	}
	// Cleared rows are body-free tombstones until their source inbox expires.
	// This also prevents an administrative recheck from resurrecting deleted text.
	_, err = s.db.Exec(`DELETE FROM meshcore_chat WHERE at<? AND id NOT IN (SELECT id FROM meshcore_messages WHERE state IN ('sending','queued'))
AND NOT (seq<=(SELECT cleared_seq FROM meshcore_conversations WHERE id=conversation) AND (id IN (SELECT id FROM meshcore_messages) OR replace(id,':reply','') IN (SELECT id FROM meshcore_messages)))`, time.Now().Add(-time.Duration(c.HistoryDays)*24*time.Hour).Unix())
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM meshcore_chat WHERE seq IN (SELECT h.seq FROM meshcore_chat h JOIN meshcore_conversations c ON c.id=h.conversation
WHERE h.seq>c.cleared_seq AND h.id NOT IN (SELECT id FROM meshcore_messages WHERE state IN ('sending','queued')) ORDER BY h.seq
LIMIT MAX(0,(SELECT COUNT(*) FROM meshcore_chat h JOIN meshcore_conversations c ON c.id=h.conversation WHERE h.seq>c.cleared_seq)-?))`, c.HistoryMessages)
	return err
}

func (m *Manager) changed(change Change) {
	if m.hooks.Changed != nil {
		m.hooks.Changed(change)
	}
}

func (m *Manager) chatChanged(msg Message, incoming bool) {
	c := messageConversation(msg)
	if stored, err := m.store.conversation(c.ID); err == nil {
		c.Muted = stored.Muted
	}
	m.changed(Change{ConversationID: c.ID, MessageID: msg.ID, Incoming: incoming, Muted: c.Muted})
}

func validRequestID(id string) bool {
	if len(id) < 16 || len(id) > 80 {
		return false
	}
	return strings.IndexFunc(id, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-')
	}) < 0
}

func (s *store) recoverChat() error {
	rows, err := s.db.Query("SELECT id,data FROM meshcore_chat WHERE incoming=0")
	if err != nil {
		return err
	}
	updates := []ChatMessage{}
	for rows.Next() {
		var id string
		var b []byte
		if err = rows.Scan(&id, &b); err != nil {
			rows.Close()
			return err
		}
		var msg ChatMessage
		if err = json.Unmarshal(b, &msg); err != nil {
			rows.Close()
			return err
		}
		if msg.SendState == "sending" || msg.SendState == "queued" {
			msg.SendState = "outcome_unknown"
			for i := range msg.Parts {
				if msg.Parts[i].State == "sending" {
					msg.Parts[i].State = "outcome_unknown"
				}
			}
			updates = append(updates, msg)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, msg := range updates {
		if err = putChat(tx, msg); err != nil {
			return err
		}
	}
	return tx.Commit()
}
