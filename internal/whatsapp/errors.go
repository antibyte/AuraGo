package whatsapp

import "errors"

// Sentinel errors the server maps to fixed HTTP error codes.
var (
	ErrNotConfigured     = errors.New("whatsapp is disabled or not configured")
	ErrAlreadyLinked     = errors.New("a whatsapp device is already linked")
	ErrLinkInProgress    = errors.New("a whatsapp link session is already active")
	ErrInvalidLinkMethod = errors.New("whatsapp link method must be qr or phone")
	ErrInvalidPhone      = errors.New("whatsapp pairing needs an international phone number")
	ErrNotConnected      = errors.New("whatsapp is not connected")
	ErrNoRecipient       = errors.New("whatsapp has no owner recipient")
)
