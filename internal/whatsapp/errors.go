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
	// ErrRecipientNotOnWhatsApp wraps whatsmeow's error when WhatsApp does not
	// know an owner number.
	ErrRecipientNotOnWhatsApp = errors.New("whatsapp owner number is not registered on WhatsApp")
	// ErrMediaTooLarge is returned for a file over the 50 MB WhatsApp limit, declared or real.
	ErrMediaTooLarge = errors.New("the file is larger than 50 MB, the WhatsApp limit")
	// ErrReadOnly is returned for a send while read_only is on (used from Task 4).
	ErrReadOnly = errors.New("whatsapp is in read-only mode")
)

// errNotOwnerRecipient is internal: no API takes a recipient, so it means a bug
// in a caller.
var errNotOwnerRecipient = errors.New("whatsapp refuses a recipient that is not an owner")
