package server

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/cydcreator"
)

// cydTLSProbeTimeout bounds the TLS handshake that reads the certificate a
// configured public URL presents.
const cydTLSProbeTimeout = 5 * time.Second

// cydDeviceURLHint names the setting that fixes a device URL or pin problem.
const cydDeviceURLHint = "cyd_creator.ota.public_url"

// cydUnspecifiedHosts are bind addresses a device cannot connect to.
var cydUnspecifiedHosts = map[string]bool{"": true, "0.0.0.0": true, "::": true, "localhost": true, "127.0.0.1": true, "::1": true}

// cydCheckDeviceURL applies the config blob's device URL rule
// (cydcreator.ValidateDeviceURL) before the pin is known: an https URL is
// checked with a well-formed placeholder pin, so only URL problems
// (CYD_BAD_REQUEST, details.reason "device_url") can come back.
func cydCheckDeviceURL(rawURL string) error {
	pin := ""
	if strings.HasPrefix(rawURL, "https://") {
		pin = strings.Repeat("0", sha256.Size*2)
	}
	return cydcreator.ValidateDeviceURL(rawURL, pin)
}

// cydCreatorDeviceBaseURL returns the base URL CYD Creator devices use to
// reach AuraGo: cyd_creator.ota.public_url when set (checked by the device URL
// rule, without a trailing slash), else derived from the server configuration:
// the HTTPS domain or the configured host (a LAN address for wildcard or
// loopback binds) with the HTTPS port, or http:// with server.port when HTTPS
// is off. A derived URL is not checked here (cydCreatorDeviceEndpoint does).
// A device behind NAT or a reverse proxy needs public_url, and so does a
// wildcard bind inside Docker, which derives the container's address.
func cydCreatorDeviceBaseURL(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", errors.New("no configuration")
	}
	if raw := strings.TrimRight(strings.TrimSpace(cfg.CYDCreator.OTA.PublicURL), "/"); raw != "" {
		if err := cydCheckDeviceURL(raw); err != nil {
			return "", err
		}
		return raw, nil
	}
	host := strings.TrimSpace(cfg.Server.Host)
	scheme, port, defaultPort := "http", cfg.Server.Port, 80
	if port <= 0 {
		port = 8088
	}
	if cfg.Server.HTTPS.Enabled {
		scheme, port, defaultPort = "https", cfg.Server.HTTPS.HTTPSPort, 443
		if port <= 0 {
			port = 443
		}
		if domain := strings.TrimSpace(cfg.Server.HTTPS.Domain); domain != "" {
			host = domain
		}
	}
	if cydUnspecifiedHosts[host] {
		host = firstLANIPv4()
	}
	if port != defaultPort {
		host = net.JoinHostPort(host, strconv.Itoa(port))
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return scheme + "://" + host, nil
}

// cydLeafCertPin returns the lowercase SHA-256 hex of the DER certificate in
// the first CERTIFICATE PEM block (other blocks, such as the private key in an
// autocert cache file, are skipped).
func cydLeafCertPin(data []byte) (string, error) {
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return "", errors.New("no CERTIFICATE PEM block found")
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		leaf, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("parse certificate: %w", err)
		}
		sum := sha256.Sum256(leaf.Raw)
		return hex.EncodeToString(sum[:]), nil
	}
}

// cydLocalCertFile is the certificate file AuraGo's own HTTPS listener
// serves: the self-signed certificate, the custom cert_file, or the autocert
// (Let's Encrypt) cache entry named after the domain.
func cydLocalCertFile(cfg *config.Config) string {
	dataDir := cfg.Directories.DataDir
	switch {
	case config.UsesSelfSignedTLS(cfg):
		return config.SelfSignedCertFile(dataDir)
	case config.NormalizeCertMode(cfg.Server.HTTPS.CertMode) == "custom":
		return cfg.Server.HTTPS.CertFile
	default:
		return filepath.Join(dataDir, "certs", strings.TrimSpace(cfg.Server.HTTPS.Domain))
	}
}

// cydTLSProbe reads the leaf certificate a TLS server presents. It verifies
// nothing on purpose: the administrator chose the URL, and the device pins
// exactly this certificate afterwards.
var cydTLSProbe = func(ctx context.Context, hostport, serverName string) ([]byte, error) {
	dialer := &tls.Dialer{Config: &tls.Config{ServerName: serverName, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}} //nolint:gosec // reading the leaf certificate to pin it
	conn, err := dialer.DialContext(ctx, "tcp", hostport)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	state := conn.(*tls.Conn).ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil, errors.New("the server presented no certificate")
	}
	return state.PeerCertificates[0].Raw, nil
}

func cydProbePin(ctx context.Context, baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	ctx, cancel := context.WithTimeout(ctx, cydTLSProbeTimeout)
	defer cancel()
	der, err := cydTLSProbe(ctx, net.JoinHostPort(u.Hostname(), port), u.Hostname())
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:]), nil
}

// cydCreatorDeviceEndpoint resolves the base URL and, for https, the leaf
// certificate pin that MintDevice writes into the config blob. The pin is the
// SHA-256 of the DER leaf certificate, exactly what the firmware compares
// (WiFiClientSecure::verify in cyd_link.cpp). A configured public URL is probed
// (a reverse proxy may present its own certificate); a derived URL pins
// AuraGo's own certificate file and falls back to a probe while Let's Encrypt
// has not issued one yet. Plain http:// gets no pin. Known v1 limitation: a
// renewed leaf certificate (Let's Encrypt about every 60 days) no longer
// matches, and pinned devices need a USB re-flash.
//
// Both URL forms pass the device URL rule (cydcreator.ValidateDeviceURL); a
// violation is CYD_BAD_REQUEST with details.reason "device_url", an unreadable
// pin CYD_BAD_REQUEST with details.reason "tls_pin", both with details.hint
// "cyd_creator.ota.public_url", so cydError passes them to the client.
func cydCreatorDeviceEndpoint(ctx context.Context, cfg *config.Config) (cydcreator.DeviceEndpoint, error) {
	base, err := cydCreatorDeviceBaseURL(cfg)
	if err == nil {
		err = cydCheckDeviceURL(base) // a derived URL, e.g. from an IPv6 bind
	}
	if err != nil {
		if cydcreator.ErrorCode(err) == cydcreator.CodeBadRequest {
			return cydcreator.DeviceEndpoint{}, err
		}
		return cydcreator.DeviceEndpoint{}, cydcreator.NewError(cydcreator.CodeBadRequest,
			"the device URL is not valid; check cyd_creator.ota.public_url",
			map[string]any{"reason": "device_url", "hint": cydDeviceURLHint})
	}
	if strings.HasPrefix(base, "http://") {
		return cydcreator.DeviceEndpoint{URL: base}, nil
	}
	var pin string
	if strings.TrimSpace(cfg.CYDCreator.OTA.PublicURL) == "" && cfg.Server.HTTPS.Enabled {
		if data, readErr := os.ReadFile(cydLocalCertFile(cfg)); readErr == nil {
			pin, err = cydLeafCertPin(data)
		} else {
			err = readErr
		}
	}
	if pin == "" {
		pin, err = cydProbePin(ctx, base)
	}
	if err != nil || pin == "" {
		return cydcreator.DeviceEndpoint{}, cydcreator.NewError(cydcreator.CodeBadRequest,
			"the TLS certificate that devices must pin could not be read from "+base+"; check cyd_creator.ota.public_url",
			map[string]any{"reason": "tls_pin", "hint": cydDeviceURLHint, "url": base})
	}
	return cydcreator.DeviceEndpoint{URL: base, CertSHA256: pin}, nil
}
