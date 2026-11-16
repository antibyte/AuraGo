package push

import (
	"crypto/ecdh"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"aurago/internal/security"
)

var ErrInvalidSubscription = errors.New("invalid push subscription")

// ValidateSubscription checks browser-controlled endpoints and encryption keys
// before a replacement can overwrite an existing working subscription.
func ValidateSubscription(sub PushSubscription) error {
	u, err := url.Parse(sub.Endpoint)
	if err != nil || len(sub.Endpoint) > 4096 || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" || strings.TrimSpace(sub.Endpoint) != sub.Endpoint {
		return fmt.Errorf("push endpoint must be a public HTTPS URL without credentials or fragment")
	}
	decode := func(value string) ([]byte, error) {
		if len(value) > 128 || strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("invalid push key")
		}
		return base64.RawURLEncoding.Strict().DecodeString(strings.TrimRight(value, "="))
	}
	auth, err := decode(sub.Keys["auth"])
	if err != nil || len(auth) != 16 {
		return fmt.Errorf("push auth key must contain 16 bytes")
	}
	publicKey, err := decode(sub.Keys["p256dh"])
	if err != nil {
		return fmt.Errorf("invalid push public key")
	}
	if _, err := ecdh.P256().NewPublicKey(publicKey); err != nil {
		return fmt.Errorf("invalid push P-256 public key")
	}
	if err := security.ValidatePublicHTTPURL(sub.Endpoint); err != nil {
		return fmt.Errorf("push endpoint must resolve only to public addresses")
	}
	return nil
}

type observedPushClient struct {
	client     *http.Client
	beforeSend []func()
}

func (c observedPushClient) Do(req *http.Request) (*http.Response, error) {
	// The SDK has completed local encryption and VAPID preparation at this point.
	for _, observe := range c.beforeSend {
		if observe != nil {
			observe()
		}
	}
	return c.client.Do(req)
}
