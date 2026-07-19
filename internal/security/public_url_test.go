package security

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestValidatePublicHTTPURLAllowsPublicLiteralAddresses(t *testing.T) {
	for _, rawURL := range []string{
		"https://8.8.8.8/image.png?signature=keep-me",
		"http://[2606:4700:4700::1111]/image.jpg",
	} {
		if err := ValidatePublicHTTPURL(rawURL); err != nil {
			t.Fatalf("ValidatePublicHTTPURL(%q) error = %v", rawURL, err)
		}
	}
}

func TestValidatePublicHTTPURLRejectsNonPublicInputs(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	tests := []string{
		"data:image/png;base64,AA==",
		"file:///tmp/image.png",
		"/tmp/image.png",
		"https://user:pass@8.8.8.8/image.png",
		"http://127.0.0.1/image.png",
		"http://169.254.169.254/latest/meta-data",
		"http://10.0.0.1/image.png",
		"http://192.168.1.1/image.png",
		"http://[::1]/image.png",
		"https://192.0.2.10/image.png",
	}
	for _, rawURL := range tests {
		if err := ValidatePublicHTTPURL(rawURL); err == nil {
			t.Fatalf("ValidatePublicHTTPURL(%q) unexpectedly succeeded", rawURL)
		}
	}
}

func TestValidatePublicHTTPURLValidatesEveryResolvedAddress(t *testing.T) {
	originalLookup := publicURLLookupIP
	defer func() { publicURLLookupIP = originalLookup }()

	publicURLLookupIP = func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "images.example.test" {
			t.Fatalf("host = %q", host)
		}
		return []net.IPAddr{
			{IP: net.ParseIP("8.8.8.8")},
			{IP: net.ParseIP("10.0.0.7")},
		}, nil
	}

	err := ValidatePublicHTTPURL("https://images.example.test/photo.png?token=secret")
	if err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("error = %v, want non-public resolution error", err)
	}
	if strings.Contains(err.Error(), "token=secret") {
		t.Fatalf("error leaked signed URL query: %v", err)
	}
}
