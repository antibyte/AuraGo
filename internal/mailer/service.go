package mailer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"path/filepath"
	"strings"
	"sync"
)

var ErrDisabled = errors.New("mailer is disabled")

type RuntimePolicy struct {
	Enabled  bool
	ReadOnly bool
}

type Event struct {
	Type      string `json:"type"`
	AccountID string `json:"account_id,omitempty"`
	ID        string `json:"id,omitempty"`
	Status    string `json:"status,omitempty"`
}

type ServiceOptions struct {
	DataDir                 string
	BlobRoot                string
	Store                   *Store
	ResolveAdapter          AdapterResolver
	ListSources             AccountSourceProvider
	Policy                  func() RuntimePolicy
	BeginOperation          func(context.Context) (context.Context, func(), error)
	SetAgentAccess          func(context.Context, string, bool) error
	OnChange                func(Event)
	MaxAttachmentBytes      int64
	MaxAttachmentTotalBytes int64
	MaxMessageCacheBytes    int64
}

type Service struct {
	store      *Store
	blobs      *BlobStore
	opts       ServiceOptions
	ownsStore  bool
	lifeMu     sync.Mutex
	lifeCancel context.CancelFunc
	lifeWG     sync.WaitGroup
	syncMu     sync.Mutex
	outboxMu   sync.Mutex
	activeMu   sync.Mutex
	activeNext uint64
	activeOps  map[uint64]context.CancelFunc
}

func NewService(opts ServiceOptions) (*Service, error) {
	if opts.ResolveAdapter == nil {
		return nil, errors.New("mailer adapter resolver is required")
	}
	store := opts.Store
	owns := false
	if store == nil {
		if strings.TrimSpace(opts.DataDir) == "" {
			return nil, errors.New("mailer data directory is required")
		}
		path := filepath.Join(opts.DataDir, "mailer", "mailer.db")
		var err error
		store, err = OpenStore(path)
		if err != nil {
			return nil, err
		}
		owns = true
	}
	root := opts.BlobRoot
	if root == "" {
		if opts.DataDir != "" {
			root = filepath.Join(opts.DataDir, "mailer", "blobs")
		} else {
			return nil, errors.New("mailer attachment root is required when a store is injected")
		}
	}
	blobs, err := OpenBlobStore(root, store, opts.MaxAttachmentBytes, opts.MaxAttachmentTotalBytes)
	if err != nil {
		if owns {
			_ = store.Close()
		}
		return nil, err
	}
	return &Service{store: store, blobs: blobs, opts: opts, ownsStore: owns, activeOps: make(map[uint64]context.CancelFunc)}, nil
}

func (s *Service) Store() *Store         { return s.store }
func (s *Service) BlobStore() *BlobStore { return s.blobs }

func (s *Service) Start(ctx context.Context) error {
	s.lifeMu.Lock()
	defer s.lifeMu.Unlock()
	if s.lifeCancel != nil {
		return errors.New("mailer service already started")
	}
	ctx, cancel := context.WithCancel(ctx)
	s.lifeCancel = cancel
	s.lifeWG.Add(2)
	go s.syncLoop(ctx)
	go s.outboxLoop(ctx)
	return nil
}

func (s *Service) Close() error {
	s.lifeMu.Lock()
	cancel := s.lifeCancel
	s.lifeCancel = nil
	s.lifeMu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.lifeWG.Wait()
	if s.ownsStore {
		return s.store.Close()
	}
	return nil
}

func (s *Service) runtimePolicy() RuntimePolicy {
	if s.opts.Policy == nil {
		return RuntimePolicy{Enabled: true}
	}
	return s.opts.Policy()
}

func (s *Service) begin(ctx context.Context, requireEnabled bool) (context.Context, func(), error) {
	p := s.runtimePolicy()
	if p.ReadOnly {
		return nil, nil, ErrReadOnly
	}
	if !p.Enabled {
		return nil, nil, ErrDisabled
	}
	if requireEnabled {
		settings, err := s.store.GetSettings(ctx)
		if err != nil {
			return nil, nil, err
		}
		if !settings.Enabled {
			return nil, nil, ErrDisabled
		}
	}
	var externalCancel context.CancelFunc = func() {}
	if s.opts.BeginOperation != nil {
		next, cancel, err := s.opts.BeginOperation(ctx)
		if err != nil {
			return nil, nil, err
		}
		if cancel != nil {
			externalCancel = cancel
		}
		ctx = next
	}
	opCtx, opCancel := context.WithCancel(ctx)
	s.activeMu.Lock()
	s.activeNext++
	id := s.activeNext
	s.activeOps[id] = opCancel
	s.activeMu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() { opCancel(); externalCancel(); s.activeMu.Lock(); delete(s.activeOps, id); s.activeMu.Unlock() })
	}
	return opCtx, cancel, nil
}

func (s *Service) cancelActiveOperations() {
	s.activeMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.activeOps))
	for _, cancel := range s.activeOps {
		cancels = append(cancels, cancel)
	}
	s.activeMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (s *Service) emit(ev Event) {
	if s.opts.OnChange != nil {
		s.opts.OnChange(ev)
	}
}

func (s *Service) GetSettings(ctx context.Context) (Settings, error) { return s.store.GetSettings(ctx) }

func (s *Service) UpdateSettings(ctx context.Context, v Settings) error {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return err
	}
	defer cancel()
	if err := s.store.UpdateSettings(ctx, v); err != nil {
		return err
	}
	if !v.Enabled {
		s.cancelActiveOperations()
	}
	s.emit(Event{Type: "settings", Status: boolStatus(v.Enabled)})
	return nil
}

func (s *Service) currentSources(ctx context.Context) ([]Account, error) {
	if s.opts.ListSources == nil {
		return nil, errors.New("current mail account sources are unavailable")
	}
	sources, err := s.opts.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(source.ID) == "" {
			return nil, errors.New("mail source has no account ID")
		}
		if _, ok := seen[source.ID]; ok {
			return nil, fmt.Errorf("duplicate mail source account %q", source.ID)
		}
		seen[source.ID] = struct{}{}
	}
	return sources, nil
}

func (s *Service) currentSource(ctx context.Context, id string) (Account, error) {
	sources, err := s.currentSources(ctx)
	if err != nil {
		return Account{}, err
	}
	for _, source := range sources {
		if source.ID == id {
			return source, nil
		}
	}
	return Account{}, ErrNotFound
}

func (s *Service) mergeAccount(ctx context.Context, source Account, linked bool) (Account, error) {
	source.Linked = linked
	if !linked {
		return source, nil
	}
	local, err := s.store.AccountSettings(ctx, source.ID)
	if err != nil {
		return Account{}, err
	}
	if stored, err := s.store.Account(ctx, source.ID); err == nil {
		if source.Address == "" {
			source.Address = stored.Address
		}
		source.SyncStatus = stored.SyncStatus
		source.SyncError = stored.SyncError
		source.SyncCheckedAt = stored.SyncCheckedAt
	}
	source.Disabled = source.Disabled || local.Disabled
	source.ReadOnly = source.ReadOnly || local.ReadOnly
	if source.ReadOnly {
		source.Capabilities = readOnlyCapabilities(source.Capabilities)
	}
	return source, nil
}

func readOnlyCapabilities(caps []string) []string {
	out := make([]string, 0, len(caps))
	for _, cap := range caps {
		switch strings.ToLower(cap) {
		case "read", "search", "folders_read":
			out = append(out, cap)
		}
	}
	return out
}

func (s *Service) Sources(ctx context.Context) ([]Account, error) {
	sources, err := s.currentSources(ctx)
	if err != nil {
		return nil, err
	}
	for i := range sources {
		linked, err := s.store.IsLinked(ctx, sources[i].ID)
		if err != nil {
			return nil, err
		}
		sources[i], err = s.mergeAccount(ctx, sources[i], linked)
		if err != nil {
			return nil, err
		}
	}
	return sources, nil
}

func (s *Service) Accounts(ctx context.Context) ([]Account, error) {
	linked, err := s.store.LinkedAccounts(ctx)
	if err != nil {
		return nil, err
	}
	linkedIDs := make(map[string]struct{}, len(linked))
	for _, a := range linked {
		linkedIDs[a.ID] = struct{}{}
	}
	sources, err := s.currentSources(ctx)
	if err != nil {
		return nil, err
	}
	accounts := make([]Account, 0, len(linked))
	for _, source := range sources {
		if _, ok := linkedIDs[source.ID]; !ok {
			continue
		}
		account, err := s.mergeAccount(ctx, source, true)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func (s *Service) Connect(ctx context.Context, id string) error {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return err
	}
	defer cancel()
	source, err := s.currentSource(ctx, id)
	if err != nil {
		return err
	}
	adapter, resolved, err := s.opts.ResolveAdapter(ctx, id)
	if err != nil {
		return err
	}
	if resolved.ID != id {
		return errors.New("resolved mail account ID does not match requested source")
	}
	address := strings.TrimSpace(source.Address)
	if address == "" {
		address = strings.TrimSpace(resolved.Address)
	}
	if address == "" {
		if profiled, ok := adapter.(ProfiledAdapter); ok {
			address, err = profiled.Profile(ctx)
			if err != nil {
				return fmt.Errorf("resolve mail account address: %w", err)
			}
		}
	}
	if address != "" {
		parsed, parseErr := mail.ParseAddress(address)
		if parseErr != nil || parsed.Address == "" {
			return fmt.Errorf("%w: account profile returned an invalid email address", ErrInvalid)
		}
		source.Address = parsed.Address
	}
	account := source
	account.Linked = true
	if err := s.store.PutAccount(ctx, account); err != nil {
		return err
	}
	s.emit(Event{Type: "account", AccountID: id, Status: "linked"})
	return nil
}

func (s *Service) Disconnect(ctx context.Context, id string) error {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return err
	}
	defer cancel()
	if err := s.store.UnlinkAccount(ctx, id); err != nil {
		return err
	}
	s.emit(Event{Type: "account", AccountID: id, Status: "unlinked"})
	return nil
}

func (s *Service) AccountSettings(ctx context.Context, id string) (AccountSettings, error) {
	account, err := s.authorizedAccount(ctx, id)
	if err != nil {
		return AccountSettings{}, err
	}
	settings, err := s.store.AccountSettings(ctx, id)
	if err != nil {
		return AccountSettings{}, err
	}
	settings.Disabled = account.Disabled
	settings.ReadOnly = account.ReadOnly
	settings.AgentAccess = account.AgentAccess
	return settings, nil
}

func (s *Service) SetAccountSettings(ctx context.Context, id string, v AccountSettings) error {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return err
	}
	defer cancel()
	account, err := s.authorizedAccount(ctx, id)
	if err != nil {
		return err
	}
	var normalizeErr error
	v.FolderMap, normalizeErr = normalizeFolderMap(v.FolderMap)
	if normalizeErr != nil {
		return normalizeErr
	}
	if v.AgentAccess != account.AgentAccess {
		if s.opts.SetAgentAccess == nil {
			return errors.New("agent access is controlled by account configuration")
		}
		if err := s.opts.SetAgentAccess(ctx, id, v.AgentAccess); err != nil {
			return err
		}
		account, err = s.currentSource(ctx, id)
		if err != nil {
			return err
		}
		if account.AgentAccess != v.AgentAccess {
			return errors.New("account source did not apply the agent access change")
		}
	}
	current, err := s.store.AccountSettings(ctx, id)
	if err != nil {
		return err
	}
	if !sameFolderMap(current.FolderMap, v.FolderMap) {
		if account.ReadOnly {
			return ErrReadOnly
		}
		folders, err := s.store.Folders(ctx, id)
		if err != nil {
			return err
		}
		allowedKinds := map[string]bool{"sent": true, "drafts": true, "archive": true, "junk": true, "trash": true, "deleted": true, "inbox": true}
		for kind, folderID := range v.FolderMap {
			kind = strings.ToLower(strings.TrimSpace(kind))
			if !allowedKinds[kind] || strings.TrimSpace(folderID) == "" {
				return fmt.Errorf("%w: invalid folder mapping", ErrInvalid)
			}
			found := false
			for _, folder := range folders {
				if folder.ID == folderID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("%w: mapped folder does not belong to this account", ErrInvalid)
			}
		}
	}
	if v.SignatureID != "" {
		signature, err := s.store.GetTemplate(ctx, v.SignatureID)
		if err != nil || signature.Kind != "signature" || (signature.AccountID != "" && signature.AccountID != id) {
			return fmt.Errorf("%w: signature does not belong to this account", ErrInvalid)
		}
	}
	// Source-level agent access is never persisted as a local grant.
	v.AgentAccess = account.AgentAccess
	if err := s.store.SetAccountSettings(ctx, id, v); err != nil {
		return err
	}
	s.emit(Event{Type: "account_settings", AccountID: id})
	return nil
}

func normalizeFolderMap(input map[string]string) (map[string]string, error) {
	if len(input) == 0 {
		return nil, nil
	}
	clean := make(map[string]string, len(input))
	for rawKind, rawID := range input {
		kind := strings.ToLower(strings.TrimSpace(rawKind))
		folderID := strings.TrimSpace(rawID)
		if kind == "" || folderID == "" {
			return nil, fmt.Errorf("%w: folder mapping requires a kind and folder ID", ErrInvalid)
		}
		if _, exists := clean[kind]; exists {
			return nil, fmt.Errorf("%w: duplicate folder mapping kind", ErrInvalid)
		}
		clean[kind] = folderID
	}
	return clean, nil
}

func sameFolderMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}

func buildThread(messages []Message) Thread {
	t := Thread{Messages: messages}
	if len(messages) == 0 {
		return t
	}
	t.ID = messages[0].ThreadID
	t.AccountID = messages[0].AccountID
	t.Count = len(messages)
	for _, m := range messages {
		if m.Unread {
			t.UnreadCount++
		}
		if m.Date.After(t.LastMessageAt) {
			t.LastMessageAt = m.Date
			t.Subject = m.Subject
			t.Preview = messagePreview(m)
			t.Starred = m.Starred
			t.FolderID = m.FolderID
		}
		for _, p := range append(append(append([]string{}, m.To...), m.CC...), m.From) {
			if p != "" && !contains(t.Participants, p) {
				t.Participants = append(t.Participants, p)
			}
		}
	}
	t.Unread = t.UnreadCount > 0
	return t
}

func (s *Service) MarkRead(ctx context.Context, threadID string, read bool) error {
	thread, err := s.GetThread(ctx, threadID)
	if err != nil {
		return err
	}
	kind := MutationUnread
	if read {
		kind = MutationRead
	}
	for _, m := range thread.Messages {
		if err := s.Mutate(ctx, m.ID, Mutation{Kind: kind}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListDrafts(ctx context.Context) ([]Draft, error) {
	drafts, err := s.store.ListDrafts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Draft, 0, len(drafts))
	for _, draft := range drafts {
		account, err := s.authorizedAccount(ctx, draft.AccountID)
		if err != nil || account.Disabled || !hasCapability(account.Capabilities, "read", "send") {
			continue
		}
		out = append(out, draft)
	}
	return out, nil
}
func (s *Service) GetDraft(ctx context.Context, id string) (Draft, error) {
	draft, err := s.store.GetDraft(ctx, id)
	if err != nil {
		return Draft{}, err
	}
	account, err := s.authorizedAccount(ctx, draft.AccountID)
	if err != nil {
		return Draft{}, err
	}
	if account.Disabled || !hasCapability(account.Capabilities, "read", "send") {
		return Draft{}, ErrNotFound
	}
	return draft, nil
}

func (s *Service) SaveDraft(ctx context.Context, d Draft) (Draft, error) {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return Draft{}, err
	}
	defer cancel()
	if d.AccountID == "" {
		return Draft{}, fmt.Errorf("%w: draft account is required", ErrInvalid)
	}
	if _, _, err := s.writableAccount(ctx, d.AccountID, "send"); err != nil {
		return Draft{}, err
	}
	if err := validateDraft(d); err != nil {
		return Draft{}, err
	}
	if d.ID != "" {
		old, err := s.store.GetDraft(ctx, d.ID)
		if err == nil && old.AccountID != d.AccountID {
			d.ReplyRemoteID = ""
			if _, _, err := s.writableAccount(ctx, old.AccountID, "send"); err != nil {
				return Draft{}, err
			}
			pending, err := s.store.DraftHasPendingSend(ctx, d.ID)
			if err != nil {
				return Draft{}, err
			}
			if pending {
				return Draft{}, ErrConflict
			}
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return Draft{}, err
		}
	} else if len(d.Attachments) != 0 {
		return Draft{}, fmt.Errorf("%w: attachments must be uploaded to the draft", ErrInvalid)
	}
	for _, a := range d.Attachments {
		if !s.store.BlobOwnedBy(ctx, a.ID, "draft", d.ID) {
			return Draft{}, fmt.Errorf("%w: attachment is not owned by this draft", ErrInvalid)
		}
	}
	saved, err := s.store.SaveDraft(ctx, d)
	if err != nil {
		return Draft{}, err
	}
	s.emit(Event{Type: "draft", AccountID: saved.AccountID, ID: saved.ID, Status: "saved"})
	return saved, nil
}

func (s *Service) DeleteDraft(ctx context.Context, id string, revision int64) error {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return err
	}
	defer cancel()
	draft, err := s.store.GetDraft(ctx, id)
	if err != nil {
		return err
	}
	if _, _, err := s.writableAccount(ctx, draft.AccountID, "send"); err != nil {
		return err
	}
	pending, err := s.store.DraftHasPendingSend(ctx, id)
	if err != nil {
		return err
	}
	if pending {
		return ErrConflict
	}
	if err := s.store.DeleteDraft(ctx, id, revision); err != nil {
		return err
	}
	for _, attachment := range draft.Attachments {
		if err := s.blobs.Remove(ctx, attachment.ID); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	s.emit(Event{Type: "draft", ID: id, Status: "deleted"})
	return nil
}

func (s *Service) AddDraftAttachment(ctx context.Context, draftID string, revision int64, filename, contentType string, src io.Reader) (Draft, error) {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return Draft{}, err
	}
	defer cancel()
	d, err := s.store.GetDraft(ctx, draftID)
	if err != nil {
		return Draft{}, err
	}
	if d.Revision != revision {
		return Draft{}, ErrConflict
	}
	if _, _, err := s.writableAccount(ctx, d.AccountID, "send"); err != nil {
		return Draft{}, err
	}
	a, err := s.blobs.Put(ctx, "draft", draftID, filename, contentType, src)
	if err != nil {
		return Draft{}, err
	}
	d.Attachments = append(d.Attachments, a)
	saved, err := s.store.SaveDraft(ctx, d)
	if err != nil {
		_ = s.blobs.Remove(context.Background(), a.ID)
		return Draft{}, err
	}
	s.emit(Event{Type: "draft_attachment", AccountID: d.AccountID, ID: draftID, Status: "added"})
	return saved, nil
}

func (s *Service) RemoveDraftAttachment(ctx context.Context, draftID string, revision int64, attachmentID string) (Draft, error) {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return Draft{}, err
	}
	defer cancel()
	d, err := s.store.GetDraft(ctx, draftID)
	if err != nil {
		return Draft{}, err
	}
	if d.Revision != revision {
		return Draft{}, ErrConflict
	}
	if _, _, err := s.writableAccount(ctx, d.AccountID, "send"); err != nil {
		return Draft{}, err
	}
	found := false
	attachments := make([]Attachment, 0, len(d.Attachments))
	for _, a := range d.Attachments {
		if a.ID == attachmentID {
			found = true
			continue
		}
		attachments = append(attachments, a)
	}
	if !found {
		return Draft{}, ErrNotFound
	}
	pending, err := s.store.BlobHasPendingSend(ctx, attachmentID)
	if err != nil {
		return Draft{}, err
	}
	if pending {
		return Draft{}, ErrConflict
	}
	d.Attachments = attachments
	saved, err := s.store.SaveDraft(ctx, d)
	if err != nil {
		return Draft{}, err
	}
	if err := s.blobs.Remove(ctx, attachmentID); err != nil {
		return saved, err
	}
	s.emit(Event{Type: "draft_attachment", AccountID: d.AccountID, ID: draftID, Status: "removed"})
	return saved, nil
}

func (s *Service) ReadAttachment(ctx context.Context, id string) (Attachment, io.ReadCloser, error) {
	kind, ownerID, err := s.store.BlobOwner(ctx, id)
	if err != nil {
		return Attachment{}, nil, err
	}
	if kind != "draft" {
		return Attachment{}, nil, ErrNotFound
	}
	draft, err := s.store.GetDraft(ctx, ownerID)
	if err != nil {
		return Attachment{}, nil, ErrNotFound
	}
	account, err := s.authorizedAccount(ctx, draft.AccountID)
	if err != nil {
		return Attachment{}, nil, err
	}
	if account.Disabled || !hasCapability(account.Capabilities, "read", "send") {
		return Attachment{}, nil, ErrDisabled
	}
	a, f, err := s.blobs.Open(ctx, id)
	if err != nil {
		return Attachment{}, nil, err
	}
	return a, f, nil
}

func (s *Service) ListTemplates(ctx context.Context) ([]Template, error) {
	values, err := s.store.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Template, 0, len(values))
	for _, value := range values {
		if value.AccountID == "" {
			out = append(out, value)
			continue
		}
		account, err := s.authorizedAccount(ctx, value.AccountID)
		if err == nil && !account.Disabled && hasCapability(account.Capabilities, "read", "send") {
			out = append(out, value)
		}
	}
	return out, nil
}
func (s *Service) SaveTemplate(ctx context.Context, t Template) (Template, error) {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return Template{}, err
	}
	defer cancel()
	if strings.TrimSpace(t.Name) == "" || len(t.Name) > 128 || strings.ContainsAny(t.Name, "\r\n\x00") {
		return Template{}, fmt.Errorf("%w: template name is required and must be at most 128 characters", ErrInvalid)
	}
	if t.Kind != "template" && t.Kind != "signature" {
		return Template{}, fmt.Errorf("%w: template kind must be template or signature", ErrInvalid)
	}
	if t.ID != "" {
		old, err := s.store.GetTemplate(ctx, t.ID)
		if err == nil && old.AccountID != t.AccountID {
			return Template{}, fmt.Errorf("%w: template account cannot be changed", ErrInvalid)
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return Template{}, err
		}
	}
	if t.AccountID == "" {
		if err := s.requireAnyWritableAccount(ctx, "send"); err != nil {
			return Template{}, err
		}
	} else if _, _, err := s.writableAccount(ctx, t.AccountID, "send"); err != nil {
		return Template{}, err
	}
	saved, err := s.store.SaveTemplate(ctx, t)
	if err == nil {
		s.emit(Event{Type: "template", AccountID: saved.AccountID, ID: saved.ID, Status: "saved"})
	}
	return saved, err
}
func (s *Service) DeleteTemplate(ctx context.Context, id string, revision int64) error {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return err
	}
	defer cancel()
	template, err := s.store.GetTemplate(ctx, id)
	if err != nil {
		return err
	}
	if template.AccountID == "" {
		if err := s.requireAnyWritableAccount(ctx, "send"); err != nil {
			return err
		}
	} else if _, _, err := s.writableAccount(ctx, template.AccountID, "send"); err != nil {
		return err
	}
	if revision < 1 || template.Revision != revision {
		return ErrConflict
	}
	if err := s.store.DeleteTemplate(ctx, id, revision); err != nil {
		return err
	}
	s.emit(Event{Type: "template", ID: id, Status: "deleted"})
	return nil
}

func validateDraft(d Draft) error {
	if len(d.ReplyRemoteID) > 998 || strings.ContainsAny(d.ReplyRemoteID, "\x00\r\n") {
		return fmt.Errorf("%w: invalid provider reply identifier", ErrInvalid)
	}
	if strings.ContainsAny(d.Subject, "\r\n") {
		return fmt.Errorf("%w: subject contains a line break", ErrInvalid)
	}
	for _, list := range [][]string{d.To, d.CC, d.BCC} {
		for _, raw := range list {
			a, err := mail.ParseAddress(raw)
			if err != nil || a.Address == "" {
				return fmt.Errorf("%w: invalid mail recipient", ErrInvalid)
			}
		}
	}
	if d.InReplyTo != "" {
		if _, err := safeMessageID(d.InReplyTo); err != nil {
			return fmt.Errorf("%w: invalid in-reply-to identifier", ErrInvalid)
		}
	}
	if len(d.References) > 100 {
		return fmt.Errorf("%w: too many reference identifiers", ErrInvalid)
	}
	for _, ref := range d.References {
		if _, err := safeMessageID(ref); err != nil {
			return fmt.Errorf("%w: invalid reference identifier", ErrInvalid)
		}
	}
	return nil
}

func validMutation(k MutationKind) bool {
	switch k {
	case MutationRead, MutationUnread, MutationStar, MutationUnstar, MutationMove, MutationArchive, MutationTrash, MutationRestore, MutationDelete, MutationJunk, MutationLabelAdd, MutationLabelRemove:
		return true
	}
	return false
}
func hasCapability(caps []string, values ...string) bool {
	for _, c := range caps {
		for _, v := range values {
			if strings.EqualFold(c, v) {
				return true
			}
		}
	}
	return false
}
func boolStatus(v bool) string {
	if v {
		return "enabled"
	}
	return "disabled"
}
