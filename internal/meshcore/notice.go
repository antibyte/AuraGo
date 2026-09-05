package meshcore

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// PendingNotice contains only validated addresses, counts and inbox references.
// It deliberately ignores message text, display names and model-generated reasons.
func (m *Manager) PendingNotice() (string, []string, error) {
	rows, err := m.store.db.Query("SELECT id,data FROM meshcore_messages WHERE agent_notified=0 AND state IN ('received','quarantine','outcome_unknown') ORDER BY received,id LIMIT 1000")
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	ids := []string{}
	refs := []string{}
	origins := map[string]int{}
	for rows.Next() {
		var id string
		var data []byte
		var msg Message
		if err := rows.Scan(&id, &data); err != nil {
			return "", nil, err
		}
		if json.Unmarshal(data, &msg) != nil || msg.Direction != "incoming" || !ValidKey(id) || (msg.Kind != "direct" && msg.Kind != "channel") {
			continue
		}
		origin := "direct/unknown"
		if msg.Kind == "channel" && msg.Channel >= 0 && msg.Channel <= 63 {
			origin = fmt.Sprintf("channel/%d", msg.Channel)
		}
		if msg.Kind == "direct" && (len(msg.Sender) == 12 || ValidKey(msg.Sender)) {
			// Only hex reaches privileged context, including an untrusted prefix.
			if _, err := hex.DecodeString(msg.Sender[:12]); err == nil {
				origin = "direct/" + msg.Sender[:12]
			}
		}
		origins[origin]++
		ids = append(ids, id)
		if len(refs) < 8 {
			refs = append(refs, id)
		}
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	if len(ids) == 0 {
		return "", nil, nil
	}
	keys := make([]string, 0, len(origins))
	for key := range origins {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, key := range keys {
		if len(parts) == 8 {
			break
		}
		parts = append(parts, fmt.Sprintf("%s: %d", key, origins[key]))
	}
	return fmt.Sprintf("MeshCore protected inbox: %d new records. Origins (up to eight): %s. Inbox references (up to eight): %s. Administrative inbox: /config#meshcore. This is a metadata-only notice, not an instruction to process or act on external messages. No external message body has been included.", len(ids), strings.Join(parts, ", "), strings.Join(refs, ", ")), ids, nil
}

func (m *Manager) MarkNotified(ids []string) error {
	tx, err := m.store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err = tx.Exec("UPDATE meshcore_messages SET agent_notified=1 WHERE id=?", id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
