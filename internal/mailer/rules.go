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
		gate := s.ruleGate(r.ID)
		gate.Lock()
		defer gate.Unlock()
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
	gate := s.ruleGate(id)
	gate.Lock()
	defer gate.Unlock()
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

func (s *Service) ListRuleRuns(ctx context.Context, messageID string) ([]RuleRun, error) {
	m, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	account, err := s.authorizedAccount(ctx, m.AccountID)
	if err != nil {
		return nil, err
	}
	if account.Disabled || !hasCapability(account.Capabilities, "read") {
		return nil, ErrNotFound
	}
	return s.store.ListRuleRuns(ctx, messageID)
}

func (s *Service) RetryRuleRun(ctx context.Context, messageID, ruleID string) error {
	ctx, cancel, err := s.begin(ctx, true)
	if err != nil {
		return err
	}
	defer cancel()
	m, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		return err
	}
	if _, _, err := s.writableAccount(ctx, m.AccountID, "read"); err != nil {
		return err
	}
	gate := s.ruleGate(ruleID)
	gate.Lock()
	defer gate.Unlock()
	run, err := s.store.RuleRun(ctx, messageID, ruleID)
	if err != nil {
		return err
	}
	rule, err := s.store.GetRule(ctx, ruleID)
	if err != nil || !rule.Enabled || rule.Revision != run.Run.RuleRevision || rule.AccountID != "" && rule.AccountID != run.AccountID {
		_ = s.store.SkipRuleRun(context.WithoutCancel(ctx), messageID, ruleID, "rule changed or disabled before retry")
		s.refreshRuleRunSyncStatus(context.WithoutCancel(ctx), run.AccountID)
		return ErrConflict
	}
	if err := s.store.RetryRuleRun(ctx, messageID, ruleID); err != nil {
		return err
	}
	if err := s.processRuleRunLocked(ctx, messageID, ruleID); err != nil {
		return err
	}
	s.refreshRuleRunSyncStatus(context.WithoutCancel(ctx), run.AccountID)
	return nil
}

func (s *Service) processPendingRuleRuns(ctx context.Context, accountID string) error {
	runs, err := s.store.PendingRuleRuns(ctx, accountID)
	if err != nil {
		return err
	}
	var firstErr error
	for _, run := range runs {
		if err := s.processRuleRun(ctx, run.Run.MessageID, run.Run.RuleID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		s.refreshRuleRunSyncStatus(context.WithoutCancel(ctx), accountID)
	}
	return firstErr
}

func (s *Service) processRuleRun(ctx context.Context, messageID, ruleID string) error {
	gate := s.ruleGate(ruleID)
	gate.Lock()
	defer gate.Unlock()
	return s.processRuleRunLocked(ctx, messageID, ruleID)
}

func (s *Service) processRuleRunLocked(ctx context.Context, messageID, ruleID string) error {
	run, err := s.store.RuleRun(ctx, messageID, ruleID)
	if err != nil {
		return err
	}
	if run.Run.Status != "pending" {
		return nil
	}
	current, err := s.store.GetRule(ctx, ruleID)
	if errors.Is(err, ErrNotFound) || err == nil && (!current.Enabled || current.Revision != run.Run.RuleRevision || (current.AccountID != "" && current.AccountID != run.AccountID)) {
		if finishErr := s.store.FinishRuleRun(context.WithoutCancel(ctx), messageID, ruleID, "skipped", "rule changed or disabled before execution"); finishErr != nil {
			return finishErr
		}
		s.emit(Event{Type: "rule_run", AccountID: run.AccountID, ID: messageID, Status: "skipped"})
		return nil
	}
	if err != nil {
		return err
	}
	m, err := s.store.GetMessage(ctx, messageID)
	if err != nil {
		_ = s.store.FinishRuleRun(context.WithoutCancel(ctx), messageID, ruleID, "failed", "message could not be loaded; retry is safe")
		s.reportRuleRunFailure(context.WithoutCancel(ctx), run, "failed")
		return err
	}
	if needsBody(run.Rule) {
		m, err = s.GetMessage(ctx, messageID)
		if err != nil {
			_ = s.store.FinishRuleRun(context.WithoutCancel(ctx), messageID, ruleID, "failed", "message body could not be loaded; retry is safe")
			s.reportRuleRunFailure(context.WithoutCancel(ctx), run, "failed")
			return err
		}
	}
	matched, err := matchesRule(run.Rule, m)
	if err != nil {
		_ = s.store.FinishRuleRun(context.WithoutCancel(ctx), messageID, ruleID, "failed", "rule conditions could not be evaluated; retry is safe")
		s.reportRuleRunFailure(context.WithoutCancel(ctx), run, "failed")
		return err
	}
	if !matched {
		if err := s.store.FinishRuleRun(context.WithoutCancel(ctx), messageID, ruleID, "skipped", "conditions did not match"); err != nil {
			return err
		}
		s.emit(Event{Type: "rule_run", AccountID: run.AccountID, ID: messageID, Status: "skipped"})
		return nil
	}

	for run.Run.ActionIndex < len(run.Rule.Actions) {
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err = s.store.GetRule(ctx, ruleID)
		if errors.Is(err, ErrNotFound) || err == nil && (!current.Enabled || current.Revision != run.Run.RuleRevision || (current.AccountID != "" && current.AccountID != run.AccountID)) {
			if finishErr := s.store.FinishRuleRun(context.WithoutCancel(ctx), messageID, ruleID, "skipped", "rule changed or disabled before execution"); finishErr != nil {
				return finishErr
			}
			s.emit(Event{Type: "rule_run", AccountID: run.AccountID, ID: messageID, Status: "skipped"})
			return nil
		}
		if err != nil {
			return err
		}
		action := run.Rule.Actions[run.Run.ActionIndex]
		capability := "read"
		if action.Kind != "snooze" {
			capability = mutationCapability(action.Kind)
		}
		if _, _, err := s.writableAccount(ctx, run.AccountID, capability); err != nil {
			_ = s.store.FinishRuleRun(context.WithoutCancel(ctx), messageID, ruleID, "failed", "account is not writable; retry is safe")
			s.reportRuleRunFailure(context.WithoutCancel(ctx), run, "failed")
			return err
		}
		if err := s.store.SetRuleRunExecuting(ctx, messageID, ruleID, run.Run.ActionIndex); err != nil {
			return err
		}
		err := s.applyRuleAction(ctx, m, action)
		if err != nil {
			_ = s.store.FinishRuleRun(context.WithoutCancel(ctx), messageID, ruleID, "unknown", "action outcome may be unknown; verify the message before retrying")
			s.reportRuleRunFailure(context.WithoutCancel(ctx), run, "unknown")
			return err
		}
		if err := s.store.AdvanceRuleRun(context.WithoutCancel(ctx), messageID, ruleID, run.Run.ActionIndex); err != nil {
			_ = s.store.FinishRuleRun(context.WithoutCancel(ctx), messageID, ruleID, "unknown", "action succeeded but progress could not be saved; verify the message before retrying")
			s.reportRuleRunFailure(context.WithoutCancel(ctx), run, "unknown")
			return err
		}
		run.Run.ActionIndex++
		run.Run.Status = "pending"
	}
	if err := s.store.FinishRuleRun(context.WithoutCancel(ctx), messageID, ruleID, "completed", ""); err != nil {
		return err
	}
	s.emit(Event{Type: "rule_run", AccountID: run.AccountID, ID: messageID, Status: "completed"})
	return nil
}

func (s *Service) applyRuleAction(ctx context.Context, m Message, action RuleAction) error {
	if action.Kind == "snooze" {
		return s.Snooze(ctx, m.ID, action.SnoozeUntil)
	}
	if !ruleMutations[action.Kind] {
		return ErrCapability
	}
	return s.Mutate(ctx, m.ID, Mutation{Kind: action.Kind, DestinationFolderID: action.DestinationFolderID, Label: action.Label})
}

func (s *Service) reportRuleRunFailure(ctx context.Context, run ruleRunRecord, status string) {
	_ = s.store.SetAccountSyncStatus(ctx, run.AccountID, "error", "rule_error", time.Now().UTC())
	s.emit(Event{Type: "account_sync", AccountID: run.AccountID, Status: "error"})
	s.emit(Event{Type: "rule_run", AccountID: run.AccountID, ID: run.Run.MessageID, Status: status})
}

func (s *Service) refreshRuleRunSyncStatus(ctx context.Context, accountID string) {
	hasErrors, err := s.store.HasRuleRunErrors(ctx, accountID)
	if err != nil {
		return
	}
	if hasErrors {
		_ = s.store.SetAccountSyncStatus(ctx, accountID, "error", "rule_error", time.Now().UTC())
		s.emit(Event{Type: "account_sync", AccountID: accountID, Status: "error"})
		return
	}
	account, err := s.store.Account(ctx, accountID)
	if err == nil && account.SyncError == "rule_error" {
		_ = s.store.SetAccountSyncStatus(ctx, accountID, "ok", "", time.Now().UTC())
		s.emit(Event{Type: "account_sync", AccountID: accountID, Status: "ok"})
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
