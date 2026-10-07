package fritzbox

import (
	"fmt"
	"net/url"

	"aurago/internal/security"
)

// routerListURL keeps router-generated exports on administrator-configured
// origins. FRITZ!OS may advertise its LAN address and HTTP port even when the
// caller uses a hostname, forwarded port or HTTPS. For the known list endpoint,
// retain only its path and query and use the configured TR-064 origin. Never
// broaden boundTransport or redirect permissions to the advertised authority.
func (c *Client) routerListURL(rawURL, exportPath, relativeBase string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || rawURL == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return "", fmt.Errorf("fritzbox: invalid list URL")
	}
	base, err := url.Parse(relativeBase)
	if err != nil {
		return "", fmt.Errorf("fritzbox: invalid configured list origin")
	}
	if !u.IsAbs() && u.Host == "" {
		u = base.ResolveReference(u)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("fritzbox: invalid list URL scheme or host")
	}
	for _, configured := range []string{c.tr.baseURL, c.webURL} {
		origin, parseErr := url.Parse(configured)
		if parseErr == nil && security.SameHTTPOrigin(u, origin) {
			return u.String(), nil
		}
	}
	// An untrusted authority is never contacted or resolved. Only the exact
	// export endpoint returned by this SOAP action can be rebound locally.
	if u.EscapedPath() != exportPath {
		return "", fmt.Errorf("fritzbox: list URL outside configured router origins")
	}
	target, err := url.Parse(c.tr.baseURL)
	if err != nil {
		return "", fmt.Errorf("fritzbox: invalid configured TR-064 origin")
	}
	target.Path = exportPath
	target.RawQuery = u.RawQuery
	return target.String(), nil
}
