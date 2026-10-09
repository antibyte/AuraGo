package retronet

import (
	"bytes"
	"net"
	"strconv"
	"sync"
	"testing"
)

// Telnet bytes the fixtures speak (RFC 854/855).
const (
	tfIAC   byte = 255
	tfDONT  byte = 254
	tfDO    byte = 253
	tfWONT  byte = 252
	tfWILL  byte = 251
	tfSB    byte = 250
	tfSE    byte = 240
	tfECHO  byte = 1
	tfSGA   byte = 3
	tfTTYPE byte = 24
	tfNAWS  byte = 31
)

// tfNAWSFor is the NAWS subnegotiation a client sends for cols x rows (both < 255).
func tfNAWSFor(cols, rows int) []byte {
	return []byte{tfIAC, tfSB, tfNAWS, 0, byte(cols), 0, byte(rows), tfIAC, tfSE}
}

// tfTTypeIs is the TTYPE IS answer for term.
func tfTTypeIs(term string) []byte {
	out := append([]byte{tfIAC, tfSB, tfTTYPE, 0}, term...)
	return append(out, tfIAC, tfSE)
}

// telnetFixtureScript is what the fake service does with every accepted connection.
type telnetFixtureScript struct {
	greeting []byte // written right after accept
	hangUp   bool   // half-close (FIN) after the greeting, then drain until the client closes
}

// telnetFixture is a fake Telnet service on 127.0.0.1 that records every byte it receives.
type telnetFixture struct {
	listener net.Listener
	script   telnetFixtureScript
	wg       sync.WaitGroup

	mu       sync.Mutex
	conns    []net.Conn
	received []byte
}

func startTelnetFixture(t *testing.T, script telnetFixtureScript) *telnetFixture {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &telnetFixture{listener: listener, script: script}
	f.wg.Add(1)
	go f.acceptLoop()
	t.Cleanup(f.close)
	return f
}

func (f *telnetFixture) acceptLoop() {
	defer f.wg.Done()
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.conns = append(f.conns, conn)
		f.mu.Unlock()
		f.wg.Add(1)
		go func() {
			defer f.wg.Done()
			f.serve(conn)
		}()
	}
}

func (f *telnetFixture) serve(conn net.Conn) {
	if len(f.script.greeting) > 0 {
		_, _ = conn.Write(f.script.greeting)
	}
	if f.script.hangUp {
		_ = conn.(*net.TCPConn).CloseWrite()
	}
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			f.mu.Lock()
			f.received = append(f.received, buf[:n]...)
			f.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (f *telnetFixture) close() {
	_ = f.listener.Close()
	f.mu.Lock()
	for _, conn := range f.conns {
		_ = conn.Close()
	}
	f.mu.Unlock()
	f.wg.Wait()
}

// write sends p on the most recent connection; errors are ignored (the session may be gone).
func (f *telnetFixture) write(p []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.conns) > 0 {
		_, _ = f.conns[len(f.conns)-1].Write(p)
	}
}

func (f *telnetFixture) receivedBytes() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return bytes.Clone(f.received)
}

// waitReceived waits until the service received needle.
func (f *telnetFixture) waitReceived(t *testing.T, needle []byte) {
	t.Helper()
	sessWaitFor(t, func() bool { return bytes.Contains(f.receivedBytes(), needle) }, func() string {
		return "service input " + strconv.Quote(string(needle)) + "; received " + strconv.Quote(string(f.receivedBytes()))
	})
}

func (f *telnetFixture) target() string { return f.listener.Addr().String() }

// entry returns a Telnet entry that dials the fixture.
func (f *telnetFixture) entry(kind Kind, charset Charset) Entry {
	addr := f.listener.Addr().(*net.TCPAddr)
	return Entry{ID: "own-telnetfixture", Name: "Telnet Fixture", Category: CategoryOwn, Protocol: ProtocolTelnet, Host: addr.IP.String(), Port: addr.Port, Kind: kind, Charset: charset, Own: true}
}
