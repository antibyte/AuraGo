package mailer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

func (s *Service) authorizedAccount(ctx context.Context, id string) (Account, error) {
	if id == "" {
		return Account{}, ErrNotFound
	}
	linked, err := s.store.IsLinked(ctx, id)
	if err != nil {
		return Account{}, err
	}
	if !linked {
		return Account{}, ErrNotFound
	}
	source, err := s.currentSource(ctx, id)
	if err != nil {
		return Account{}, err
	}
	account, err := s.mergeAccount(ctx, source, true)
	if err != nil {
		return Account{}, err
	}
	return account, nil
}

func (s *Service) allowedAccountIDs(ctx context.Context) ([]string, error) {
	accounts, err := s.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if account.Disabled || !account.Linked {
			continue
		}
		_, current, err := s.resolve(ctx, account.ID)
		if err != nil || current.Disabled || !hasCapability(current.Capabilities, "read") {
			continue
		}
		ids = append(ids, account.ID)
	}
	return ids, nil
}

func (s *Service) resolve(ctx context.Context, id string) (Adapter, Account, error) {
	account, err := s.authorizedAccount(ctx, id)
	if err != nil {
		return nil, Account{}, err
	}
	adapter, resolved, err := s.opts.ResolveAdapter(ctx, id)
	if err != nil {
		return nil, Account{}, err
	}
	if resolved.ID != id {
		return nil, Account{}, errors.New("resolved mail account ID does not match linked account")
	}
	account.Capabilities = intersectCapabilities(account.Capabilities, resolved.Capabilities)
	account.Disabled = account.Disabled || resolved.Disabled
	account.ReadOnly = account.ReadOnly || resolved.ReadOnly
	account.AgentAccess = account.AgentAccess && resolved.AgentAccess
	if account.ReadOnly {
		account.Capabilities = readOnlyCapabilities(account.Capabilities)
	}
	return adapter, account, nil
}

func (s *Service) writableAccount(ctx context.Context, id, capability string) (Adapter, Account, error) {
	adapter, account, err := s.resolve(ctx, id)
	if err != nil {
		return nil, Account{}, err
	}
	if account.Disabled {
		return nil, Account{}, ErrDisabled
	}
	if account.ReadOnly {
		return nil, Account{}, ErrReadOnly
	}
	if capability != "" && !hasCapability(account.Capabilities, capability) {
		return nil, Account{}, ErrCapability
	}
	return adapter, account, nil
}

func (s *Service) requireAnyWritableAccount(ctx context.Context, capability string) error {
	accounts, err := s.Accounts(ctx)
	if err != nil {
		return err
	}
	for _, candidate := range accounts {
		if candidate.Disabled || !candidate.Linked {
			continue
		}
		_, account, err := s.writableAccount(ctx, candidate.ID, capability)
		if err == nil && !account.Disabled {
			return nil
		}
	}
	return ErrReadOnly
}

func (s *Service) QueryMessages(ctx context.Context, query MessageQuery) (MessagePage, error) {
	allowed, err := s.allowedAccountIDs(ctx)
	if err != nil {
		return MessagePage{}, err
	}
	query.AllowedAccountIDs = allowed
	if query.AccountID != "" && !contains(allowed, query.AccountID) {
		query.AllowedAccountIDs = []string{}
	}
	return s.store.QueryMessages(ctx, query)
}

func (s *Service) QueryThreads(ctx context.Context, query MessageQuery) (ThreadPage, error) {
	allowed, err := s.allowedAccountIDs(ctx)
	if err != nil {
		return ThreadPage{}, err
	}
	query.AllowedAccountIDs = allowed
	if query.AccountID != "" && !contains(allowed, query.AccountID) {
		query.AllowedAccountIDs = []string{}
	}
	return s.store.QueryThreadPage(ctx, query)
}

func (s *Service) Folders(ctx context.Context, accountID string) ([]Folder, error) {
	if accountID != "" {
		account, err := s.authorizedAccount(ctx, accountID)
		if err != nil {
			return nil, err
		}
		if account.Disabled {
			return nil, ErrDisabled
		}
		if !hasCapability(account.Capabilities, "folders_read") {
			return nil, ErrCapability
		}
		return s.store.Folders(ctx, accountID)
	}
	accounts, err := s.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(accounts))
	for _, account := range accounts {
		if account.Disabled || !hasCapability(account.Capabilities, "folders_read") {
			continue
		}
		allowed[account.ID] = struct{}{}
	}
	folders, err := s.store.Folders(ctx, "")
	if err != nil {
		return nil, err
	}
	out := make([]Folder, 0, len(folders))
	for _, folder := range folders {
		if _, ok := allowed[folder.AccountID]; ok {
			out = append(out, folder)
		}
	}
	return out, nil
}

func intersectCapabilities(source, resolved []string) []string {
	if len(source) == 0 || len(resolved) == 0 {
		return []string{}
	}
	allowed := make(map[string]struct{}, len(resolved))
	for _, capability := range resolved {
		allowed[strings.ToLower(capability)] = struct{}{}
	}
	out := make([]string, 0, len(source))
	for _, capability := range source {
		if _, ok := allowed[strings.ToLower(capability)]; ok {
			out = append(out, capability)
		}
	}
	return out
}

func isPOP3(account Account) bool {
	return strings.EqualFold(account.Protocol, "pop3") || strings.EqualFold(account.Source, "pop3")
}

func localFolderID(accountID, kind string) string {
	return accountID + ":local:" + kind
}

func (s *Service) ensureLocalPOP3Folders(ctx context.Context, account Account) error {
	folders, err := s.store.Folders(ctx, account.ID)
	if err != nil {
		return err
	}
	byKind := make(map[string]bool, len(folders))
	for _, folder := range folders {
		byKind[strings.ToLower(folder.Kind)] = true
	}
	local := make([]Folder, 0, 5)
	if !byKind["inbox"] {
		local = append(local, Folder{ID: account.ID + ":inbox", ProviderID: "inbox", Name: "Inbox", Kind: "inbox"})
	}
	for _, def := range []struct{ kind, name string }{{"archive", "Archive"}, {"trash", "Trash"}, {"junk", "Junk"}, {"deleted", "Deleted"}} {
		if !byKind[def.kind] {
			local = append(local, Folder{ID: localFolderID(account.ID, def.kind), ProviderID: def.kind, Name: def.name, Kind: def.kind})
		}
	}
	for i := range local {
		local[i].AccountID = account.ID
	}
	if len(local) == 0 {
		return nil
	}
	return s.store.PutFolders(ctx, account.ID, local)
}

func (s *Service) CreateFolder(ctx context.Context, accountID, name, parentID string) (Folder, error) {
	ctx, cancel, err := s.begin(ctx, true)
	if err != nil {
		return Folder{}, err
	}
	defer cancel()
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 || strings.ContainsAny(name, "\r\n\x00") {
		return Folder{}, fmt.Errorf("%w: folder name is required and must be at most 128 characters", ErrInvalid)
	}
	account, err := s.authorizedAccount(ctx, accountID)
	if err != nil {
		return Folder{}, err
	}
	if account.Disabled {
		return Folder{}, ErrDisabled
	}
	if account.ReadOnly {
		return Folder{}, ErrReadOnly
	}
	if !hasCapability(account.Capabilities, "folders_create") {
		return Folder{}, ErrCapability
	}
	if parentID != "" {
		folders, err := s.store.Folders(ctx, accountID)
		if err != nil {
			return Folder{}, err
		}
		found := false
		for _, folder := range folders {
			if folder.ID == parentID {
				found = true
				break
			}
		}
		if !found {
			return Folder{}, fmt.Errorf("%w: parent folder does not belong to this account", ErrInvalid)
		}
	}
	var folder Folder
	if isPOP3(account) {
		if err := s.ensureLocalPOP3Folders(ctx, account); err != nil {
			return Folder{}, err
		}
		sum := sha256.Sum256([]byte(strings.ToLower(name)))
		folder = Folder{ID: account.ID + ":local:custom:" + hex.EncodeToString(sum[:8]), AccountID: accountID, Name: name, Kind: "custom", ProviderID: name}
	} else {
		adapter, current, err := s.resolve(ctx, accountID)
		if err != nil {
			return Folder{}, err
		}
		if current.Disabled {
			return Folder{}, ErrDisabled
		}
		manager, ok := adapter.(FolderManager)
		if !ok {
			return Folder{}, ErrCapability
		}
		folder, err = manager.CreateFolder(ctx, name, parentID)
		if err != nil {
			return Folder{}, err
		}
		if folder.ID == "" {
			return Folder{}, errors.New("adapter returned a folder without an ID")
		}
		folder.AccountID = accountID
	}
	if err := ctx.Err(); err != nil {
		return Folder{}, err
	}
	if err := s.store.PutFolders(ctx, accountID, []Folder{folder}); err != nil {
		return Folder{}, err
	}
	s.emit(Event{Type: "folder", AccountID: accountID, ID: folder.ID, Status: "created"})
	return folder, nil
}
