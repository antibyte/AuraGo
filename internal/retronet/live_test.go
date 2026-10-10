package retronet

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	liveEnv            = "AURAGO_RETRONET_LIVE"
	liveReadableTarget = 40               // readable runes that prove an entry works
	liveWindow         = 10 * time.Second // output window after "connected"
	liveBackstop       = 40 * time.Second // whole session; dial and handshake have their own 10 s limits
	liveParallel       = 8
	liveTextLimit      = 256 << 10

	// C64 boards (KindC64) often answer only after several seconds and stop at a detection
	// prompt, and their output is font glyphs, not readable runes. A board passes with at least
	// livePETSCIITarget printable PETSCII characters (source bytes, counted before the codec)
	// within liveC64Window of "connected", or with a BUSY banner (a single-line board in use).
	livePETSCIITarget = 10
	liveC64Window     = 45 * time.Second
	liveC64Backstop   = 60 * time.Second // window plus the 10 s dial budget and some slack
)

// liveKick holds the keystrokes sent once, right after "connected", to entries that print only
// a prompt until the user types. The bytes are exactly what the browser sends (Enter is "\r",
// Backspace - the C64 DEL key - is "\x7f"). Use harmless, read-only commands; a C64 board may
// only get one detection key ("\r" or "\x7f"), never an answer to a login or new-user prompt.
// Verified 2026-10-09/10:
//   - telehack-ssh shows only its "." prompt; "date" answers with about 39 readable runes
//     (too few, and the length varies with the date), "help" lists the commands (> 130 runes).
//   - the C64 boards need no kick: each sent at least 12 printable PETSCII characters (banner
//     or detection prompt) within 6 s of connecting.
var liveKick = map[string][]byte{
	"telehack-ssh": []byte("help\r"),
}

// liveRule is how long an entry may print after "connected" and how many readable runes end
// that window early (0: never; C64 boards are counted on the wire instead).
type liveRule struct {
	window time.Duration
	target int
}

// liveRuleFor returns the C64 rule for KindC64 entries and the readable-text rule otherwise.
func liveRuleFor(e Entry) liveRule {
	if e.Kind == KindC64 {
		return liveRule{window: liveC64Window}
	}
	return liveRule{window: liveWindow, target: liveReadableTarget}
}

// liveClient is a browser that types nothing except an optional kick. Its output window starts
// with "connected"; it stops the session once enough readable text arrived or the window ends.
type liveClient struct {
	events chan ClientEvent // buffered for the single kick
	kick   []byte
	rule   liveRule
	cancel context.CancelFunc

	mu        sync.Mutex
	connected bool
	window    *time.Timer
	bytes     int
	readable  int
	text      []byte
}

func newLiveClient(e Entry, cancel context.CancelFunc) *liveClient {
	return &liveClient{events: make(chan ClientEvent, 1), kick: liveKick[e.ID], rule: liveRuleFor(e), cancel: cancel}
}

func (c *liveClient) Events() <-chan ClientEvent { return c.events }

func (c *liveClient) SendData(p []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bytes += len(p)
	if len(c.text) < liveTextLimit {
		c.text = append(c.text, p...)
	}
	c.readable = liveReadableRunes(string(c.text))
	if c.rule.target > 0 && c.readable >= c.rule.target {
		c.cancel()
	}
	return nil
}

func (c *liveClient) SendControl(ctl Control) error {
	if ctl.Type != controlConnected {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.connected {
		return nil
	}
	c.connected = true
	c.window = time.AfterFunc(c.rule.window, c.cancel)
	if c.kick != nil {
		c.events <- ClientEvent{Data: bytes.Clone(c.kick)} // buffered: never blocks the Run goroutine
	}
	return nil
}

// stop ends the output window timer.
func (c *liveClient) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.window != nil {
		c.window.Stop()
	}
}

// livePETSCIICount counts what a C64 board sends on the wire, before the codec: printable
// PETSCII characters, and whether a BUSY banner arrived. enough ends the session once the board
// has proved itself. It decides like runC64: when the first chunk opens Telnet, its own Telnet
// strips the negotiation and only the board's data counts (the session answers the board), so
// negotiation alone never passes; otherwise every byte is PETSCII data.
type livePETSCIICount struct {
	enough context.CancelFunc

	mu        sync.Mutex
	started   bool    // the first chunk has been seen
	telnet    *Telnet // set when the first chunk opened Telnet
	printable int
	letters   []byte // the board's bytes, PETSCII letters folded to lower-case ASCII
	busy      bool
}

// dial is the Dialer.Dial of a C64 live session: a plain TCP connect (the guard has already
// validated and pinned the address) whose reads are counted.
func (c *livePETSCIICount) dial(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return livePETSCIIConn{Conn: conn, count: c}, nil
}

// add counts one chunk from the board.
func (c *livePETSCIICount) add(p []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		c.started = true
		if opensTelnet(p) {
			c.telnet = NewTelnet(termTypePETSCII, c64Cols, c64Rows)
		}
	}
	if c.telnet != nil {
		p, _ = c.telnet.Feed(p) // replies are the session's job; count the board's data only
	}
	for _, b := range p {
		if livePrintablePETSCII(b) {
			c.printable++
		}
		if len(c.letters) < liveTextLimit {
			c.letters = append(c.letters, liveFoldPETSCII(b))
		}
	}
	c.busy = c.busy || bytes.Contains(c.letters, []byte("busy"))
	if (c.printable >= livePETSCIITarget || c.busy) && c.enough != nil {
		c.enough()
	}
}

func (c *livePETSCIICount) result() (printable int, busy bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.printable, c.busy
}

// livePETSCIIConn hands every chunk read from a C64 board to its count.
type livePETSCIIConn struct {
	net.Conn
	count *livePETSCIICount
}

func (c livePETSCIIConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.count.add(p[:n])
	}
	return n, err
}

// livePrintablePETSCII reports a printable PETSCII character other than a space: 0x21-0x7F and
// 0xA1-0xFF (0xFF is pi).
func livePrintablePETSCII(b byte) bool {
	return (b >= 0x21 && b <= 0x7F) || b >= 0xA1
}

// liveFoldPETSCII folds PETSCII letters of both case ranges (0x41-0x5A, 0x61-0x7A, 0xC1-0xDA)
// to lower-case ASCII, so "BUSY" is found in either character set.
func liveFoldPETSCII(b byte) byte {
	switch {
	case b >= 0x41 && b <= 0x5A:
		return b + 0x20
	case b >= 0xC1 && b <= 0xDA:
		return b - 0x60
	default:
		return b
	}
}

// liveRow is one line of the live report.
type liveRow struct {
	c64       bool
	connected bool
	bytes     int
	readable  int
	petscii   int  // C64 boards: printable PETSCII characters received
	busy      bool // C64 boards: a BUSY banner arrived
	result    Result
}

// passed: at least liveReadableTarget readable runes within the window; C64 boards need
// livePETSCIITarget printable PETSCII characters or a BUSY banner.
func (r liveRow) passed() bool {
	if r.c64 {
		return r.petscii >= livePETSCIITarget || r.busy
	}
	return r.readable >= liveReadableTarget
}

// liveDial runs one catalog entry; the only input is its liveKick, if any. A C64 board gets its
// own Manager whose dialer counts the board's bytes.
func liveDial(m *Manager, e Entry) liveRow {
	backstop := liveBackstop
	if e.Kind == KindC64 {
		backstop = liveC64Backstop
	}
	ctx, cancel := context.WithTimeout(context.Background(), backstop)
	defer cancel()
	var count *livePETSCIICount
	if e.Kind == KindC64 {
		count = &livePETSCIICount{enough: cancel}
		m = &Manager{Dialer: Dialer{Dial: count.dial}}
	}
	client := newLiveClient(e, cancel)
	res := m.Run(ctx, e, Size{Cols: 80, Rows: 25}, client)
	client.stop()
	client.mu.Lock()
	defer client.mu.Unlock()
	row := liveRow{c64: count != nil, connected: client.connected, bytes: client.bytes, readable: client.readable, result: res}
	if count != nil {
		row.petscii, row.busy = count.result()
	}
	return row
}

// liveReadableRunes counts visible, non-space runes outside ANSI escape sequences.
func liveReadableRunes(s string) int {
	count := 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			i = liveSkipEscape(s, i)
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r != utf8.RuneError && unicode.IsGraphic(r) && !unicode.IsSpace(r) {
			count++
		}
	}
	return count
}

// liveSkipEscape returns the index after the escape sequence that starts with ESC at s[i].
func liveSkipEscape(s string, i int) int {
	i++
	if i >= len(s) {
		return i
	}
	switch s[i] {
	case '[': // CSI: parameters until a final byte 0x40-0x7E
		for i++; i < len(s); i++ {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return i + 1
			}
		}
		return i
	case ']': // OSC: until BEL or ESC \
		for i++; i < len(s); i++ {
			if s[i] == 0x07 {
				return i + 1
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
		}
		return i
	case '(', ')': // character set designation: one more byte
		return min(i+2, len(s))
	default: // two-byte sequence such as ESC 7 or ESC c
		return i + 1
	}
}

func TestLiveReadableRunesSkipsEscapesAndSpaces(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"Hello, World", 11},
		{"\x1b[1;33mAB\x1b[0m", 2},
		{"\x1b]0;title\x07Z", 1},
		{"╔═╗ ok", 5},
		{"\x1b(B\x1b7x", 1},
		{"\r\n\t  ", 0},
		{"\x1b[", 0},
	}
	for _, tc := range cases {
		if got := liveReadableRunes(tc.in); got != tc.want {
			t.Errorf("liveReadableRunes(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestLiveKickTargetsCatalogEntries(t *testing.T) {
	catalog := map[string]Entry{}
	for _, e := range DefaultCatalog() {
		catalog[e.ID] = e
	}
	for id, kick := range liveKick {
		e, ok := catalog[id]
		switch {
		case !ok:
			t.Errorf("liveKick names %q, which is not a catalog entry", id)
		case e.Kind == KindC64:
			if string(kick) != "\r" && string(kick) != "\x7f" {
				t.Errorf("liveKick[%q] = %q, want a single C64 detection key: RETURN \"\\r\" or DEL \"\\x7f\"", id, kick)
			}
		case len(kick) == 0 || kick[len(kick)-1] != '\r' || bytes.Contains(kick, []byte("\n")):
			t.Errorf("liveKick[%q] = %q, want keystrokes ending with the browser's Enter \"\\r\"", id, kick)
		}
	}
}

func TestLiveClientSendsKickOnceAfterConnected(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	kicked := newLiveClient(Entry{ID: "telehack-ssh", Protocol: ProtocolSSH}, cancel)
	defer kicked.stop()
	_ = kicked.SendControl(Control{Type: controlEcho})
	if len(kicked.events) != 0 {
		t.Fatal("kick sent before connected")
	}
	_ = kicked.SendControl(Control{Type: controlConnected})
	_ = kicked.SendControl(Control{Type: controlConnected})
	if len(kicked.events) != 1 {
		t.Fatalf("queued events = %d, want exactly one kick", len(kicked.events))
	}
	if ev := <-kicked.events; !bytes.Equal(ev.Data, liveKick["telehack-ssh"]) || ev.Resize != nil || ev.HostKeyAccept != nil {
		t.Fatalf("kick event = %+v, want Data %q only", ev, liveKick["telehack-ssh"])
	}

	silent := newLiveClient(Entry{ID: "towel", Protocol: ProtocolTelnet, Kind: KindWorld}, cancel)
	defer silent.stop()
	_ = silent.SendControl(Control{Type: controlConnected})
	if len(silent.events) != 0 {
		t.Fatal("an entry without a kick received input")
	}
	if (liveRow{readable: liveReadableTarget - 1}).passed() || !(liveRow{readable: liveReadableTarget}).passed() {
		t.Fatalf("pass rule is not readable >= %d", liveReadableTarget)
	}
}

func TestLiveC64RuleCountsPETSCIIOnTheWire(t *testing.T) {
	if rule := liveRuleFor(Entry{Protocol: ProtocolTelnet, Kind: KindC64}); rule.window != 45*time.Second || rule.target != 0 {
		t.Fatalf("C64 rule = %+v, want a 45 s window that readable text never ends", rule)
	}
	if rule := liveRuleFor(Entry{Protocol: ProtocolTelnet, Kind: KindBBS}); rule.window != liveWindow || rule.target != liveReadableTarget {
		t.Fatalf("BBS rule = %+v, want the readable-text rule", rule)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	count := &livePETSCIICount{enough: cancel}
	// CLR, CR, space and shifted space are not printable; "PRESS" (5), "DEL" in the upper
	// case range (3) and pi (1) are.
	count.add([]byte{0x93, 0x0D, 'P', 'R', 'E', 'S', 'S', 0x20, 0xC4, 0xC5, 0xCC, 0xA0, 0xFF})
	if printable, busy := count.result(); printable != 9 || busy || ctx.Err() != nil {
		t.Fatalf("after 9 printable characters: printable %d, busy %v, ended %v", printable, busy, ctx.Err() != nil)
	}
	count.add([]byte{':'})
	if printable, _ := count.result(); printable != 10 || ctx.Err() == nil {
		t.Fatalf("printable %d, ended %v; the 10th printable character must end the session", printable, ctx.Err() != nil)
	}

	for name, chunks := range map[string][][]byte{
		"lower/upper set capitals split across reads": {{0xC2, 0xD5}, {0xD3, 0xD9}},
		"upper/graphics set":                          {[]byte("BUSY! Call later")},
		"mixed ranges":                                {{'b', 0xD5, 's', 'Y'}},
	} {
		busy := &livePETSCIICount{}
		for _, chunk := range chunks {
			busy.add(chunk)
		}
		if _, seen := busy.result(); !seen {
			t.Errorf("%s: BUSY banner not recognised", name)
		}
	}

	rows := []struct {
		row  liveRow
		want bool
	}{
		{liveRow{c64: true, petscii: livePETSCIITarget - 1}, false},
		{liveRow{c64: true, petscii: livePETSCIITarget}, true},
		{liveRow{c64: true, busy: true}, true},
		{liveRow{c64: true, readable: 100}, false},
	}
	for _, tc := range rows {
		if got := tc.row.passed(); got != tc.want {
			t.Errorf("%+v passed = %v, want %v", tc.row, got, tc.want)
		}
	}
}

// The count decides like the session: when the first chunk opens Telnet, only the data left
// after negotiation counts, so a board that only negotiates cannot pass; a raw board's later
// IAC bytes are PETSCII data and count.
func TestLivePETSCIICountStripsTelnetNegotiation(t *testing.T) {
	negotiation := []byte{
		tfIAC, tfDO, tfTTYPE, tfIAC, tfDO, tfNAWS, tfIAC, tfWILL, tfECHO, tfIAC, tfWILL, tfSGA,
		tfIAC, tfSB, tfTTYPE, 1, tfIAC, tfSE,
	} // 12 bytes in the printable PETSCII ranges
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	telnet := &livePETSCIICount{enough: cancel}
	telnet.add(negotiation)
	telnet.add([]byte{tfIAC, tfDO, tfNAWS}) // still Telnet for the rest of the call
	if printable, busy := telnet.result(); printable != 0 || busy || ctx.Err() != nil {
		t.Fatalf("negotiation only: printable %d, busy %v, ended %v; want nothing counted", printable, busy, ctx.Err() != nil)
	}
	telnet.add([]byte{'H', 'I', tfIAC, tfIAC, 0x0D, tfIAC, tfWILL, tfECHO, 'B', 'U', 'S', 'Y'})
	if printable, busy := telnet.result(); printable != 7 || !busy {
		t.Fatalf("Telnet data: printable %d, busy %v; want 7 (H, I, pi from IAC IAC, BUSY) and busy", printable, busy)
	}

	raw := &livePETSCIICount{}
	raw.add([]byte{0xFF, 'A'})            // pi first: no Telnet
	raw.add([]byte{tfIAC, tfDO, tfTTYPE}) // later IAC bytes are data: 0xFF and 0xFD count, 0x18 not
	if printable, _ := raw.result(); printable != 4 {
		t.Fatalf("raw board: printable %d, want 4 (pi, A, pi, 0xFD)", printable)
	}
}

// TestLiveCatalog dials every catalog entry through the real engine. It needs Internet
// access and runs only with AURAGO_RETRONET_LIVE=1. Every entry must show at least 40
// readable runes within 10 s of "connected"; C64 boards need at least 10 printable PETSCII
// characters within 45 s, or a BUSY banner. The only input is the entry's liveKick.
//
// funtopia (funtopia.synchro.net:3023) publishes an AAAA record that does not answer on port
// 3023 (checked 2026-10-09); it passes only because DialEntry tries every validated address,
// IPv4 first, and connects to the IPv4 address.
func TestLiveCatalog(t *testing.T) {
	if os.Getenv(liveEnv) != "1" {
		t.Skipf("set %s=1 to dial the live catalog", liveEnv)
	}
	entries := DefaultCatalog()
	m := &Manager{Dialer: Dialer{}, MaxSessions: liveParallel}
	rows := make([]liveRow, len(entries))
	sem := make(chan struct{}, liveParallel)
	t.Run("dial", func(t *testing.T) {
		for i, e := range entries {
			t.Run(e.ID, func(t *testing.T) {
				t.Parallel()
				sem <- struct{}{}
				defer func() { <-sem }()
				rows[i] = liveDial(m, e)
				if !rows[i].passed() {
					t.Errorf("%s (%s): %d readable runes, %d printable PETSCII, busy %v, connected %v, result %s/%s",
						e.ID, e.Address(), rows[i].readable, rows[i].petscii, rows[i].busy, rows[i].connected,
						rows[i].result.Code, rows[i].result.Reason)
				}
			})
		}
	})
	var report strings.Builder
	fmt.Fprintf(&report, "\n%-22s %-36s %8s %8s %9s  %-26s %s\n", "ENTRY", "ADDRESS", "BYTES", "READABLE", "PETSCII", "RESULT", "KICK")
	for i, e := range entries {
		row := rows[i]
		petscii := "-"
		if row.c64 {
			petscii = strconv.Itoa(row.petscii)
			if row.busy {
				petscii += " BUSY"
			}
		}
		kick := ""
		if k, ok := liveKick[e.ID]; ok {
			kick = fmt.Sprintf("%q", k)
		}
		fmt.Fprintf(&report, "%-22s %-36s %8d %8d %9s  %-26s %s\n", e.ID, e.Address(), row.bytes, row.readable, petscii,
			row.result.Code+"/"+row.result.Reason, kick)
	}
	t.Log(report.String())
}
