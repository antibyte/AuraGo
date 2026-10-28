package security

import (
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestCastTicketHasExactPathAndExpiry(t *testing.T) {
	signed := SignCastMediaURL("http://127.0.0.1:8081/tts/one.wav", CastMediaTicketLifetime)
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
		if SignCastMediaURL(path, 0) != "" {
			t.Fatalf("invalid path signed: %q", path)
		}
	}
	for _, ip := range []string{"::", "::1", "::ffff:127.0.0.1", "64:ff9b::7f00:1", "2002:7f00:1::", "2001:db8::1"} {
		if err := ValidatePublicHTTPURL("http://[" + ip + "]/file"); err == nil {
			t.Fatalf("nonpublic address accepted: %s", ip)
		}
	}
}

func TestCastTicketLifetimeCapsAtThreeHours(t *testing.T) {
	short := SignCastMediaURL("http://127.0.0.1:8081/tts/short.wav", CastTTSTicketLifetime)
	shortURL, err := url.Parse(short)
	if err != nil || !ValidCastMediaTicket(shortURL, time.Now()) {
		t.Fatalf("10-minute ticket rejected: %v", err)
	}
	if ValidCastMediaTicket(shortURL, time.Now().Add(11*time.Minute)) {
		t.Fatal("10-minute ticket accepted after 11 minutes")
	}
	capped := SignCastMediaURL("http://127.0.0.1:8081/tts/long.wav", 4*time.Hour)
	cappedURL, err := url.Parse(capped)
	if err != nil {
		t.Fatal(err)
	}
	unix, err := strconv.ParseInt(cappedURL.Query().Get("cast_expires"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if unix > now.Add(CastMediaTicketLifetime+time.Minute).Unix() || unix < now.Add(CastMediaTicketLifetime-time.Minute).Unix() {
		t.Fatalf("lifetime above 3h was not capped: expires %d", unix)
	}
}
