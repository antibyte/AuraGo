package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var castTicketKey struct {
	once  sync.Once
	key   [32]byte
	valid bool
}

const CastMediaTicketLifetime = 3 * time.Hour

// SignCastMediaURL grants GET/HEAD access to one file for this process lifetime.
// Restarting AuraGo invalidates outstanding tickets. No persistent secret is exposed.
func SignCastMediaURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || !castTicketPath(u.Path) {
		return ""
	}
	castTicketKey.once.Do(func() { _, err := rand.Read(castTicketKey.key[:]); castTicketKey.valid = err == nil })
	if !castTicketKey.valid {
		return ""
	}
	q := u.Query()
	expires := strconv.FormatInt(time.Now().Add(CastMediaTicketLifetime).Unix(), 10)
	q.Set("cast_expires", expires)
	q.Set("cast_ticket", castTicketSignature(u.EscapedPath(), expires))
	u.RawQuery = q.Encode()
	return u.String()
}

func ValidCastMediaTicket(u *url.URL, now time.Time) bool {
	if u == nil || !castTicketPath(u.Path) {
		return false
	}
	castTicketKey.once.Do(func() { _, err := rand.Read(castTicketKey.key[:]); castTicketKey.valid = err == nil })
	q := u.Query()
	if !castTicketKey.valid || len(q["cast_expires"]) != 1 || len(q["cast_ticket"]) != 1 {
		return false
	}
	expires, err := strconv.ParseInt(q.Get("cast_expires"), 10, 64)
	if err != nil || expires <= now.Unix() || expires > now.Add(CastMediaTicketLifetime).Unix() {
		return false
	}
	want := castTicketSignature(u.EscapedPath(), q.Get("cast_expires"))
	return hmac.Equal([]byte(want), []byte(q.Get("cast_ticket")))
}

func castTicketSignature(path, expires string) string {
	mac := hmac.New(sha256.New, castTicketKey.key[:])
	mac.Write([]byte(path + "\x00" + expires))
	return hex.EncodeToString(mac.Sum(nil))
}

func castTicketPath(path string) bool {
	for _, prefix := range []string{"/tts/", "/cast-media/"} {
		if strings.HasPrefix(path, prefix) {
			name := strings.TrimPrefix(path, prefix)
			return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\:\x00")
		}
	}
	return false
}
