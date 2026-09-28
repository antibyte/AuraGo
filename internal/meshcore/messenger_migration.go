package meshcore

import "encoding/json"

// Backfill only surviving, uncleared rows with their original identity/binding.
// In particular, do not project inbox messages again: that resurrects history.
func (s *store) migrateChatDetails() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`ALTER TABLE meshcore_send_parts ADD COLUMN route INTEGER;
ALTER TABLE meshcore_send_parts ADD COLUMN ack_millis INTEGER;`); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT c.id,c.conversation,c.data,m.data,m.binding FROM meshcore_chat c
JOIN meshcore_messages m ON m.id=c.id JOIN meshcore_conversations v ON v.id=c.conversation
WHERE c.incoming=1 AND c.seq>v.cleared_seq`)
	if err != nil {
		return err
	}
	type update struct {
		id   string
		data []byte
	}
	var updates []update
	for rows.Next() {
		var id, conversation, binding string
		var chat, inbox []byte
		if err = rows.Scan(&id, &conversation, &chat, &inbox, &binding); err != nil {
			break
		}
		var c ChatMessage
		var m Message
		if json.Unmarshal(chat, &c) != nil || json.Unmarshal(inbox, &m) != nil {
			continue
		}
		m.Binding = binding
		if c.ID != id || m.ID != id || c.ConversationID != conversation || messageConversation(m).ID != conversation || m.Direction == "outgoing" {
			continue
		}
		c.Details = chatDetails(m)
		if c.Protected {
			c.Details.SenderLabel = ""
		}
		b, marshalErr := json.Marshal(c)
		if marshalErr != nil {
			err = marshalErr
			break
		}
		updates = append(updates, update{id, b})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, u := range updates {
		if _, err = tx.Exec("UPDATE meshcore_chat SET data=? WHERE id=?", u.data, u.id); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("UPDATE meshcore_meta SET value='3' WHERE key='version'"); err != nil {
		return err
	}
	return tx.Commit()
}
