package retronet

import (
	"net"
	"strconv"
)

// Protocol is the wire protocol used to reach an entry.
type Protocol string

// Supported protocols.
const (
	ProtocolTelnet Protocol = "telnet"
	ProtocolSSH    Protocol = "ssh"
)

// Kind selects the terminal behaviour of a Telnet entry.
type Kind string

// Telnet entry kinds.
const (
	KindBBS   Kind = "bbs"   // fixed 80x25, TTYPE "ANSI", character mode, Enter -> CR (CR NUL unless we transmit BINARY)
	KindWorld Kind = "world" // fit window, TTYPE "XTERM-256COLOR", kludge line mode, Enter -> CR LF
	KindC64   Kind = "c64"   // 40x25, raw TCP unless the board opens with IAC, Enter -> CR
)

// Charset is the byte encoding a Telnet service speaks.
type Charset string

// Supported charsets.
const (
	CharsetUTF8    Charset = "utf8"
	CharsetCP437   Charset = "cp437"
	CharsetLatin1  Charset = "latin1"
	CharsetPETSCII Charset = "petscii" // Commodore 64: stateful 40x25 engine, one codec per session
)

// Category values, rendered in this order.
const (
	CategoryClassics  = "classics"
	CategoryBBS       = "bbs"
	CategoryMUDs      = "muds"
	CategoryGames     = "games"
	CategoryCommodore = "commodore" // rendered after CategoryGames, before CategoryOwn
	CategoryOwn       = "own"
)

// Terminal types announced through Telnet TTYPE.
const (
	termTypeANSI    = "ANSI"
	termTypeXterm   = "XTERM-256COLOR"
	termTypePETSCII = "PETSCII" // KindC64; announced only when the board opens with Telnet
)

// Entry is one dialable service from the catalog or the admin's own entries.
type Entry struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`     // own entries only (free text)
	DescriptionKey string   `json:"description_key,omitempty"` // catalog only: i18n key
	Category       string   `json:"category"`
	Protocol       Protocol `json:"protocol"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	Kind           Kind     `json:"kind,omitempty"`     // telnet only
	Charset        Charset  `json:"charset,omitempty"`  // telnet only
	User           string   `json:"user,omitempty"`     // ssh only
	HostKey        string   `json:"host_key,omitempty"` // ssh: "SHA256:<base64>" as printed by ssh.FingerprintSHA256
	Own            bool     `json:"own"`
}

// Address returns net.JoinHostPort(e.Host, strconv.Itoa(e.Port)).
func (e Entry) Address() string {
	return net.JoinHostPort(e.Host, strconv.Itoa(e.Port))
}

// TermType returns "ANSI" for KindBBS, "PETSCII" for KindC64 (sent only when a C64 board
// negotiates Telnet) and "XTERM-256COLOR" otherwise.
func (e Entry) TermType() string {
	switch e.Kind {
	case KindBBS:
		return termTypeANSI
	case KindC64:
		return termTypePETSCII
	default:
		return termTypeXterm
	}
}

// EffectiveCharset returns e.Charset, or CharsetUTF8 for SSH and empty values.
func (e Entry) EffectiveCharset() Charset {
	if e.Protocol == ProtocolSSH || e.Charset == "" {
		return CharsetUTF8
	}
	return e.Charset
}

// valid reports whether k is one of the Telnet entry kinds.
func (k Kind) valid() bool {
	return k == KindBBS || k == KindWorld || k == KindC64
}

// valid reports whether c is one of the supported charsets.
func (c Charset) valid() bool {
	return c == CharsetUTF8 || c == CharsetCP437 || c == CharsetLatin1 || c == CharsetPETSCII
}
