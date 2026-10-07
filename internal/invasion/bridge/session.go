package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const ProtocolVersion = 2
const UpgradeRequired = "invasion_protocol_upgrade_required: upgrade master and egg to protocol v2"

// Session binds ordered messages to a fresh server challenge and both identities.
// The owning connection serializes Prepare with writes and Accept with key changes.
type Session struct {
	ID, EggID, NestID, Role string
	sent, received          uint64
}

func NewChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func NewSession(id, eggID, nestID, role string) (*Session, error) {
	b, err := hex.DecodeString(id)
	if err != nil || len(b) != 32 || eggID == "" || nestID == "" || (role != "master" && role != "egg") {
		return nil, fmt.Errorf("invalid invasion session")
	}
	return &Session{ID: id, EggID: eggID, NestID: nestID, Role: role}, nil
}

func (s *Session) Prepare(msg *Message, key string) error {
	if s == nil {
		return fmt.Errorf("authenticated session required")
	}
	s.sent++
	msg.Protocol, msg.Session, msg.Sender, msg.Sequence = ProtocolVersion, s.ID, s.Role, s.sent
	msg.EggID, msg.NestID = s.EggID, s.NestID
	msg.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	return SignMessage(msg, key)
}

func (s *Session) Accept(msg Message, key, previousKey string) error {
	if msg.Protocol != ProtocolVersion {
		return fmt.Errorf("%s", UpgradeRequired)
	}
	if s == nil || msg.Session != s.ID || msg.EggID != s.EggID || msg.NestID != s.NestID ||
		(msg.Sender != "master" && msg.Sender != "egg") || msg.Sender == s.Role ||
		msg.Sequence == 0 || msg.Sequence <= s.received || msg.ID == "" || len(msg.ID) > 128 {
		return fmt.Errorf("invalid session identity or replayed message")
	}
	stamp, err := time.Parse(time.RFC3339Nano, msg.Timestamp)
	if err != nil || time.Since(stamp) > 2*time.Minute || time.Until(stamp) > 30*time.Second {
		return fmt.Errorf("message timestamp outside acceptance window")
	}
	ok, err := VerifyMessage(msg, key)
	if (err != nil || !ok) && previousKey != "" {
		ok, err = VerifyMessage(msg, previousKey)
	}
	if err != nil || !ok {
		return fmt.Errorf("message authentication failed")
	}
	s.received = msg.Sequence
	return nil
}
