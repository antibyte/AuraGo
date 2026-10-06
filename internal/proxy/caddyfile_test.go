package proxy

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"aurago/internal/config"
)

func basicAuthConfig(user, password string) *config.Config {
	cfg := &config.Config{}
	cfg.SecurityProxy.HTTPSPort = 443
	cfg.SecurityProxy.BasicAuth.Enabled = true
	cfg.SecurityProxy.BasicAuth.Username = user
	cfg.SecurityProxy.BasicAuth.Password = password
	return cfg
}

var basicAuthAccountLine = regexp.MustCompile(`(?m)^\t\t"([^"]*)" (\S+)$`)

func TestGenerateCaddyfileWritesBasicAuthHashLiterally(t *testing.T) {
	cfg := basicAuthConfig("admin", "correct horse battery staple")

	out, err := GenerateCaddyfile(cfg, "upstream:8088")
	if err != nil {
		t.Fatalf("GenerateCaddyfile() error = %v", err)
	}
	if strings.Contains(out, "{$PROXY_BASIC_AUTH") {
		t.Fatalf("Caddyfile still depends on container environment variables:\n%s", out)
	}
	if strings.Contains(out, "correct horse battery staple") {
		t.Fatal("Caddyfile contains the plaintext password")
	}
	if !strings.Contains(out, "\tbasic_auth * {\n") {
		t.Fatalf("Caddyfile has no basic_auth block:\n%s", out)
	}
	match := basicAuthAccountLine.FindStringSubmatch(out)
	if match == nil {
		t.Fatalf("Caddyfile has no basic_auth account line:\n%s", out)
	}
	if match[1] != "admin" {
		t.Fatalf("basic_auth user = %q, want admin", match[1])
	}
	if err := bcrypt.CompareHashAndPassword([]byte(match[2]), []byte("correct horse battery staple")); err != nil {
		t.Fatalf("basic_auth hash does not verify the Vault password: %v", err)
	}
}

func TestGenerateCaddyfileRejectsBasicAuthWithoutVaultCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, user, password string
	}{
		{"no credentials", "", ""},
		{"no password", "admin", ""},
		{"no user", "", "secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := GenerateCaddyfile(basicAuthConfig(tc.user, tc.password), "upstream:8088")
			if !errors.Is(err, ErrBasicAuthCredentialsMissing) {
				t.Fatalf("GenerateCaddyfile() error = %v, want ErrBasicAuthCredentialsMissing", err)
			}
		})
	}
}

func TestGenerateCaddyfileRejectsUnusableBasicAuthCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, user, password string
	}{
		{"colon in user", "ad:min", "secret"},
		{"quote in user", `ad"min`, "secret"},
		{"backslash in user", `ad\min`, "secret"},
		{"placeholder in user", "{$HOME}", "secret"},
		{"newline in user", "ad\nmin", "secret"},
		{"password beyond bcrypt limit", "admin", strings.Repeat("x", 73)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := GenerateCaddyfile(basicAuthConfig(tc.user, tc.password), "upstream:8088")
			if !errors.Is(err, ErrBasicAuthCredentialsInvalid) {
				t.Fatalf("GenerateCaddyfile() error = %v, want ErrBasicAuthCredentialsInvalid", err)
			}
		})
	}
}

func TestGenerateCaddyfileKeepsBasicAuthOffWithoutCredentials(t *testing.T) {
	cfg := &config.Config{}
	cfg.SecurityProxy.HTTPSPort = 443

	out, err := GenerateCaddyfile(cfg, "upstream:8088")
	if err != nil {
		t.Fatalf("GenerateCaddyfile() error = %v", err)
	}
	if strings.Contains(out, "basic_auth") {
		t.Fatalf("basic_auth emitted while disabled:\n%s", out)
	}
}

func TestGenerateCaddyfileEmitsRateLimitZone(t *testing.T) {
	for _, tc := range []struct {
		rps, burst int
		window     string
	}{
		{10, 50, "5s"},
		{100, 10, "100ms"},
		{3, 10, "3.334s"},
		{10000, 1, "1ms"},
	} {
		cfg := &config.Config{}
		cfg.SecurityProxy.HTTPSPort = 443
		cfg.SecurityProxy.RateLimiting.Enabled = true
		cfg.SecurityProxy.RateLimiting.RequestsPerSecond = tc.rps
		cfg.SecurityProxy.RateLimiting.Burst = tc.burst

		out, err := GenerateCaddyfile(cfg, "upstream:8088")
		if err != nil {
			t.Fatalf("GenerateCaddyfile() error = %v", err)
		}
		want := "\trate_limit {\n" +
			"\t\tzone aurago_client {\n" +
			"\t\t\tkey {http.request.remote.host}\n" +
			"\t\t\tevents " + strconv.Itoa(tc.burst) + "\n" +
			"\t\t\twindow " + tc.window + "\n" +
			"\t\t}\n" +
			"\t}\n"
		if !strings.Contains(out, want) {
			t.Fatalf("rps=%d burst=%d: rate_limit block missing, want\n%s\ngot:\n%s", tc.rps, tc.burst, want, out)
		}
		if strings.Contains(out, "rate_limit {remote.host}") {
			t.Fatalf("legacy inline rate_limit syntax emitted:\n%s", out)
		}
	}
}

func TestGenerateCaddyfileOmitsRateLimitWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.SecurityProxy.HTTPSPort = 443
	cfg.SecurityProxy.RateLimiting.RequestsPerSecond = 10
	cfg.SecurityProxy.RateLimiting.Burst = 50

	out, err := GenerateCaddyfile(cfg, "upstream:8088")
	if err != nil {
		t.Fatalf("GenerateCaddyfile() error = %v", err)
	}
	if strings.Contains(out, "rate_limit") {
		t.Fatalf("rate_limit emitted while disabled:\n%s", out)
	}
}

func TestGenerateCaddyfileAppliesProtectionToAdditionalRoutes(t *testing.T) {
	cfg := basicAuthConfig("admin", "secret")
	cfg.SecurityProxy.Domain = "aurago.example.com"
	cfg.SecurityProxy.RateLimiting.Enabled = true
	cfg.SecurityProxy.RateLimiting.RequestsPerSecond = 10
	cfg.SecurityProxy.RateLimiting.Burst = 50
	cfg.SecurityProxy.AdditionalRoutes = []config.ProxyRoute{{Name: "grafana", Domain: "grafana.example.com", Upstream: "grafana:3000"}}

	out, err := GenerateCaddyfile(cfg, "aurago:8088")
	if err != nil {
		t.Fatalf("GenerateCaddyfile() error = %v", err)
	}
	for _, want := range []string{"aurago.example.com {", "grafana.example.com {", "reverse_proxy aurago:8088 {", "reverse_proxy grafana:3000 {"} {
		if !strings.Contains(out, want) {
			t.Fatalf("Caddyfile lacks %q:\n%s", want, out)
		}
	}
	if got := strings.Count(out, "\tbasic_auth * {"); got != 2 {
		t.Fatalf("basic_auth blocks = %d, want one per site:\n%s", got, out)
	}
	if got := strings.Count(out, "\trate_limit {"); got != 2 {
		t.Fatalf("rate_limit blocks = %d, want one per site:\n%s", got, out)
	}
}
