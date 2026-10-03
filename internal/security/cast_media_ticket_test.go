package security

import (
	"net/url"
	"testing"
	"time"
)

func TestCastTicketHasExactPathAndExpiry(t *testing.T) {
	signed := SignCastMediaURL("http://127.0.0.1:8081/tts/one.wav")
	u, err := url.Parse(signed)
	if err != nil || !ValidCastMediaTicket(u, time.Now()) {
		t.Fatalf("valid ticket rejected: %v", err)
	}
	if ValidCastMediaTicket(u, time.Now().Add(CastMediaTicketLifetime+time.Second)) {
		t.Fatal("expired ticket accepted")
	}
	u.Path = "/tts/two.wav"
	if ValidCastMediaTicket(u, time.Now()) {
		t.Fatal("ticket grants another file")
	}
	for _, path := range []string{"/tts/", "/tts/../secret", "/cast-media/dir/file", "/files/private.wav"} {
		if SignCastMediaURL(path) != "" {
			t.Fatalf("invalid path signed: %q", path)
		}
	}
	for _, ip := range []string{"::", "::1", "::ffff:127.0.0.1", "64:ff9b::7f00:1", "2002:7f00:1::", "2001:db8::1"} {
		if err := ValidatePublicHTTPURL("http://[" + ip + "]/file"); err == nil {
			t.Fatalf("nonpublic address accepted: %s", ip)
		}
	}
}
