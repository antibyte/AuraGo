package mailer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ruleMutations = map[MutationKind]bool{
	MutationRead: true, MutationUnread: true, MutationStar: true, MutationUnstar: true,
	MutationMove: true, MutationArchive: true, MutationTrash: true, MutationJunk: true,
	MutationLabelAdd: true, MutationLabelRemove: true,
}

func validateRule(r Rule) error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("rule name is required")
	}
	if len(r.Conditions) == 0 || len(r.Conditions) > 20 {
		return errors.New("rule must have between 1 and 20 conditions")
	}
	if len(r.Actions) == 0 || len(r.Actions) > 20 {
		return errors.New("rule must have between 1 and 20 actions")
	}
	for _, c := range r.Conditions {
		switch strings.ToLower(c.Field) {
		case "from", "to", "subject", "body", "has_attachments":
		default:
			return fmt.Errorf("unsupported rule field %q", c.Field)
		}
		switch strings.ToLower(c.Operator) {
		case "contains", "equals", "starts_with":
		default:
			return fmt.Errorf("unsupported rule operator %q", c.Operator)
		}
		if len(c.Value) > 2048 {
			return errors.New("rule condition value is too long")
		}
		if strings.EqualFold(c.Field, "has_attachments") && (strings.ToLower(c.Operator) != "equals" || (strings.ToLower(c.Value) != "true" && strings.ToLower(c.Value) != "false")) {
			return errors.New("has_attachments requires equals true or false")
		}
	}
	for _, a := range r.Actions {
		if !ruleMutations[a.Kind] && a.Kind != "snooze" {
			return fmt.Errorf("rule action %q is not allowed", a.Kind)
		}
		if a.Kind == MutationMove && a.DestinationFolderID == "" {
			return fmt.Errorf("rule action %q needs a destination folder", a.Kind)
		}
		if (a.Kind == MutationLabelAdd || a.Kind == MutationLabelRemove) && !validLabel(a.Label) {
			return fmt.Errorf("rule action %q needs a valid label", a.Kind)
		}
		if a.Kind == "snooze" && !a.SnoozeUntil.After(time.Now()) {
			return errors.New("rule snooze time must be in the future")
		}
	}
	return nil
}

func (s *Service) ListRules(ctx context.Context) ([]Rule, error) {
	rules, err := s.store.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		if rule.AccountID == "" {
			out = append(out, rule)
			continue
		}
		account, err := s.authorizedAccount(ctx, rule.AccountID)
		if err == nil && !account.Disabled && hasCapability(account.Capabilities, "read") {
			out = append(out, rule)
		}
	}
	return out, nil
}

func (s *Service) SaveRule(ctx context.Context, r Rule) (Rule, error) {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return Rule{}, err
	}
	defer cancel()
	if err := validateRule(r); err != nil {
		return Rule{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if r.ID != "" {
		old, err := s.store.GetRule(ctx, r.ID)
		if err == nil && old.AccountID != r.AccountID {
			return Rule{}, fmt.Errorf("%w: rule account cannot be changed", ErrInvalid)
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return Rule{}, err
		}
	}
	if r.AccountID == "" {
		if err := s.requireAnyWritableAccount(ctx, "read"); err != nil {
			return Rule{}, err
		}
	} else if _, _, err := s.writableAccount(ctx, r.AccountID, "read"); err != nil {
		return Rule{}, err
	}
	saved, err := s.store.SaveRule(ctx, r)
	if err == nil {
		s.emit(Event{Type: "rule", AccountID: saved.AccountID, ID: saved.ID, Status: "saved"})
	}
	return saved, err
}

func (s *Service) DeleteRule(ctx context.Context, id string, revision int64) error {
	ctx, cancel, err := s.begin(ctx, false)
	if err != nil {
		return err
	}
	defer cancel()
	rule, err := s.store.GetRule(ctx, id)
	if err != nil {
		return err
	}
	if rule.AccountID == "" {
		if err := s.requireAnyWritableAccount(ctx, "read"); err != nil {
			return err
		}
	} else if _, _, err := s.writableAccount(ctx, rule.AccountID, "read"); err != nil {
		return err
	}
	if revision < 1 || rule.Revision != revision {
		return ErrConflict
	}
	if err := s.store.DeleteRule(ctx, id, revision); err != nil {
		return err
	}
	s.emit(Event{Type: "rule", ID: id, Status: "deleted"})
	return nil
}

func (s *Service) PreviewRule(ctx context.Context, r Rule, messageIDs []string) ([]RulePreview, error) {
	if r.ID != "" {
		saved, err := s.store.GetRule(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		if r.Revision != saved.Revision || r.AccountID != saved.AccountID {
			return nil, ErrConflict
		}
	}
	if err := validateRule(r); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if len(messageIDs) == 0 {
		return nil, fmt.Errorf("%w: preview requires at least one message ID", ErrInvalid)
	}
	if len(messageIDs) > 500 {
		return nil, fmt.Errorf("%w: preview is limited to 500 messages", ErrInvalid)
	}
	out := make([]RulePreview, 0, len(messageIDs))
	for _, id := range messageIDs {
		m, err := s.GetMessage(ctx, id)
		if err != nil {
			return nil, err
		}
		match, err := matchesRule(r, m)
		if err != nil {
			return nil, err
		}
		entry := RulePreview{MessageID: id, Matches: match}
		if match {
			entry.Actions = append([]RuleAction(nil), r.Actions...)
		}
		out = append(out, entry)
	}
	return out, nil
}

func (s *Service) ApplyRule(ctx context.Context, id string, revision int64, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return fmt.Errorf("%w: historical rule application requires explicit message IDs", ErrInvalid)
	}
	if len(messageIDs) > 500 {
		return fmt.Errorf("%w: rule application is limited to 500 messages", ErrInvalid)
	}
	ctx, cancel, err := s.begin(ctx, true)
	if err != nil {
		return err
	}
	defer cancel()
	r, err := s.store.GetRule(ctx, id)
	if err != nil {
		return err
	}
	if revision < 1 || r.Revision != revision {
		return ErrConflict
	}
	if err := validateRule(r); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if r.AccountID != "" {
		if _, _, err := s.writableAccount(ctx, r.AccountID, "read"); err != nil {
			return err
		}
	}
	for _, messageID := range messageIDs {
		m, err := s.GetMessage(ctx, messageID)
		if err != nil {
			return err
		}
		if r.AccountID != "" && r.AccountID != m.AccountID {
			continue
		}
		ok, err := matchesRule(r, m)
		if err != nil {
			return err
		}
		if ok {
			if err := s.applyRuleActions(ctx, m, r.Actions); err != nil {
				return err
			}
		}
	}
	s.emit(Event{Type: "rule", ID: r.ID, Status: "applied"})
	return nil
}

func (s *Service) applyNewRules(ctx context.Context, m Message) {
	rules, err := s.store.ListRules(ctx)
	if err != nil {
		return
	}
	for _, r := range rules {
		if !r.Enabled || (r.AccountID != "" && r.AccountID != m.AccountID) {
			continue
		}
		candidate := m
		if needsBody(r) {
			candidate, err = s.GetMessage(ctx, m.ID)
			if err != nil {
				continue
			}
		}
		ok, err := matchesRule(r, candidate)
		if err != nil || !ok {
			continue
		}
		_ = s.applyRuleActions(ctx, candidate, r.Actions)
	}
}

func (s *Service) applyRuleActions(ctx context.Context, m Message, actions []RuleAction) error {
	for _, a := range actions {
		if a.Kind == "snooze" {
			if err := s.Snooze(ctx, m.ID, a.SnoozeUntil); err != nil {
				return err
			}
			continue
		}
		if !ruleMutations[a.Kind] {
			return ErrCapability
		}
		if err := s.Mutate(ctx, m.ID, Mutation{Kind: a.Kind, DestinationFolderID: a.DestinationFolderID, Label: a.Label}); err != nil {
			return err
		}
	}
	return nil
}

func matchesRule(r Rule, m Message) (bool, error) {
	for _, c := range r.Conditions {
		field := strings.ToLower(c.Field)
		var actual string
		switch field {
		case "from":
			actual = m.From
		case "to":
			actual = strings.Join(append(append([]string{}, m.To...), m.CC...), " ")
		case "subject":
			actual = m.Subject
		case "body":
			actual = m.Text + " " + m.HTML
		case "has_attachments":
			actual = fmt.Sprint(len(m.Attachments) > 0)
		}
		want := strings.ToLower(c.Value)
		got := strings.ToLower(actual)
		match := false
		switch strings.ToLower(c.Operator) {
		case "contains":
			match = strings.Contains(got, want)
		case "equals":
			match = got == want
		case "starts_with":
			match = strings.HasPrefix(got, want)
		default:
			return false, fmt.Errorf("unsupported rule operator %q", c.Operator)
		}
		if !match {
			return false, nil
		}
	}
	return true, nil
}

func needsBody(r Rule) bool {
	for _, c := range r.Conditions {
		if strings.EqualFold(c.Field, "body") {
			return true
		}
	}
	return false
}
