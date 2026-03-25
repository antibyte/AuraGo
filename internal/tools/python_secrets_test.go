package tools

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// mockVault is a simple in-memory implementation of config.SecretReader for testing.
type mockVault struct {
	secrets map[string]string
}

func (m *mockVault) ReadSecret(key string) (string, error) {
	v, ok := m.secrets[key]
	if !ok {
		return "", fmt.Errorf("secret %q not found", key)
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// IsPythonAccessibleSecret
// ---------------------------------------------------------------------------

func TestIsPythonAccessibleSecret_AllowsUserKeys(t *testing.T) {
	allowed := []string{
		"my_api_key",
		"user_secret",
		"custom_token",
		"database_url",
		"MY_UPPERCASE_KEY",
	}
	for _, k := range allowed {
		if !IsPythonAccessibleSecret(k) {
			t.Errorf("expected %q to be accessible, but it was blocked", k)
		}
	}
}

func TestIsPythonAccessibleSecret_BlocksExactKeys(t *testing.T) {
	for key := range blockedSecretExact {
		if IsPythonAccessibleSecret(key) {
			t.Errorf("expected exact key %q to be blocked, but it was allowed", key)
		}
	}
}

func TestIsPythonAccessibleSecret_BlocksExactKeysCaseInsensitive(t *testing.T) {
	cases := []string{
		"TELEGRAM_BOT_TOKEN",
		"Discord_Bot_Token",
		"GitHub_Token",
	}
	for _, k := range cases {
		if IsPythonAccessibleSecret(k) {
			t.Errorf("expected %q to be blocked (case-insensitive), but it was allowed", k)
		}
	}
}

func TestIsPythonAccessibleSecret_BlocksPrefixedKeys(t *testing.T) {
	prefixed := []string{
		"email_smtp_pass",
		"google_workspace_client_secret",
		"provider_openrouter_key",
		"s3_bucket_key",
		"telnyx_api_key",
		"fritzbox_password",
		"mqtt_broker_token",
		"ollama_managed_password",
		"sqlconn_mydb",
		"cloudflare_tunnel_token",
		"auth_jwt_secret",
		"credential_password_admin",
		"credential_certificate_tls",
		"vapid_private_key",
		"homepage_api",
		"nest_refresh_token",
	}
	for _, k := range prefixed {
		if IsPythonAccessibleSecret(k) {
			t.Errorf("expected prefix-blocked key %q to be blocked, but it was allowed", k)
		}
	}
}

func TestIsPythonAccessibleSecret_EmptyKey(t *testing.T) {
	// Empty key is technically user-provided; function allows it.
	if !IsPythonAccessibleSecret("") {
		t.Error("expected empty key to be accessible (not blocked)")
	}
}

// ---------------------------------------------------------------------------
// sanitizeEnvKey
// ---------------------------------------------------------------------------

func TestSanitizeEnvKey_Basic(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my_key", "MY_KEY"},
		{"MyApiKey", "MYAPIKEY"},
		{"key-with-dashes", "KEY_WITH_DASHES"},
		{"key.with.dots", "KEY_WITH_DOTS"},
		{"key with spaces", "KEY_WITH_SPACES"},
		{"ALREADY_UPPER", "ALREADY_UPPER"},
		{"123numeric", "123NUMERIC"},
		{"a/b\\c", "A_B_C"},
	}
	for _, tc := range tests {
		got := sanitizeEnvKey(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeEnvKey(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSanitizeEnvKey_EmptyString(t *testing.T) {
	if got := sanitizeEnvKey(""); got != "" {
		t.Errorf("sanitizeEnvKey(\"\") = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// ResolveVaultSecrets
// ---------------------------------------------------------------------------

func TestResolveVaultSecrets_AllAllowed(t *testing.T) {
	vault := &mockVault{secrets: map[string]string{
		"my_key":   "val1",
		"my_other": "val2",
	}}
	resolved, rejected, err := ResolveVaultSecrets(vault, []string{"my_key", "my_other"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rejected) != 0 {
		t.Errorf("expected no rejected, got %v", rejected)
	}
	if resolved["my_key"] != "val1" || resolved["my_other"] != "val2" {
		t.Errorf("unexpected resolved: %v", resolved)
	}
}

func TestResolveVaultSecrets_BlockedKeysRejected(t *testing.T) {
	vault := &mockVault{secrets: map[string]string{
		"telegram_bot_token": "secret_tg",
		"my_key":             "allowed_val",
	}}
	resolved, rejected, err := ResolveVaultSecrets(vault, []string{"telegram_bot_token", "my_key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rejected) != 1 || rejected[0] != "telegram_bot_token" {
		t.Errorf("expected telegram_bot_token rejected, got %v", rejected)
	}
	if _, ok := resolved["telegram_bot_token"]; ok {
		t.Error("blocked key should not be in resolved map")
	}
	if resolved["my_key"] != "allowed_val" {
		t.Errorf("expected my_key=allowed_val, got %v", resolved)
	}
}

func TestResolveVaultSecrets_MissingKeysSkipped(t *testing.T) {
	vault := &mockVault{secrets: map[string]string{}}
	resolved, rejected, err := ResolveVaultSecrets(vault, []string{"nonexistent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rejected) != 0 {
		t.Errorf("expected no rejected, got %v", rejected)
	}
	if len(resolved) != 0 {
		t.Errorf("expected empty resolved (key not in vault), got %v", resolved)
	}
}

func TestResolveVaultSecrets_EmptyAndWhitespaceKeysSkipped(t *testing.T) {
	vault := &mockVault{secrets: map[string]string{"x": "v"}}
	resolved, rejected, err := ResolveVaultSecrets(vault, []string{"", "  ", "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rejected) != 0 {
		t.Errorf("expected no rejected, got %v", rejected)
	}
	if len(resolved) != 1 || resolved["x"] != "v" {
		t.Errorf("unexpected resolved: %v", resolved)
	}
}

func TestResolveVaultSecrets_EmptyValueSkipped(t *testing.T) {
	vault := &mockVault{secrets: map[string]string{"key1": ""}}
	resolved, _, err := ResolveVaultSecrets(vault, []string{"key1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 0 {
		t.Errorf("expected empty resolved for empty-value secret, got %v", resolved)
	}
}

func TestResolveVaultSecrets_PrefixBlockedKeys(t *testing.T) {
	vault := &mockVault{secrets: map[string]string{
		"email_password": "pw",
	}}
	resolved, rejected, err := ResolveVaultSecrets(vault, []string{"email_password"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rejected) != 1 || rejected[0] != "email_password" {
		t.Errorf("expected email_password rejected, got %v", rejected)
	}
	if len(resolved) != 0 {
		t.Errorf("expected empty resolved, got %v", resolved)
	}
}

// ---------------------------------------------------------------------------
// InjectSecretsEnv
// ---------------------------------------------------------------------------

func TestInjectSecretsEnv_AddsEnvVars(t *testing.T) {
	cmd := exec.Command("echo", "test")
	secrets := map[string]string{
		"my_key": "my_value",
	}
	InjectSecretsEnv(cmd, secrets)

	found := false
	for _, e := range cmd.Env {
		if e == "AURAGO_SECRET_MY_KEY=my_value" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected AURAGO_SECRET_MY_KEY=my_value in cmd.Env")
	}
}

func TestInjectSecretsEnv_PreservesExistingEnv(t *testing.T) {
	cmd := exec.Command("echo", "test")
	cmd.Env = []string{"EXISTING=1"}
	secrets := map[string]string{"k": "v"}
	InjectSecretsEnv(cmd, secrets)

	hasExisting := false
	hasSecret := false
	for _, e := range cmd.Env {
		if e == "EXISTING=1" {
			hasExisting = true
		}
		if e == "AURAGO_SECRET_K=v" {
			hasSecret = true
		}
	}
	if !hasExisting {
		t.Error("expected EXISTING=1 to be preserved")
	}
	if !hasSecret {
		t.Error("expected AURAGO_SECRET_K=v to be added")
	}
}

func TestInjectSecretsEnv_EmptySecretsNoop(t *testing.T) {
	cmd := exec.Command("echo", "test")
	InjectSecretsEnv(cmd, map[string]string{})
	if cmd.Env != nil {
		t.Error("expected cmd.Env to remain nil for empty secrets")
	}
}

func TestInjectSecretsEnv_NilSecretsNoop(t *testing.T) {
	cmd := exec.Command("echo", "test")
	InjectSecretsEnv(cmd, nil)
	if cmd.Env != nil {
		t.Error("expected cmd.Env to remain nil for nil secrets")
	}
}

// ---------------------------------------------------------------------------
// BuildSecretPrelude
// ---------------------------------------------------------------------------

func TestBuildSecretPrelude_EmptyMap(t *testing.T) {
	if got := BuildSecretPrelude(map[string]string{}); got != "" {
		t.Errorf("expected empty prelude for empty map, got %q", got)
	}
}

func TestBuildSecretPrelude_NilMap(t *testing.T) {
	if got := BuildSecretPrelude(nil); got != "" {
		t.Errorf("expected empty prelude for nil map, got %q", got)
	}
}

func TestBuildSecretPrelude_SingleSecret(t *testing.T) {
	prelude := BuildSecretPrelude(map[string]string{"api_key": "s3cr3t"})

	if !strings.HasPrefix(prelude, "import os as _aurago_os\n") {
		t.Error("prelude should start with import statement")
	}
	if !strings.HasSuffix(prelude, "del _aurago_os\n") {
		t.Error("prelude should end with del _aurago_os")
	}
	if !strings.Contains(prelude, "_aurago_os.environ['AURAGO_SECRET_API_KEY'] = 's3cr3t'") {
		t.Errorf("prelude missing expected env assignment, got:\n%s", prelude)
	}
}

func TestBuildSecretPrelude_EscapesSingleQuotes(t *testing.T) {
	prelude := BuildSecretPrelude(map[string]string{"key": "it's a secret"})

	if strings.Contains(prelude, "it's a secret") {
		t.Error("single quotes in value should be escaped")
	}
	if !strings.Contains(prelude, `it\'s a secret`) {
		t.Errorf("expected escaped single quote, got:\n%s", prelude)
	}
}

func TestBuildSecretPrelude_EscapesBackslashes(t *testing.T) {
	prelude := BuildSecretPrelude(map[string]string{"key": `path\to\file`})

	if !strings.Contains(prelude, `path\\to\\file`) {
		t.Errorf("expected escaped backslash, got:\n%s", prelude)
	}
}

// ---------------------------------------------------------------------------
// ScrubSecretOutput
// ---------------------------------------------------------------------------

func TestScrubSecretOutput_ReturnsStrings(t *testing.T) {
	// ScrubSecretOutput delegates to security.Scrub. We just verify
	// it returns both strings and doesn't panic.
	out, errOut := ScrubSecretOutput("stdout text", "stderr text")
	if out == "" || errOut == "" {
		t.Error("expected non-empty output from ScrubSecretOutput")
	}
}

func TestScrubSecretOutput_EmptyInputs(t *testing.T) {
	out, errOut := ScrubSecretOutput("", "")
	if out != "" || errOut != "" {
		t.Error("expected empty output for empty inputs")
	}
}

// ---------------------------------------------------------------------------
// IsPythonAccessibleSecret — credential_token_ prefix
// ---------------------------------------------------------------------------

func TestIsPythonAccessibleSecret_BlocksCredentialTokenPrefix(t *testing.T) {
	blocked := []string{
		"credential_token_abc123",
		"credential_token_",
		"credential_password_xyz",
		"credential_certificate_def",
	}
	for _, key := range blocked {
		if IsPythonAccessibleSecret(key) {
			t.Errorf("IsPythonAccessibleSecret(%q) = true, should be blocked", key)
		}
	}
}

// ---------------------------------------------------------------------------
// InjectCredentialEnv
// ---------------------------------------------------------------------------

func TestInjectCredentialEnv_AddsEnvVars(t *testing.T) {
	cmd := exec.Command("echo", "test")

	creds := []CredentialFields{
		{
			Name: "MY_SERVICE",
			Fields: map[string]string{
				"username": "admin",
				"password": "s3cret",
			},
		},
	}
	InjectCredentialEnv(cmd, creds)

	envMap := make(map[string]string)
	for _, e := range cmd.Env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	if envMap["AURAGO_CRED_MY_SERVICE_USERNAME"] != "admin" {
		t.Errorf("USERNAME env = %q, want admin", envMap["AURAGO_CRED_MY_SERVICE_USERNAME"])
	}
	if envMap["AURAGO_CRED_MY_SERVICE_PASSWORD"] != "s3cret" {
		t.Errorf("PASSWORD env = %q, want s3cret", envMap["AURAGO_CRED_MY_SERVICE_PASSWORD"])
	}
}

func TestInjectCredentialEnv_TokenField(t *testing.T) {
	cmd := exec.Command("echo", "test")

	creds := []CredentialFields{
		{
			Name: "API_KEY",
			Fields: map[string]string{
				"username": "svc",
				"token":    "tok-xyz",
			},
		},
	}
	InjectCredentialEnv(cmd, creds)

	envMap := make(map[string]string)
	for _, e := range cmd.Env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	if envMap["AURAGO_CRED_API_KEY_TOKEN"] != "tok-xyz" {
		t.Errorf("TOKEN env = %q, want tok-xyz", envMap["AURAGO_CRED_API_KEY_TOKEN"])
	}
}

func TestInjectCredentialEnv_EmptyCredsNoop(t *testing.T) {
	cmd := exec.Command("echo", "test")
	InjectCredentialEnv(cmd, nil)
	if cmd.Env != nil {
		t.Error("expected cmd.Env to remain nil for empty creds")
	}
	InjectCredentialEnv(cmd, []CredentialFields{})
	if cmd.Env != nil {
		t.Error("expected cmd.Env to remain nil for zero-length creds")
	}
}

func TestInjectCredentialEnv_PreservesExistingEnv(t *testing.T) {
	cmd := exec.Command("echo", "test")
	cmd.Env = []string{"EXISTING=value"}

	creds := []CredentialFields{
		{
			Name:   "SVC",
			Fields: map[string]string{"username": "u"},
		},
	}
	InjectCredentialEnv(cmd, creds)

	found := false
	for _, e := range cmd.Env {
		if e == "EXISTING=value" {
			found = true
			break
		}
	}
	if !found {
		t.Error("existing env var was not preserved")
	}
}

// ---------------------------------------------------------------------------
// BuildCredentialPrelude
// ---------------------------------------------------------------------------

func TestBuildCredentialPrelude_Empty(t *testing.T) {
	if got := BuildCredentialPrelude(nil); got != "" {
		t.Errorf("BuildCredentialPrelude(nil) = %q, want empty", got)
	}
	if got := BuildCredentialPrelude([]CredentialFields{}); got != "" {
		t.Errorf("BuildCredentialPrelude([]) = %q, want empty", got)
	}
}

func TestBuildCredentialPrelude_GeneratesPython(t *testing.T) {
	creds := []CredentialFields{
		{
			Name: "DB_CRED",
			Fields: map[string]string{
				"username": "dbuser",
				"password": "dbpass",
			},
		},
	}
	prelude := BuildCredentialPrelude(creds)

	if !strings.HasPrefix(prelude, "import os as _aurago_os\n") {
		t.Error("prelude should start with import statement")
	}
	if !strings.HasSuffix(prelude, "del _aurago_os\n") {
		t.Error("prelude should end with del _aurago_os")
	}
	if !strings.Contains(prelude, "AURAGO_CRED_DB_CRED_USERNAME") {
		t.Errorf("prelude missing USERNAME env: %s", prelude)
	}
	if !strings.Contains(prelude, "AURAGO_CRED_DB_CRED_PASSWORD") {
		t.Errorf("prelude missing PASSWORD env: %s", prelude)
	}
	if !strings.Contains(prelude, "'dbuser'") {
		t.Error("prelude missing username value")
	}
}

func TestBuildCredentialPrelude_EscapesValues(t *testing.T) {
	creds := []CredentialFields{
		{
			Name: "ESC",
			Fields: map[string]string{
				"password": "it's a p\\ath",
			},
		},
	}
	prelude := BuildCredentialPrelude(creds)

	if strings.Contains(prelude, "it's") {
		t.Error("single quotes should be escaped in prelude")
	}
	if !strings.Contains(prelude, `it\'s`) {
		t.Error("expected escaped single quote in prelude")
	}
	if !strings.Contains(prelude, `p\\ath`) {
		t.Error("expected escaped backslash in prelude")
	}
}
