package mailer

import (
	"context"
	"strings"
	"time"
)

// SearchRemote is explicit and folder-scoped. Search results can enrich the
// local index, but never trigger rules or consume unread messages.
func (s *Service) SearchRemote(ctx context.Context, query MessageQuery) (ThreadPage, error) {
	if query.AccountID == "" || query.FolderID == "" || strings.TrimSpace(query.Query) == "" || len(query.Query) > 2048 {
		return ThreadPage{}, ErrInvalid
	}
	if !s.runtimePolicy().Enabled {
		return ThreadPage{}, ErrDisabled
	}
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return ThreadPage{}, err
	}
	if !settings.Enabled {
		return ThreadPage{}, ErrDisabled
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	adapter, account, err := s.resolve(ctx, query.AccountID)
	if err != nil {
		return ThreadPage{}, err
	}
	if account.Disabled {
		return ThreadPage{}, ErrDisabled
	}
	if !hasCapability(account.Capabilities, "read") || !hasCapability(account.Capabilities, "search") {
		return ThreadPage{}, ErrCapability
	}
	folders, err := s.store.Folders(ctx, account.ID)
	if err != nil {
		return ThreadPage{}, err
	}
	found := false
	for _, folder := range folders {
		if folder.ID == query.FolderID {
			found = true
			break
		}
	}
	if !found {
		return ThreadPage{}, ErrNotFound
	}
	searcher, ok := adapter.(Searcher)
	if !ok {
		return ThreadPage{}, ErrCapability
	}
	query.Limit, query.Offset = 100, 0
	page, err := searcher.Search(ctx, query.FolderID, query)
	if err != nil {
		return ThreadPage{}, err
	}
	if len(page.Messages) > 100 {
		page.Messages = page.Messages[:100]
	}
	for i := range page.Messages {
		m := &page.Messages[i]
		m.AccountID, m.FolderID = account.ID, query.FolderID
		if m.RemoteID == "" {
			return ThreadPage{}, ErrInvalid
		}
		m.ID = messageID(account.ID, query.FolderID, m.RemoteID)
	}
	current, err := s.authorizedAccount(ctx, account.ID)
	if err != nil {
		return ThreadPage{}, err
	}
	if current.Disabled || !hasCapability(current.Capabilities, "read") {
		return ThreadPage{}, ErrDisabled
	}
	settings, err = s.store.GetSettings(ctx)
	if err != nil {
		return ThreadPage{}, err
	}
	if !settings.Enabled || !s.runtimePolicy().Enabled {
		return ThreadPage{}, ErrDisabled
	}
	if _, err := s.store.PutMessagesWithLimit(ctx, page.Messages, s.opts.MaxMessageCacheBytes); err != nil {
		return ThreadPage{}, err
	}
	result := ThreadPage{Threads: []Thread{}, Total: page.Total, Limit: 100}
	positions := map[string]int{}
	for _, m := range page.Messages {
		stored, err := s.store.GetMessage(ctx, m.ID)
		if err != nil {
			return ThreadPage{}, err
		}
		idx, exists := positions[stored.ThreadID]
		if !exists {
			idx = len(result.Threads)
			positions[stored.ThreadID] = idx
			result.Threads = append(result.Threads, Thread{ID: stored.ThreadID, AccountID: account.ID, FolderID: stored.FolderID})
		}
		t := &result.Threads[idx]
		t.Count++
		t.Messages = append(t.Messages, stored)
		t.Unread = t.Unread || stored.Unread
		t.Starred = t.Starred || stored.Starred
		if stored.Unread {
			t.UnreadCount++
		}
		if t.LastMessageAt.IsZero() || stored.Date.After(t.LastMessageAt) {
			t.LastMessageAt = stored.Date
			t.Subject = stored.Subject
			t.Preview = stored.Text
		}
	}
	if result.Total < len(result.Threads) {
		result.Total = len(result.Threads)
	}
	return result, nil
}
