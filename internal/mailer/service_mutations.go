package mailer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (s *Service) Mutate(ctx context.Context, id string, mutation Mutation) error {
	if !validMutation(mutation.Kind) {
		return fmt.Errorf("%w: unsupported mail mutation %q", ErrInvalid, mutation.Kind)
	}
	ctx, cancel, err := s.begin(ctx, true)
	if err != nil {
		return err
	}
	defer cancel()
	m, err := s.store.GetMessage(ctx, id)
	if err != nil {
		return err
	}
	account, err := s.authorizedAccount(ctx, m.AccountID)
	if err != nil {
		return err
	}
	var adapter Adapter
	if !isPOP3(account) {
		adapter, account, err = s.resolve(ctx, m.AccountID)
		if err != nil {
			return err
		}
	}
	if account.Disabled {
		return ErrDisabled
	}
	if account.ReadOnly {
		return ErrReadOnly
	}
	if !hasCapability(account.Capabilities, "read") {
		return ErrCapability
	}
	if err := s.ensureCurrentRemoteMessage(ctx, account, m); err != nil {
		return err
	}
	if capability := mutationCapability(mutation.Kind); !hasCapability(account.Capabilities, capability) {
		return ErrCapability
	}
	if (mutation.Kind == MutationLabelAdd || mutation.Kind == MutationLabelRemove) && !validLabel(mutation.Label) {
		return fmt.Errorf("%w: label is required and must be at most 128 characters", ErrInvalid)
	}
	if mutation.Kind == MutationMove && mutation.DestinationFolderID == "" {
		return fmt.Errorf("%w: move requires a destination folder", ErrInvalid)
	}
	if mutation.Kind == MutationMove || mutation.DestinationFolderID != "" {
		if mutation.DestinationFolderID == "" {
			return fmt.Errorf("%w: destination folder is required", ErrInvalid)
		}
		folders, err := s.store.Folders(ctx, m.AccountID)
		if err != nil {
			return err
		}
		found := false
		for _, folder := range folders {
			if folder.ID == mutation.DestinationFolderID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: destination folder does not belong to this account", ErrInvalid)
		}
	}
	if mutation.Kind == MutationArchive || mutation.Kind == MutationTrash || mutation.Kind == MutationRestore || mutation.Kind == MutationJunk {
		if mutation.DestinationFolderID == "" {
			kind := map[MutationKind]string{MutationArchive: "archive", MutationTrash: "trash", MutationRestore: "inbox", MutationJunk: "junk"}[mutation.Kind]
			folder, err := s.folderByKind(ctx, m.AccountID, kind)
			if err != nil {
				return err
			}
			mutation.DestinationFolderID = folder.ID
		}
	}
	if isPOP3(account) {
		if err := s.applyLocalMutation(ctx, &m, mutation); err != nil {
			return err
		}
	} else {
		if err := adapter.Mutate(ctx, m.FolderID, m.RemoteID, mutation); err != nil {
			return err
		}
		if err := s.checkMutationPublication(ctx); err != nil {
			return err
		}
		if mutation.Kind == MutationDelete {
			if err := s.store.DeleteMessage(ctx, id); err != nil {
				return err
			}
			s.emit(Event{Type: "message", AccountID: m.AccountID, ID: m.ID, Status: string(mutation.Kind)})
			return nil
		}
		if err := s.applyLocalMutation(ctx, &m, mutation); err != nil {
			return err
		}
	}
	if err := s.checkMutationPublication(ctx); err != nil {
		return err
	}
	if err := s.store.UpdateMessage(ctx, m); err != nil {
		return err
	}
	s.emit(Event{Type: "message", AccountID: m.AccountID, ID: m.ID, Status: string(mutation.Kind)})
	return nil
}

func (s *Service) ensureCurrentRemoteMessage(ctx context.Context, account Account, message Message) error {
	if !strings.EqualFold(account.Protocol, "imap") && !(account.Protocol == "" && strings.EqualFold(account.Source, "email")) {
		return nil
	}
	validity, err := s.store.CursorValidity(ctx, message.AccountID, message.FolderID)
	if err != nil {
		return err
	}
	if validity == 0 {
		return nil
	}
	encodedValidity, _, ok := strings.Cut(message.RemoteID, ":")
	if !ok {
		// Legacy cache IDs do not carry a generation, so once the server has
		// reported a UIDVALIDITY we cannot safely target them.
		return ErrConflict
	}
	messageValidity, err := strconv.ParseUint(encodedValidity, 10, 32)
	if err != nil || uint32(messageValidity) != validity {
		return ErrConflict
	}
	return nil
}

func mutationCapability(kind MutationKind) string {
	return map[MutationKind]string{
		MutationRead: "read_state", MutationUnread: "read_state", MutationStar: "star", MutationUnstar: "star",
		MutationMove: "move", MutationArchive: "archive", MutationTrash: "trash", MutationRestore: "restore",
		MutationDelete: "delete", MutationJunk: "junk", MutationLabelAdd: "labels_write", MutationLabelRemove: "labels_write",
	}[kind]
}

func validLabel(label string) bool {
	label = strings.TrimSpace(label)
	return label != "" && len(label) <= 128 && !strings.ContainsAny(label, "\r\n\x00")
}

func (s *Service) applyLocalMutation(ctx context.Context, message *Message, mutation Mutation) error {
	switch mutation.Kind {
	case MutationRead:
		message.Unread = false
	case MutationUnread:
		message.Unread = true
	case MutationStar:
		message.Starred = true
	case MutationUnstar:
		message.Starred = false
	case MutationMove, MutationArchive, MutationTrash, MutationRestore, MutationJunk:
		message.FolderID = mutation.DestinationFolderID
	case MutationDelete:
		folder, err := s.folderByKind(ctx, message.AccountID, "deleted")
		if err != nil {
			return err
		}
		message.FolderID = folder.ID
	case MutationLabelAdd:
		if !contains(message.Labels, strings.TrimSpace(mutation.Label)) {
			message.Labels = append(message.Labels, strings.TrimSpace(mutation.Label))
		}
	case MutationLabelRemove:
		kept := make([]string, 0, len(message.Labels))
		for _, label := range message.Labels {
			if label != strings.TrimSpace(mutation.Label) {
				kept = append(kept, label)
			}
		}
		message.Labels = kept
	default:
		return ErrCapability
	}
	return nil
}

func (s *Service) folderByKind(ctx context.Context, accountID, kind string) (Folder, error) {
	account, err := s.authorizedAccount(ctx, accountID)
	if err != nil {
		return Folder{}, err
	}
	if isPOP3(account) {
		if err := s.ensureLocalPOP3Folders(ctx, account); err != nil {
			return Folder{}, err
		}
	}
	folders, err := s.store.Folders(ctx, accountID)
	if err != nil {
		return Folder{}, err
	}
	settings, err := s.store.AccountSettings(ctx, accountID)
	if err != nil {
		return Folder{}, err
	}
	if mapped := settings.FolderMap[strings.ToLower(kind)]; mapped != "" {
		for _, folder := range folders {
			if folder.ID == mapped {
				return folder, nil
			}
		}
		return Folder{}, fmt.Errorf("%w: mapped %s folder is unavailable", ErrCapability, kind)
	}
	for _, folder := range folders {
		if strings.EqualFold(folder.Kind, kind) {
			return folder, nil
		}
	}
	return Folder{}, fmt.Errorf("%w: account has no %s folder", ErrCapability, kind)
}

func (s *Service) checkMutationPublication(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	policy := s.runtimePolicy()
	if !policy.Enabled {
		return ErrDisabled
	}
	if policy.ReadOnly {
		return ErrReadOnly
	}
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return ErrDisabled
	}
	return nil
}

func (s *Service) Snooze(ctx context.Context, id string, until time.Time) error {
	ctx, cancel, err := s.begin(ctx, true)
	if err != nil {
		return err
	}
	defer cancel()
	if !until.After(time.Now()) {
		return fmt.Errorf("%w: snooze time must be in the future", ErrInvalid)
	}
	m, err := s.store.GetMessage(ctx, id)
	if err != nil {
		return err
	}
	account, err := s.authorizedAccount(ctx, m.AccountID)
	if err != nil {
		return err
	}
	if account.Disabled {
		return ErrDisabled
	}
	if account.ReadOnly {
		return ErrReadOnly
	}
	if !hasCapability(account.Capabilities, "read") {
		return ErrCapability
	}
	if err := s.ensureCurrentRemoteMessage(ctx, account, m); err != nil {
		return err
	}
	if err := s.checkMutationPublication(ctx); err != nil {
		return err
	}
	if err := s.store.SetSnooze(ctx, id, until.UTC()); err != nil {
		return err
	}
	s.emit(Event{Type: "snooze", AccountID: m.AccountID, ID: id, Status: "snoozed"})
	return nil
}

func (s *Service) Unsnooze(ctx context.Context, id string) error {
	ctx, cancel, err := s.begin(ctx, true)
	if err != nil {
		return err
	}
	defer cancel()
	m, err := s.store.GetMessage(ctx, id)
	if err != nil {
		return err
	}
	account, err := s.authorizedAccount(ctx, m.AccountID)
	if err != nil {
		return err
	}
	if account.Disabled {
		return ErrDisabled
	}
	if account.ReadOnly {
		return ErrReadOnly
	}
	if !hasCapability(account.Capabilities, "read") {
		return ErrCapability
	}
	if err := s.ensureCurrentRemoteMessage(ctx, account, m); err != nil {
		return err
	}
	if err := s.checkMutationPublication(ctx); err != nil {
		return err
	}
	if err := s.store.SetSnooze(ctx, id, time.Time{}); err != nil {
		return err
	}
	s.emit(Event{Type: "snooze", AccountID: m.AccountID, ID: id, Status: "unsnoozed"})
	return nil
}

func (s *Service) SetLabels(ctx context.Context, id string, labels []string) error {
	if len(labels) > 100 {
		return fmt.Errorf("%w: too many labels", ErrInvalid)
	}
	m, err := s.store.GetMessage(ctx, id)
	if err != nil {
		return err
	}
	account, err := s.authorizedAccount(ctx, m.AccountID)
	if err != nil {
		return err
	}
	if account.Disabled {
		return ErrDisabled
	}
	if account.ReadOnly {
		return ErrReadOnly
	}
	if !hasCapability(account.Capabilities, "labels_write") {
		return ErrCapability
	}
	wanted := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if !validLabel(label) {
			return fmt.Errorf("%w: invalid label", ErrInvalid)
		}
		if !contains(wanted, label) {
			wanted = append(wanted, label)
		}
	}
	for _, label := range append([]string(nil), m.Labels...) {
		if !contains(wanted, label) {
			if err := s.Mutate(ctx, id, Mutation{Kind: MutationLabelRemove, Label: label}); err != nil {
				return err
			}
		}
	}
	for _, label := range wanted {
		if !contains(m.Labels, label) {
			if err := s.Mutate(ctx, id, Mutation{Kind: MutationLabelAdd, Label: label}); err != nil {
				return err
			}
		}
	}
	return nil
}
