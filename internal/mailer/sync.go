package mailer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	maxConcurrentAccountSyncs = 4
	accountSyncTimeout        = 90 * time.Second
	syncIdleInterval          = 5 * time.Minute
	syncActiveInterval        = 60 * time.Second
)

func (s *Service) now() time.Time {
	if s.opts.Now != nil {
		return s.opts.Now()
	}
	return time.Now()
}

func (s *Service) syncDue(accountID string, now time.Time) bool {
	s.scheduleMu.Lock()
	defer s.scheduleMu.Unlock()
	due, ok := s.syncDueAt[accountID]
	if !ok {
		return true
	}
	return !now.Before(due)
}

func (s *Service) noteSyncSchedule(accountID string, changed bool, err error) {
	if accountID == "" {
		return
	}
	interval := syncIdleInterval
	if changed || (err != nil && !errors.Is(err, ErrDisabled) && !errors.Is(err, ErrReadOnly)) {
		interval = syncActiveInterval
	}
	due := s.now().Add(interval)
	s.scheduleMu.Lock()
	defer s.scheduleMu.Unlock()
	if s.syncDueAt == nil {
		s.syncDueAt = map[string]time.Time{}
	}
	s.syncDueAt[accountID] = due
}

func (s *Service) SyncAccount(ctx context.Context, accountID string) error {
	ctx, cancel, err := s.begin(ctx, true)
	if err != nil {
		return err
	}
	defer cancel()
	if !s.syncMu.TryLock() {
		return ErrConflict
	}
	defer s.syncMu.Unlock()
	changed, err := s.syncAccount(ctx, accountID)
	s.noteSyncSchedule(accountID, changed, err)
	return err
}

func (s *Service) syncAll(ctx context.Context) {
	if !s.syncMu.TryLock() {
		return
	}
	defer s.syncMu.Unlock()
	policy := s.runtimePolicy()
	settings, err := s.store.GetSettings(ctx)
	if err != nil || !policy.Enabled || !settings.Enabled {
		return
	}
	accounts, err := s.Accounts(ctx)
	if err != nil {
		return
	}
	semaphore := make(chan struct{}, maxConcurrentAccountSyncs)
	var workers sync.WaitGroup
accountsLoop:
	for _, account := range accounts {
		if ctx.Err() != nil {
			break
		}
		if account.Disabled {
			continue
		}
		if !s.syncDue(account.ID, s.now()) {
			continue
		}
		select {
		case semaphore <- struct{}{}:
		case <-ctx.Done():
			break accountsLoop
		}
		workers.Add(1)
		go func(accountID string) {
			defer workers.Done()
			defer func() { <-semaphore }()
			accountCtx, cancel := context.WithTimeout(ctx, accountSyncTimeout)
			defer cancel()
			// Each account resolves current config and gets a revocable Desktop
			// operation context. One broken source cannot hold up healthy accounts.
			changed, err := s.syncAccount(accountCtx, accountID)
			s.noteSyncSchedule(accountID, changed, err)
			if err != nil && !errors.Is(err, ErrDisabled) && !errors.Is(err, ErrReadOnly) {
				return
			}
		}(account.ID)
	}
	workers.Wait()
}

func (s *Service) syncAccount(parent context.Context, accountID string) (bool, error) {
	ctx, cancel, err := s.begin(parent, true)
	if err != nil {
		return false, err
	}
	defer cancel()
	ctx, releaseAccount := s.trackAccountOperation(ctx, cancel, accountID)
	defer releaseAccount()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	account, err := s.authorizedAccount(ctx, accountID)
	if err != nil {
		return false, err
	}
	if account.Disabled {
		return false, ErrDisabled
	}
	if !hasCapability(account.Capabilities, "read") {
		return false, ErrCapability
	}
	adapter, resolved, err := s.resolve(ctx, accountID)
	if err != nil {
		return false, err
	}
	if resolved.Disabled {
		return false, ErrDisabled
	}
	if !hasCapability(resolved.Capabilities, "read") {
		return false, ErrCapability
	}
	if resolved.ReadOnly {
		if err := s.store.SkipPendingRuleRuns(ctx, accountID, "account is read-only"); err != nil {
			return false, err
		}
	} else if err := s.processPendingRuleRuns(ctx, accountID); err != nil && ctx.Err() != nil {
		return false, ctx.Err()
	}
	var stopSession func()
	defer func() {
		if stopSession != nil {
			stopSession()
		}
	}()
	if opener, ok := adapter.(SyncSession); ok {
		stop, sessionErr := opener.StartSyncSession(ctx)
		if sessionErr != nil {
			return false, s.failSync(ctx, accountID, sessionErr)
		}
		stopSession = stop
	}
	changed := false
	folders, err := adapter.Folders(ctx)
	if err != nil {
		return false, s.failSync(ctx, accountID, err)
	}
	for i := range folders {
		if folders[i].ID == "" {
			return false, s.failSync(ctx, accountID, ErrInvalid)
		}
		folders[i].AccountID = accountID
	}
	if err := s.store.PutFolders(ctx, accountID, folders); err != nil {
		return false, s.failSync(ctx, accountID, err)
	}
	isPop3 := isPOP3(resolved)
	if isPop3 {
		if err := s.ensureLocalPOP3Folders(ctx, resolved); err != nil {
			return false, s.failSync(ctx, accountID, err)
		}
	}
	for _, folder := range folders {
		cursor, err := s.store.Cursor(ctx, accountID, folder.ID)
		if err != nil {
			return false, s.failSync(ctx, accountID, err)
		}
		snapshotActive, err := s.store.RemoteSnapshotActive(ctx, accountID, folder.ID)
		if err != nil {
			return false, s.failSync(ctx, accountID, err)
		}
		if cursor == "" && !isPop3 && !snapshotActive {
			if err := s.store.BeginRemoteSnapshot(ctx, accountID, folder.ID); err != nil {
				return false, s.failSync(ctx, accountID, err)
			}
			snapshotActive = true
		}
		hasMore := false
		for pageNumber := 0; pageNumber < 10; pageNumber++ {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			result, err := adapter.Sync(ctx, folder.ID, cursor)
			if err != nil {
				return false, s.failSync(ctx, accountID, err)
			}
			if result.ResetSnapshot && !isPop3 {
				if err := s.store.SetCursor(ctx, accountID, folder.ID, ""); err != nil {
					return false, s.failSync(ctx, accountID, err)
				}
				cursor = ""
				if err := s.store.BeginRemoteSnapshot(ctx, accountID, folder.ID); err != nil {
					return false, s.failSync(ctx, accountID, err)
				}
				snapshotActive = true
			}
			previousValidity, err := s.store.CursorValidity(ctx, accountID, folder.ID)
			if err != nil {
				return false, s.failSync(ctx, accountID, err)
			}
			if !isPop3 && previousValidity != 0 && result.UIDValidity != 0 && previousValidity != result.UIDValidity {
				if !snapshotActive {
					if err := s.store.BeginRemoteSnapshot(ctx, accountID, folder.ID); err != nil {
						return false, s.failSync(ctx, accountID, err)
					}
					snapshotActive = true
				}
				// If the adapter did not identify its returned page as the start
				// of a replacement snapshot, discard its incremental cursor and
				// ask for a clean first page. Keep the previous cache until the
				// complete snapshot can atomically reconcile it.
				if !result.ResetSnapshot {
					if err := s.store.SetCursor(ctx, accountID, folder.ID, ""); err != nil {
						return false, s.failSync(ctx, accountID, err)
					}
					cursor = ""
					continue
				}
			}
			for i := range result.Messages {
				result.Messages[i].AccountID = accountID
				result.Messages[i].FolderID = folder.ID
			}
			if len(result.Messages) > 0 || len(result.DeletedRemoteIDs) > 0 {
				changed = true
			}
			hasMore = result.HasMore
			// Keep sync-index insertion atomic with local permission changes. The
			// gate is intentionally limited to this transaction; rule actions take
			// their own gate through Mutate/Snooze.
			gate := s.accountGate(accountID)
			gate.RLock()
			if err := ctx.Err(); err != nil {
				gate.RUnlock()
				return false, err
			}
			current, currentErr := s.authorizedAccount(ctx, accountID)
			if currentErr != nil {
				gate.RUnlock()
				return false, currentErr
			}
			readOnly := resolved.ReadOnly || current.ReadOnly
			if readOnly {
				if err := s.store.SkipPendingRuleRuns(ctx, accountID, "account is read-only"); err != nil {
					gate.RUnlock()
					return false, err
				}
				_, err = s.store.PutMessagesWithLimit(ctx, result.Messages, s.opts.MaxMessageCacheBytes)
			} else {
				_, err = s.store.PutSyncMessagesWithLimit(ctx, result.Messages, s.opts.MaxMessageCacheBytes)
			}
			gate.RUnlock()
			if err != nil {
				return false, s.failSync(ctx, accountID, err)
			}
			if !isPop3 && snapshotActive {
				seen := make([]string, 0, len(result.Messages))
				for _, message := range result.Messages {
					seen = append(seen, message.RemoteID)
				}
				if err := s.store.MarkRemoteSnapshotSeen(ctx, accountID, folder.ID, seen); err != nil {
					return false, s.failSync(ctx, accountID, err)
				}
			}
			if len(result.DeletedRemoteIDs) > 0 && !isPop3 {
				if err := s.store.DeleteRemoteMessages(ctx, accountID, folder.ID, result.DeletedRemoteIDs); err != nil {
					return false, s.failSync(ctx, accountID, err)
				}
			}
			if result.UIDValidity != 0 {
				if err := s.store.SetCursorValidity(ctx, accountID, folder.ID, result.UIDValidity); err != nil {
					return false, s.failSync(ctx, accountID, err)
				}
			}
			if result.RemoteSnapshotComplete && !isPop3 && snapshotActive {
				if err := s.store.FinishRemoteSnapshot(ctx, accountID, folder.ID); err != nil {
					return false, s.failSync(ctx, accountID, err)
				}
				snapshotActive = false
			}
			if !resolved.ReadOnly {
				if err := s.processPendingRuleRuns(ctx, accountID); err != nil && ctx.Err() != nil {
					return false, ctx.Err()
				}
			}
			next := result.NextCursor
			if next == cursor && result.HasMore {
				return false, s.failSync(ctx, accountID, ErrSyncFailed)
			}
			if next != cursor {
				if err := s.store.SetCursor(ctx, accountID, folder.ID, next); err != nil {
					return false, s.failSync(ctx, accountID, err)
				}
				cursor = next
			}
			if !result.HasMore {
				break
			}
		}
		if hasMore {
			changed = true
		}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	hasRuleErrors, err := s.store.HasRuleRunErrors(ctx, accountID)
	if err != nil {
		return false, err
	}
	status, code := "ok", ""
	if hasRuleErrors {
		status, code = "error", "rule_error"
	}
	if err := s.store.SetAccountSyncStatus(ctx, accountID, status, code, s.now().UTC()); err != nil {
		return false, err
	}
	s.emit(Event{Type: "account_sync", AccountID: accountID, Status: status})
	return changed, nil
}

func (s *Service) failSync(ctx context.Context, accountID string, cause ...error) error {
	writeCtx := context.WithoutCancel(ctx)
	code := "sync_failed"
	if len(cause) > 0 && errors.Is(cause[0], ErrStorageFull) {
		code = "storage_full"
	} else if len(cause) > 0 {
		code = syncFailureCode(cause[0])
	}
	_ = s.store.SetAccountSyncStatus(writeCtx, accountID, "error", code, time.Now().UTC())
	s.emit(Event{Type: "account_sync", AccountID: accountID, Status: "error"})
	return fmt.Errorf("%w", ErrSyncFailed)
}

func syncFailureCode(err error) string {
	if errors.Is(err, ErrDisabled) {
		return "disabled"
	}
	if errors.Is(err, ErrReadOnly) {
		return "permission_denied"
	}
	if errors.Is(err, ErrCapability) {
		return "missing_scope"
	}
	if err == nil {
		return "sync_failed"
	}
	var googleErr *gmailAPIError
	if errors.As(err, &googleErr) {
		switch {
		case googleErr.StatusCode == 401:
			return "auth_required"
		case googleErr.StatusCode == 403 && strings.Contains(strings.ToLower(googleErr.Message), "scope"):
			return "missing_scope"
		case googleErr.StatusCode == 403:
			return "permission_denied"
		case googleErr.StatusCode == 429 || googleErr.StatusCode >= 500:
			return "offline"
		}
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "auth"), strings.Contains(text, "oauth"), strings.Contains(text, "token"):
		return "auth_required"
	case strings.Contains(text, "scope"):
		return "missing_scope"
	case strings.Contains(text, "permission"), strings.Contains(text, "forbidden"):
		return "permission_denied"
	case strings.Contains(text, "timeout"), strings.Contains(text, "offline"), strings.Contains(text, "connection"), strings.Contains(text, "network"):
		return "offline"
	default:
		return "sync_failed"
	}
}
