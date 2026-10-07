package ldap

import (
	"crypto/tls"
	"net"
	"net/url"
	"time"

	ldappkg "github.com/go-ldap/ldap/v3"
)

// Retain the socket to bound the TLS handshake as well as LDAP requests.
type boundedLDAPConn struct {
	*ldappkg.Conn
	socket  net.Conn
	timeout time.Duration
}

func dialBoundedLDAP(addr string, dialer *net.Dialer, tlsConfig *tls.Config) (ldapConn, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	var socket net.Conn
	if u.Scheme == "ldaps" {
		socket, err = tls.DialWithDialer(dialer, "tcp", u.Host, tlsConfig)
	} else {
		socket, err = dialer.Dial("tcp", u.Host)
	}
	if err != nil {
		return nil, err
	}
	conn := ldappkg.NewConn(socket, u.Scheme == "ldaps")
	conn.Start()
	return &boundedLDAPConn{Conn: conn, socket: socket, timeout: 30 * time.Second}, nil
}

func (c *boundedLDAPConn) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
	c.Conn.SetTimeout(timeout)
}

func (c *boundedLDAPConn) StartTLS(cfg *tls.Config) error {
	if err := c.socket.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return err
	}
	defer c.socket.SetDeadline(time.Time{})
	return c.Conn.StartTLS(cfg)
}
