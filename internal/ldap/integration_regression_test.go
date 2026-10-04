package ldap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"testing"
	"time"

	ber "github.com/go-asn1-ber/asn1-ber"
	ldappkg "github.com/go-ldap/ldap/v3"
)

type auditLDAPConn struct {
	fakeLDAPConn
	order     []string
	tlsErr    error
	tlsConfig *tls.Config
	search    func(*ldappkg.SearchRequest) (*ldappkg.SearchResult, error)
}

func (c *auditLDAPConn) StartTLS(cfg *tls.Config) error {
	c.order = append(c.order, "tls")
	c.tlsConfig = cfg
	return c.tlsErr
}
func (c *auditLDAPConn) Bind(_, _ string) error { c.order = append(c.order, "bind"); return nil }
func (c *auditLDAPConn) Close() error           { c.order = append(c.order, "close"); return nil }
func (c *auditLDAPConn) Search(r *ldappkg.SearchRequest) (*ldappkg.SearchResult, error) {
	return c.search(r)
}

func TestStartTLSPrecedesBindAndNeverFallsBack(t *testing.T) {
	original := dialLDAPURL
	t.Cleanup(func() { dialLDAPURL = original })
	for _, fail := range []bool{false, true} {
		t.Run(strconv.FormatBool(fail), func(t *testing.T) {
			conn := &auditLDAPConn{}
			if fail {
				conn.tlsErr = errors.New("TLS rejected")
			}
			dialLDAPURL = func(addr string, d *net.Dialer, cfg *tls.Config) (ldapConn, error) {
				if addr != "ldap://directory.example:389" || d.Timeout <= 0 {
					t.Fatalf("invalid dial: %s %v", addr, d)
				}
				return conn, nil
			}
			client := NewClient(LDAPConfig{Host: "directory.example", Port: 389, TLSMode: "starttls"})
			err := client.ConnectAndBind()
			want := []string{"tls", "bind"}
			if fail {
				want = []string{"tls", "close"}
			}
			if (err != nil) != fail || !reflect.DeepEqual(conn.order, want) {
				t.Fatalf("err=%v order=%v", err, conn.order)
			}
			if conn.tlsConfig.ServerName != "directory.example" || conn.tlsConfig.InsecureSkipVerify || conn.tlsConfig.MinVersion != tls.VersionTLS12 {
				t.Fatal("TLS identity policy missing")
			}
		})
	}
}

func TestLDAPTransportPreservesLegacyAndRejectsUnknownMode(t *testing.T) {
	original := dialLDAPURL
	defer func() { dialLDAPURL = original }()
	for _, tc := range []struct {
		mode   string
		legacy bool
		scheme string
	}{{"", true, "ldaps"}, {"", false, "ldap"}, {"plain", true, "ldap"}, {"ldaps", false, "ldaps"}, {"bad", false, ""}} {
		conn := &auditLDAPConn{}
		called := false
		dialLDAPURL = func(addr string, _ *net.Dialer, _ *tls.Config) (ldapConn, error) {
			called = true
			if addr != tc.scheme+"://example:636" {
				t.Fatal(addr)
			}
			return conn, nil
		}
		err := NewClient(LDAPConfig{Host: "example", Port: 636, UseTLS: tc.legacy, TLSMode: tc.mode}).ConnectAndBind()
		if tc.scheme == "" {
			if err == nil || called {
				t.Fatal("unknown mode accepted")
			}
		} else if err != nil || !reflect.DeepEqual(conn.order, []string{"bind"}) {
			t.Fatalf("err=%v order=%v", err, conn.order)
		}
	}
}

func TestStartTLSHandshakeHasSocketDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
		request, err := ber.ReadPacket(conn)
		if err != nil {
			return
		}
		response := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "")
		response.AppendChild(request.Children[0])
		result := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ldappkg.ApplicationExtendedResponse, nil, "")
		result.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, 0, ""))
		result.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", ""))
		result.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", ""))
		response.AppendChild(result)
		_, _ = conn.Write(response.Bytes())
		buf := make([]byte, 4096)
		for {
			if _, err := conn.Read(buf); err != nil {
				return
			}
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	client := NewClient(LDAPConfig{Host: "127.0.0.1", Port: port, TLSMode: "starttls", RequestTimeout: 1})
	start := time.Now()
	if err := client.ConnectAndBind(); err == nil {
		t.Fatal("stalled TLS succeeded")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("unbounded handshake: %s", elapsed)
	}
	<-done
}

func TestLDAPSearchReadsEveryPageAndRejectsPartialResults(t *testing.T) {
	for _, failure := range []string{"", "repeat", "later-error"} {
		t.Run(failure, func(t *testing.T) {
			page := 0
			conn := &auditLDAPConn{}
			conn.search = func(r *ldappkg.SearchRequest) (*ldappkg.SearchResult, error) {
				control := ldappkg.FindControl(r.Controls, ldappkg.ControlTypePaging).(*ldappkg.ControlPaging)
				if control.PagingSize != 500 || string(control.Cookie) != []string{"", "one", "two"}[page] {
					t.Fatalf("bad page request %d: %#v", page, control)
				}
				page++
				if failure == "later-error" && page == 2 {
					return nil, errors.New("server failed")
				}
				cookie := ""
				count := 5
				if page < 3 {
					cookie = []string{"one", "two"}[page-1]
					count = 500
				}
				if failure == "repeat" && page == 2 {
					cookie = "one"
				}
				result := &ldappkg.SearchResult{}
				for i := 0; i < count; i++ {
					result.Entries = append(result.Entries, &ldappkg.Entry{DN: fmt.Sprintf("cn=%d-%d", page, i)})
				}
				next := ldappkg.NewControlPaging(500)
				next.SetCookie([]byte(cookie))
				result.Controls = []ldappkg.Control{next}
				return result, nil
			}
			client := &Client{conn: conn}
			result, err := client.ListUsers()
			if failure != "" {
				if err == nil || result != nil {
					t.Fatalf("partial result accepted: %v", err)
				}
				return
			}
			if err != nil || len(result.Entries) != 1005 || page != 3 {
				t.Fatalf("result=%v err=%v pages=%d", result, err, page)
			}
		})
	}
}
