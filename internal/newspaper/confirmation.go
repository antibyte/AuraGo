package newspaper

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"time"
)

func (s *Store) NewEmailChallenge(ctx context.Context, address, accountID string) (string, error) {
	var random [7]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate email confirmation code: %w", err)
	}
	code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(random[:])[:10]
	h := sha256.Sum256([]byte(code))
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, "INSERT INTO newspaper_email_challenges(address,account_id,digest,expires_at,attempts) VALUES(?,?,?,?,0) ON CONFLICT(address) DO UPDATE SET account_id=excluded.account_id,digest=excluded.digest,expires_at=excluded.expires_at,attempts=0 WHERE julianday(newspaper_email_challenges.expires_at)<=julianday(?)", address, accountID, h[:], now.Add(10*time.Minute).Format(time.RFC3339Nano), now.Add(9*time.Minute).Format(time.RFC3339Nano))
	if err != nil {
		return "", fmt.Errorf("save email confirmation code: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("check email confirmation code: %w", err)
	}
	if changed == 0 {
		return "", ErrChallengePending
	}
	return code, nil
}

// CancelEmailChallenge removes only the code from this send attempt.
func (s *Store) CancelEmailChallenge(ctx context.Context, address, accountID, code string) (bool, error) {
	h := sha256.Sum256([]byte(code))
	result, err := s.db.ExecContext(ctx, "DELETE FROM newspaper_email_challenges WHERE address=? AND account_id=? AND digest=?", address, accountID, h[:])
	if err != nil {
		return false, fmt.Errorf("cancel email confirmation code: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check cancelled email confirmation code: %w", err)
	}
	return changed == 1, nil
}

func (s *Store) ConfirmEmail(ctx context.Context, address, accountID, code string) (Profile, error) {
	var digest []byte
	var expires string
	var attempts int
	err := s.db.QueryRowContext(ctx, "SELECT digest,expires_at,attempts FROM newspaper_email_challenges WHERE address=? AND account_id=?", address, accountID).Scan(&digest, &expires, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, err
	}
	until, _ := time.Parse(time.RFC3339Nano, expires)
	if time.Now().After(until) || attempts >= 5 {
		return Profile{}, ErrConflict
	}
	_, _ = s.db.ExecContext(ctx, "UPDATE newspaper_email_challenges SET attempts=attempts+1 WHERE address=?", address)
	h := sha256.Sum256([]byte(code))
	if subtle.ConstantTimeCompare(digest, h[:]) != 1 {
		return Profile{}, errors.New("confirmation code is incorrect")
	}
	p, err := s.MarkEmailVerified(ctx, address, accountID)
	if err != nil {
		return Profile{}, err
	}
	_, _ = s.db.ExecContext(ctx, "DELETE FROM newspaper_email_challenges WHERE address=?", address)
	return p, nil
}
