package mailer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type ruleRunRecord struct {
	Run       RuleRun
	AccountID string
	Rule      Rule
}

func (s *Store) RecoverRuleRuns(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE mailer_rule_runs
SET status='unknown',error='rule action outcome unknown after restart',updated_at=?
WHERE status='executing'`, timeNS(time.Now()))
	if err != nil {
		return fmt.Errorf("recover interrupted mail rule actions: %w", err)
	}
	return nil
}

func (s *Store) PendingRuleRuns(ctx context.Context, accountID string) ([]ruleRunRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT message_id,account_id,rule_id,rule_revision,rule_body,action_index,status,error,updated_at
FROM mailer_rule_runs WHERE account_id=? AND status='pending' ORDER BY updated_at,message_id,rule_id`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list pending mail rule actions: %w", err)
	}
	defer rows.Close()
	values := make([]ruleRunRecord, 0)
	for rows.Next() {
		record, err := scanRuleRun(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, record)
	}
	return values, rows.Err()
}

func (s *Store) RuleRun(ctx context.Context, messageID, ruleID string) (ruleRunRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT message_id,account_id,rule_id,rule_revision,rule_body,action_index,status,error,updated_at
FROM mailer_rule_runs WHERE message_id=? AND rule_id=?`, messageID, ruleID)
	record, err := scanRuleRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ruleRunRecord{}, ErrNotFound
	}
	return record, err
}

func (s *Store) ListRuleRuns(ctx context.Context, messageID string) ([]RuleRun, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT message_id,account_id,rule_id,rule_revision,rule_body,action_index,status,error,updated_at
FROM mailer_rule_runs WHERE message_id=? ORDER BY rule_id`, messageID)
	if err != nil {
		return nil, fmt.Errorf("list mail rule runs: %w", err)
	}
	defer rows.Close()
	values := make([]RuleRun, 0)
	for rows.Next() {
		record, err := scanRuleRun(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, record.Run)
	}
	return values, rows.Err()
}

type ruleRunScanner interface {
	Scan(...any) error
}

func scanRuleRun(scanner ruleRunScanner) (ruleRunRecord, error) {
	var record ruleRunRecord
	var body []byte
	var updated int64
	if err := scanner.Scan(&record.Run.MessageID, &record.AccountID, &record.Run.RuleID, &record.Run.RuleRevision,
		&body, &record.Run.ActionIndex, &record.Run.Status, &record.Run.Error, &updated); err != nil {
		return ruleRunRecord{}, err
	}
	record.Run.ActionCount = 0
	record.Run.UpdatedAt = fromNS(updated)
	if err := unmarshal(body, &record.Rule); err != nil {
		return ruleRunRecord{}, err
	}
	record.Run.ActionCount = len(record.Rule.Actions)
	return record, nil
}

func (s *Store) SetRuleRunExecuting(ctx context.Context, messageID, ruleID string, actionIndex int) error {
	res, err := s.db.ExecContext(ctx, `UPDATE mailer_rule_runs SET status='executing',error='',updated_at=?
WHERE message_id=? AND rule_id=? AND status='pending' AND action_index=?`, timeNS(time.Now()), messageID, ruleID, actionIndex)
	if err != nil {
		return fmt.Errorf("start mail rule action: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Store) AdvanceRuleRun(ctx context.Context, messageID, ruleID string, actionIndex int) error {
	res, err := s.db.ExecContext(ctx, `UPDATE mailer_rule_runs SET action_index=?,status='pending',error='',updated_at=?
WHERE message_id=? AND rule_id=? AND status='executing' AND action_index=?`, actionIndex+1, timeNS(time.Now()), messageID, ruleID, actionIndex)
	if err != nil {
		return fmt.Errorf("advance mail rule action: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Store) FinishRuleRun(ctx context.Context, messageID, ruleID, status, message string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE mailer_rule_runs SET status=?,error=?,updated_at=?
WHERE message_id=? AND rule_id=? AND status IN ('pending','executing')`, status, message, timeNS(time.Now()), messageID, ruleID)
	if err != nil {
		return fmt.Errorf("save mail rule result: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Store) RetryRuleRun(ctx context.Context, messageID, ruleID string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE mailer_rule_runs SET status='pending',error='',updated_at=?
WHERE message_id=? AND rule_id=? AND status IN ('unknown','failed')`, timeNS(time.Now()), messageID, ruleID)
	if err != nil {
		return fmt.Errorf("retry mail rule action: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Store) SkipRuleRun(ctx context.Context, messageID, ruleID, message string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE mailer_rule_runs SET status='skipped',error=?,updated_at=?
WHERE message_id=? AND rule_id=? AND status IN ('pending','executing','unknown','failed')`, message, timeNS(time.Now()), messageID, ruleID)
	if err != nil {
		return fmt.Errorf("skip mail rule action: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Store) HasRuleRunErrors(ctx context.Context, accountID string) (bool, error) {
	var found int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM mailer_rule_runs
WHERE account_id=? AND status IN ('unknown','failed') LIMIT 1`, accountID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) SkipPendingRuleRuns(ctx context.Context, accountID, message string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE mailer_rule_runs SET status='skipped',error=?,updated_at=?
WHERE account_id=? AND status='pending'`, message, timeNS(time.Now()), accountID)
	if err != nil {
		return fmt.Errorf("skip pending mail rule actions: %w", err)
	}
	return nil
}
