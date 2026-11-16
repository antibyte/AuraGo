package security

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
)

type configuredHTTPPinKey struct{}
type configuredHTTPPin struct {
	host string
	ip   net.IP
}

type configuredHTTPTransport struct {
	origin    *url.URL
	err       error
	transport *http.Transport
	lookup    func(context.Context, string) ([]net.IPAddr, error)
}

// NewConfiguredHTTPTransport binds requests to an administrator-selected origin.
// Resolve once per request, dial that address, and bypass proxies for LAN targets.
// Unlike public-download policy, explicitly configured private services are allowed.
func NewConfiguredHTTPTransport(rawURL string, source *http.Transport) http.RoundTripper {
	origin, _ := url.Parse(rawURL)
	t := &configuredHTTPTransport{origin: origin, err: ValidateHTTPBaseURL(rawURL), transport: source.Clone(), lookup: net.DefaultResolver.LookupIPAddr}
	proxy := t.transport.Proxy
	t.transport.Proxy = func(req *http.Request) (*url.URL, error) {
		pin, ok := req.Context().Value(configuredHTTPPinKey{}).(configuredHTTPPin)
		if !ok {
			return nil, fmt.Errorf("integration target has no DNS pin")
		}
		if isPrivateIP(pin.ip) || proxy == nil {
			return nil, nil
		}
		return proxy(req)
	}
	dial := t.transport.DialContext
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	t.transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		pin, ok := ctx.Value(configuredHTTPPinKey{}).(configuredHTTPPin)
		if !ok {
			return nil, fmt.Errorf("integration target has no DNS pin")
		}
		if host == pin.host {
			address = net.JoinHostPort(pin.ip.String(), port)
		}
		return dial(ctx, network, address)
	}
	// Keep the original hostname for certificate verification and TLS SNI.
	t.transport.DialTLSContext = nil
	t.transport.DialTLS = nil
	return t
}

func (t *configuredHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.err != nil {
		return nil, t.err
	}
	if !SameHTTPOrigin(t.origin, req.URL) {
		return nil, fmt.Errorf("integration request left its configured origin")
	}
	host := req.URL.Hostname()
	ip := net.ParseIP(host)
	if ip == nil {
		addresses, err := t.lookup(req.Context(), host)
		if err != nil || len(addresses) == 0 {
			return nil, fmt.Errorf("integration hostname resolution failed")
		}
		ip = addresses[0].IP
		// Mixed public/private answers must take the direct route as well.
		for _, address := range addresses {
			if isPrivateIP(address.IP) {
				ip = address.IP
				break
			}
		}
	}
	ctx := context.WithValue(req.Context(), configuredHTTPPinKey{}, configuredHTTPPin{host: host, ip: ip})
	return t.transport.RoundTrip(req.Clone(ctx))
}

func (t *configuredHTTPTransport) CloseIdleConnections() { t.transport.CloseIdleConnections() }
