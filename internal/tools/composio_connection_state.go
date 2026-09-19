package tools

import (
	"crypto/sha256"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
)

type composioConnectionSnapshot struct {
	accounts []ComposioConnectedAccount
	complete bool
	expires  time.Time
}

var composioConnections = struct {
	sync.Mutex
	entries map[[32]byte]composioConnectionSnapshot
}{entries: map[[32]byte]composioConnectionSnapshot{}}

func composioConnectionKey(base, key, user, toolkit string) [32]byte {
	return sha256.Sum256([]byte(strings.TrimRight(base, "/") + "\x00" + key + "\x00" + user + "\x00" + toolkit))
}

func (c *ComposioClient) rememberConnections(toolkit, user string, page ComposioListPage[ComposioConnectedAccount]) {
	composioConnections.Lock()
	defer composioConnections.Unlock()
	now := time.Now()
	for key, entry := range composioConnections.entries {
		if now.After(entry.expires) {
			delete(composioConnections.entries, key)
		}
	}
	if len(composioConnections.entries) >= 256 {
		clear(composioConnections.entries)
	}
	composioConnections.entries[composioConnectionKey(c.baseURL, c.apiKey, user, toolkit)] = composioConnectionSnapshot{accounts: append([]ComposioConnectedAccount(nil), page.Items...), complete: page.NextCursor == "", expires: now.Add(c.cacheTTL)}
}

// ComposioCachedConnectionState performs no network or authorization operation.
func ComposioCachedConnectionState(cfg config.ComposioConfig, toolkit string) string {
	client := NewComposioClientFromConfig(cfg)
	composioConnections.Lock()
	defer composioConnections.Unlock()
	snapshot, ok := composioConnections.entries[composioConnectionKey(client.baseURL, client.apiKey, cfg.UserID, toolkit)]
	if !ok {
		snapshot, ok = composioConnections.entries[composioConnectionKey(client.baseURL, client.apiKey, cfg.UserID, "")]
	}
	if !ok || time.Now().After(snapshot.expires) {
		return "connection_unknown"
	}
	preferred := ""
	for _, selected := range cfg.Toolkits {
		if selected.Slug == toolkit {
			preferred = selected.PreferredConnectedAccountID
		}
	}
	for _, account := range snapshot.accounts {
		if account.ToolkitSlug == toolkit && strings.EqualFold(account.Status, "active") && (preferred == "" || account.ID == preferred) {
			return "connected"
		}
	}
	if snapshot.complete {
		return "connect_required"
	}
	return "connection_unknown"
}
