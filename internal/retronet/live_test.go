package retronet

import (
	"bytes"
	"context"
	"fmt"
	"os"
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
)

// liveKick holds the keystrokes sent once, right after "connected", to entries that print only
// a prompt until the user types. The bytes are exactly what the browser sends (Enter is "\r").
// Use harmless, read-only commands. Verified 2026-10-09:
//   - telehack-ssh shows only its "." prompt; "date" answers with about 39 readable runes
//     (too few, and the length varies with the date), "help" lists the commands (> 130 runes).
var liveKick = map[string][]byte{
	"telehack-ssh": []byte("help\r"),
}

// liveClient is a browser that types nothing except an optional kick. Its output window starts
// with "connected"; it stops the session once enough readable text arrived or the window ends.
type liveClient struct {
	events chan ClientEvent // buffered for the single kick
	kick   []byte
	cancel context.CancelFunc

	mu        sync.Mutex
	connected bool
	window    *time.Timer
	bytes     int
	readable  int
	text      []byte
}

func newLiveClient(entryID string, cancel context.CancelFunc) *liveClient {
	return &liveClient{events: make(chan ClientEvent, 1), kick: liveKick[entryID], cancel: cancel}
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
	if c.readable >= liveReadableTarget {
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
	c.window = time.AfterFunc(liveWindow, c.cancel)
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

// liveRow is one line of the live report.
type liveRow struct {
	connected bool
	bytes     int
	readable  int
	result    Result
}

// passed is the same rule for every entry: enough readable output within the window.
func (r liveRow) passed() bool { return r.readable >= liveReadableTarget }

// liveDial runs one catalog entry; the only input is its liveKick, if any.
func liveDial(m *Manager, e Entry) liveRow {
	ctx, cancel := context.WithTimeout(context.Background(), liveBackstop)
	defer cancel()
	client := newLiveClient(e.ID, cancel)
	res := m.Run(ctx, e, Size{Cols: 80, Rows: 25}, client)
	client.stop()
	client.mu.Lock()
	defer client.mu.Unlock()
	return liveRow{connected: client.connected, bytes: client.bytes, readable: client.readable, result: res}
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
	ids := map[string]bool{}
	for _, e := range DefaultCatalog() {
		ids[e.ID] = true
	}
	for id, kick := range liveKick {
		if !ids[id] {
			t.Errorf("liveKick names %q, which is not a catalog entry", id)
		}
		if len(kick) == 0 || kick[len(kick)-1] != '\r' || bytes.Contains(kick, []byte("\n")) {
			t.Errorf("liveKick[%q] = %q, want keystrokes ending with the browser's Enter \"\\r\"", id, kick)
		}
	}
}

func TestLiveClientSendsKickOnceAfterConnected(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	kicked := newLiveClient("telehack-ssh", cancel)
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

	silent := newLiveClient("towel", cancel)
	defer silent.stop()
	_ = silent.SendControl(Control{Type: controlConnected})
	if len(silent.events) != 0 {
		t.Fatal("an entry without a kick received input")
	}
	if (liveRow{readable: liveReadableTarget - 1}).passed() || !(liveRow{readable: liveReadableTarget}).passed() {
		t.Fatalf("pass rule is not readable >= %d", liveReadableTarget)
	}
}

// TestLiveCatalog dials every catalog entry through the real engine. It needs Internet
// access and runs only with AURAGO_RETRONET_LIVE=1. Every entry must show at least 40
// readable runes within 10 s of "connected"; the only input is the entry's liveKick.
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
					t.Errorf("%s (%s): %d readable runes, connected %v, result %s/%s",
						e.ID, e.Address(), rows[i].readable, rows[i].connected, rows[i].result.Code, rows[i].result.Reason)
				}
			})
		}
	})
	var report strings.Builder
	fmt.Fprintf(&report, "\n%-22s %-36s %8s %8s  %-26s %s\n", "ENTRY", "ADDRESS", "BYTES", "READABLE", "RESULT", "KICK")
	for i, e := range entries {
		row := rows[i]
		kick := ""
		if k, ok := liveKick[e.ID]; ok {
			kick = fmt.Sprintf("%q", k)
		}
		fmt.Fprintf(&report, "%-22s %-36s %8d %8d  %-26s %s\n", e.ID, e.Address(), row.bytes, row.readable,
			row.result.Code+"/"+row.result.Reason, kick)
	}
	t.Log(report.String())
}
