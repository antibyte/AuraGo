// Package retronet implements the server side of the desktop Terminal's
// Retro-Net dialer: the curated catalog and the admin's own entries, a guarded
// dialer that reaches public addresses only, the Telnet option negotiator and
// the charset codecs that turn CP437 and Latin-1 output into UTF-8.
//
// The browser never supplies a host or port. A session is always dialed by
// entry ID from DefaultCatalog or the validated own-entry document, and every
// dial goes through Dialer, which resolves once, rejects restricted addresses
// and mail ports, and connects to the pinned address only.
//
// The package is pure logic without HTTP. It imports only the standard
// library, aurago/internal/security and golang.org/x/crypto/ssh, and it must
// never import aurago/internal/desktop or aurago/internal/server.
package retronet
