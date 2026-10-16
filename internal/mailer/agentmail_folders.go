package mailer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/agentmail"
)

const (
	agentMailFolderDiscoveryTTL = 30 * time.Second
	maxAgentMailFolderListPages = 1000
)

type agentMailFolderCacheEntry struct {
	expires time.Time
	labels  []string
}

var agentMailFolderCache = struct {
	sync.Mutex
	entries map[string]agentMailFolderCacheEntry
}{entries: make(map[string]agentMailFolderCacheEntry)}

func (a *agentMailAdapter) discoverAgentMailFolderLabels(ctx context.Context) ([]string, error) {
	key := a.cfg.AgentMailBaseURL + "\x00" + a.cfg.AgentMailInboxID
	now := time.Now()
	agentMailFolderCache.Lock()
	if entry, ok := agentMailFolderCache.entries[key]; ok && now.Before(entry.expires) {
		labels := append([]string(nil), entry.labels...)
		agentMailFolderCache.Unlock()
		return labels, nil
	}
	agentMailFolderCache.Unlock()

	labels := make(map[string]struct{})
	cursor := ""
	seenCursors := make(map[string]struct{})
	for page := 0; page < maxAgentMailFolderListPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		res, err := a.client.ListMessages(ctx, a.cfg.AgentMailInboxID, agentmail.ListMessagesOptions{
			Limit: 100, Cursor: cursor,
		})
		if err != nil {
			return nil, fmt.Errorf("discover AgentMail labels: %w", err)
		}
		for _, message := range res.Messages {
			for _, raw := range message.Labels {
				label := strings.TrimSpace(raw)
				if !validMailLabel(label) || agentMailSystemLabel(label) {
					continue
				}
				labels[label] = struct{}{}
			}
		}
		next := res.NextCursor
		if next == "" {
			if res.HasMore {
				return nil, fmt.Errorf("discover AgentMail labels: provider indicated more pages without a page token")
			}
			ordered := make([]string, 0, len(labels))
			for label := range labels {
				ordered = append(ordered, label)
			}
			sort.Strings(ordered)
			agentMailFolderCache.Lock()
			agentMailFolderCache.entries[key] = agentMailFolderCacheEntry{expires: time.Now().Add(agentMailFolderDiscoveryTTL), labels: append([]string(nil), ordered...)}
			agentMailFolderCache.Unlock()
			return ordered, nil
		}
		if _, exists := seenCursors[next]; exists {
			return nil, fmt.Errorf("discover AgentMail labels: provider repeated a page token")
		}
		seenCursors[next] = struct{}{}
		cursor = next
	}
	return nil, fmt.Errorf("discover AgentMail labels: exceeded %d page limit", maxAgentMailFolderListPages)
}

func invalidateAgentMailFolderCache(cfg TransportConfig) {
	key := cfg.AgentMailBaseURL + "\x00" + cfg.AgentMailInboxID
	agentMailFolderCache.Lock()
	delete(agentMailFolderCache.entries, key)
	agentMailFolderCache.Unlock()
}

func agentMailSystemLabel(label string) bool {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "inbox", "received", "sent", "read", "unread", "starred", "unstarred", "bounced", "delivered", "complained", "rejected", "spam", "trash", "blocked", "unauthenticated", "opened":
		return true
	default:
		return false
	}
}

func agentMailProviderLabel(label string) string {
	if strings.EqualFold(label, "inbox") {
		return "received"
	}
	return label
}

func agentMailMessageInFolder(message agentmail.Message, folder string) bool {
	labels := make(map[string]struct{}, len(message.Labels))
	for _, label := range message.Labels {
		labels[strings.ToLower(strings.TrimSpace(label))] = struct{}{}
	}
	has := func(label string) bool {
		_, ok := labels[strings.ToLower(label)]
		return ok
	}
	switch strings.ToLower(folder) {
	case "inbox":
		return has("received") && !has("sent") && !has("archive") && !has("trash") && !has("spam") && !has("junk")
	case "sent":
		return has("sent")
	case "archive":
		return has("archive") && !has("trash")
	case "trash":
		return has("trash")
	case "spam":
		return has("spam")
	case "junk":
		return has("junk") && !has("trash")
	default:
		return has(folder) && !has("trash") && !has("spam")
	}
}
