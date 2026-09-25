package newspaper

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"errors"
	"time"
)

func (s *Store) NewEmailChallenge(ctx context.Context, address, accountID string) (string, error) {
	var existing string
	err := s.db.QueryRowContext(ctx, "SELECT expires_at FROM newspaper_email_challenges WHERE address=?", address).Scan(&existing)
	if err == nil {
		expires, _ := time.Parse(time.RFC3339Nano, existing)
		if time.Until(expires) > 9*time.Minute {
			return "", ErrConflict
		}
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	var random [7]byte
	if _, err = rand.Read(random[:]); err != nil {
		return "", err
	}
	code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(random[:])[:10]
	h := sha256.Sum256([]byte(code))
	_, err = s.db.ExecContext(ctx, "INSERT INTO newspaper_email_challenges(address,account_id,digest,expires_at,attempts) VALUES(?,?,?,?,0) ON CONFLICT(address) DO UPDATE SET account_id=excluded.account_id,digest=excluded.digest,expires_at=excluded.expires_at,attempts=0", address, accountID, h[:], time.Now().UTC().Add(10*time.Minute).Format(time.RFC3339Nano))
	return code, err
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
