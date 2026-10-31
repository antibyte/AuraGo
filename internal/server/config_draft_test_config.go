package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"

	"gopkg.in/yaml.v3"
)

// draftTestMaxBody caps the request body read by draftTestConfig.
const draftTestMaxBody = 10 << 20

// draftConfigMask is the placeholder the config API sends for stored secrets.
const draftConfigMask = "••••••••"

// draftConfigError marks a rejected draft (HTTP 400). The reason never contains
// secret values.
type draftConfigError struct{ reason string }

func (e *draftConfigError) Error() string { return e.reason }

func newDraftConfigError(format string, args ...interface{}) error {
	return &draftConfigError{reason: fmt.Sprintf(format, args...)}
}

// draftTestConfigStatus maps an error returned by draftTestConfig to an HTTP status.
func draftTestConfigStatus(err error) int {
	var draftErr *draftConfigError
	if errors.As(err, &draftErr) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// writeDraftTestConfigError answers a failed draftTestConfig call with
// {"status":"error","message":reason} and 400 (draft rejected) or 500.
func writeDraftTestConfigError(w http.ResponseWriter, err error) {
	writeIntegrationTestResult(w, draftTestConfigStatus(err), "error", security.Scrub(err.Error()))
}

// draftSecretReader serves draft secret values first and the real vault second.
// It is read-only: nothing is ever written to the vault.
type draftSecretReader struct {
	overrides map[string]string
	base      config.SecretReader
	// blocked vault keys belong to sections whose target changed in the draft;
	// the saved value must not travel to the new target.
	blocked map[string]bool
}

func (r draftSecretReader) ReadSecret(key string) (string, error) {
	if value, ok := r.overrides[key]; ok && value != "" {
		return value, nil
	}
	if r.blocked[key] || r.base == nil {
		return "", security.ErrSecretNotFound
	}
	return r.base.ReadSecret(key)
}

// draftSecretCollector receives the secrets that extractRecursive would write to
// the vault and keeps them in memory instead.
type draftSecretCollector map[string]string

func (c draftSecretCollector) WriteSecret(key, value string) error {
	c[key] = value
	return nil
}

var (
	draftTopLevelKeysOnce sync.Once
	draftTopLevelKeys     map[string]struct{}
)

// configTopLevelKeys lists the YAML sections of config.Config.
func configTopLevelKeys() map[string]struct{} {
	draftTopLevelKeysOnce.Do(func() {
		keys := map[string]struct{}{}
		typ := reflect.TypeOf(config.Config{})
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if !field.IsExported() {
				continue
			}
			name := strings.Split(field.Tag.Get("yaml"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = strings.ToLower(field.Name)
			}
			keys[name] = struct{}{}
		}
		draftTopLevelKeys = keys
	})
	return draftTopLevelKeys
}

// draftTestConfig builds the configuration a connection test should use.
//
// The request body is optional: {"draft": <patch>} where the patch has the shape
// of PUT /api/config. Without a draft the result is a deep clone of the saved
// configuration snapshot (vault secrets included). With a draft the patch is
// overlaid on that clone; secret leaves (sensitive key names or vaultKeyMap
// paths, including vault-only yaml:"-" fields) take the draft value when it is
// non-empty and not the mask, otherwise the saved/vault value stays.
//
// Target rule (stored-secret exfiltration guard, automatic for every caller):
// if the draft changes a target leaf of a section (draftTargetFieldNames: url,
// host, port, tls/insecure, user names, region, bucket, ...), the section's saved
// secrets (struct fields and vault keys) are NOT reused. Only secrets typed in
// the draft remain; if a secret that was saved is then missing, the call fails
// with a 400 draftConfigError asking to enter the credentials again. Handlers
// must therefore take credentials from the returned config only and must not
// re-read the vault themselves (pass nil as vault to clients).
//
// Also: provider references in the draft are validated like PUT /api/config and
// the resolved image/video provider slots are reset before resolution.
// Limitation: masked secrets inside list items (klipper printers, MCP, SQL,
// remote agents) are dropped and the list is replaced, so list-based tests need
// an id-based fallback of their own plus the same target rule.
//
// Nothing is persisted: no config file, no vault write, no snapshot mutation.
// r.Body is restored so the caller may still read its own fields. A rejected
// draft yields a *draftConfigError (HTTP 400, see writeDraftTestConfigError).
func (s *Server) draftTestConfig(r *http.Request) (*config.Config, error) {
	saved := s.ConfigSnapshot()
	if saved == nil {
		return nil, errors.New("Configuration is unavailable")
	}
	draft, err := readDraftPatch(r)
	if err != nil {
		return nil, err
	}
	cfg := saved.Clone()
	if len(draft) == 0 {
		return cfg, nil
	}
	if err := validateDraftPatch(draft); err != nil {
		return nil, err
	}
	if err := validateProviderReferencePatch(saved, draft); err != nil {
		return nil, newDraftConfigError("Invalid draft: %s", err.Error())
	}
	draftSections := make([]string, 0, len(draft))
	for section := range draft {
		draftSections = append(draftSections, section)
	}
	sort.Strings(draftSections)

	// Pull every vault-backed secret out of the patch (in memory only), and note
	// sensitive leaves without a vault key so they can still be applied.
	yamlSecrets := collectUnmappedDraftSecrets(draft, "")
	overrides := draftSecretCollector{}
	if err := extractRecursive(draft, "", overrides, discardLogger()); err != nil {
		return nil, newDraftConfigError("Invalid draft: %s", err.Error())
	}
	registerDraftSecretsScoped(r, overrides)

	overlay := map[string]interface{}{}
	stripDraftMaskedLeaves(draft)
	deepMerge(overlay, convertNumericKeyedDraftMaps(draft), "")
	overlay = normalizeConfigYAMLMap(overlay)
	out, err := yaml.Marshal(overlay)
	if err != nil {
		return nil, newDraftConfigError("Invalid draft: cannot encode patch")
	}
	if err := yaml.Unmarshal(out, cfg); err != nil {
		return nil, newDraftConfigError("Invalid draft: values do not match the configuration schema")
	}

	// Target rule: when the draft points a section at another target, none of
	// that section's saved secrets may follow. Clear them (struct fields and
	// vault keys); only secrets typed in the draft remain.
	changed := draftChangedTargetSections(saved, cfg, draftSections)
	blocked := map[string]bool{}
	for _, section := range changed {
		zeroDraftSectionSecrets(cfg, section)
		for path, vaultKey := range vaultKeyMap {
			if strings.HasPrefix(path, section+".") {
				blocked[vaultKey] = true
			}
		}
	}
	if len(yamlSecrets) > 0 {
		secretOverlay := map[string]interface{}{}
		deepSetDraftSecrets(secretOverlay, yamlSecrets)
		secretYAML, err := yaml.Marshal(secretOverlay)
		if err != nil || yaml.Unmarshal(secretYAML, cfg) != nil {
			return nil, newDraftConfigError("Invalid draft: values do not match the configuration schema")
		}
	}

	var base config.SecretReader
	if s.Vault != nil {
		base = s.Vault
	}
	reader := draftSecretReader{overrides: overrides, base: base, blocked: blocked}
	cfg.ApplyVaultSecrets(reader)
	for _, section := range changed {
		if section == "mqtt" && overrides[config.MQTTPasswordVaultKey] == "" {
			cfg.MQTT.Password = "" // ApplyVaultSecrets may fall back to MQTT_PASSWORD
		}
	}
	// Resolved slots are only overwritten when a provider is found; reset them
	// so a cleared or changed provider cannot keep testing the old one.
	cfg.ImageGeneration.ProviderType, cfg.ImageGeneration.BaseURL = "", ""
	cfg.ImageGeneration.APIKey, cfg.ImageGeneration.ResolvedModel = "", ""
	cfg.VideoGeneration.ProviderType, cfg.VideoGeneration.BaseURL = "", ""
	cfg.VideoGeneration.APIKey, cfg.VideoGeneration.ResolvedModel = "", ""
	cfg.ResolveProviders()
	cfg.ApplyOAuthTokens(reader)
	for _, section := range changed {
		if missing := missingDraftSectionSecret(saved, cfg, section); missing != "" {
			if draftSecretSaveFirst(section, missing) {
				return nil, newDraftConfigError("The target of %q changed: save it first, its generated credentials cannot be entered here (Ziel geändert – bitte zuerst speichern)", section)
			}
			return nil, newDraftConfigError("The target of %q changed: enter the credentials again to test it (Ziel geändert – bitte Zugangsdaten erneut eingeben)", section)
		}
	}
	cfg.Runtime = saved.Runtime
	cfg.ConfigPath = saved.ConfigPath
	return cfg, nil
}

// draftScopedSecretTTL bounds how long a draft secret stays registered when the
// request context never ends (tests, background contexts).
const draftScopedSecretTTL = 2 * time.Minute

// registerDraftSecretsScoped keeps the typed draft secrets out of logs and error
// text while the request runs, without adding them to the permanent scrubber
// registry: a draft value is not saved, so it must not redact unrelated text
// for the rest of the process. The values are released when the request context
// ends, at the latest after draftScopedSecretTTL.
func registerDraftSecretsScoped(r *http.Request, secrets map[string]string) {
	releases := make([]func(), 0, len(secrets))
	for _, value := range secrets {
		releases = append(releases, security.RegisterScopedSensitiveExact(value))
	}
	if len(releases) == 0 {
		return
	}
	release := func() {
		for _, fn := range releases {
			fn()
		}
	}
	time.AfterFunc(draftScopedSecretTTL, release)
	if r != nil {
		context.AfterFunc(r.Context(), release)
	}
}

// readDraftPatch parses the optional {"draft": {...}} body and restores r.Body.
func readDraftPatch(r *http.Request) (map[string]interface{}, error) {
	if r == nil || r.Body == nil || r.Body == http.NoBody {
		return nil, nil
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, draftTestMaxBody+1))
	_ = r.Body.Close()
	if err != nil {
		return nil, newDraftConfigError("Failed to read request body")
	}
	if len(raw) > draftTestMaxBody {
		return nil, newDraftConfigError("Request body too large")
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var envelope struct {
		Draft json.RawMessage `json:"draft"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, newDraftConfigError("Invalid JSON")
	}
	trimmed := bytes.TrimSpace(envelope.Draft)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var draft map[string]interface{}
	if err := json.Unmarshal(trimmed, &draft); err != nil {
		return nil, newDraftConfigError("Invalid draft: it must be an object")
	}
	return draft, nil
}

// validateDraftPatch applies the PUT /api/config guards that matter for a
// non-persisted overlay: only known sections, no sections with dedicated APIs
// and complete lists (a sparse list would replace the saved one).
func validateDraftPatch(patch map[string]interface{}) error {
	delete(patch, "_effective_tool_policy")
	delete(patch, "_config_migrations")
	known := configTopLevelKeys()
	for key := range patch {
		if _, ok := known[key]; !ok {
			return newDraftConfigError("Invalid draft: unknown config section %q", key)
		}
	}
	if _, present := patch["sip"]; present {
		return newDraftConfigError("Invalid draft: SIP settings use /api/sip/config")
	}
	if _, present := patch["providers"]; present {
		return newDraftConfigError("Invalid draft: provider settings use /api/providers")
	}
	if err := config.ValidatePersonalityLegacyPatch(patch); err != nil {
		return newDraftConfigError("Invalid draft: %s", err.Error())
	}
	listErr := validateGo2RTCStreamsPatch(patch)
	if listErr == nil {
		listErr = validateCompleteConfigListPatch(patch)
	}
	if listErr != nil {
		return newDraftConfigError("Invalid draft: %s", listErr.Error())
	}
	return nil
}

// collectUnmappedDraftSecrets finds string leaves with a sensitive key name but
// no vault key; PUT drops them, the draft applies them to the YAML-backed field.
func collectUnmappedDraftSecrets(m map[string]interface{}, prefix string) map[string]string {
	found := map[string]string{}
	for key, val := range m {
		fullPath := key
		if prefix != "" {
			fullPath = prefix + "." + key
		}
		switch v := val.(type) {
		case map[string]interface{}:
			for path, value := range collectUnmappedDraftSecrets(v, fullPath) {
				found[path] = value
			}
		case string:
			if _, mapped := vaultKeyMap[fullPath]; mapped || !sensitiveKeys[key] {
				continue
			}
			if trimmed := strings.TrimSpace(v); trimmed != "" && trimmed != draftConfigMask {
				found[fullPath] = trimmed
			}
		}
	}
	return found
}

func deepSetDraftSecrets(overlay map[string]interface{}, secrets map[string]string) {
	for path, value := range secrets {
		parts := strings.Split(path, ".")
		current := overlay
		for _, part := range parts[:len(parts)-1] {
			next, ok := current[part].(map[string]interface{})
			if !ok {
				next = map[string]interface{}{}
				current[part] = next
			}
			current = next
		}
		current[parts[len(parts)-1]] = value
	}
}

// stripDraftMaskedLeaves removes the mask placeholder (and empty sensitive
// values) inside lists, where deepMerge would otherwise copy them verbatim.
func stripDraftMaskedLeaves(value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if str, ok := child.(string); ok && (str == draftConfigMask || sensitiveKeys[key] && strings.TrimSpace(str) == "") {
				delete(typed, key)
				continue
			}
			stripDraftMaskedLeaves(child)
		}
	case []interface{}:
		for _, child := range typed {
			stripDraftMaskedLeaves(child)
		}
	}
}

// convertNumericKeyedDraftMaps turns {"0":..,"1":..} objects (a form-library
// array encoding the PUT path also accepts) into lists.
func convertNumericKeyedDraftMaps(value map[string]interface{}) map[string]interface{} {
	for key, child := range value {
		switch typed := child.(type) {
		case map[string]interface{}:
			if converted, ok := numericKeyedMapToSlice(typed); ok {
				value[key] = convertNumericKeyedDraftList(converted)
				continue
			}
			convertNumericKeyedDraftMaps(typed)
		case []interface{}:
			convertNumericKeyedDraftList(typed)
		}
	}
	return value
}

func convertNumericKeyedDraftList(list []interface{}) []interface{} {
	for i, item := range list {
		if m, ok := item.(map[string]interface{}); ok {
			if converted, isList := numericKeyedMapToSlice(m); isList {
				list[i] = convertNumericKeyedDraftList(converted)
			} else {
				convertNumericKeyedDraftMaps(m)
			}
		}
	}
	return list
}

// discardLogger returns a logger that drops everything; draft handling must not
// log (the PUT logger names secret paths and vault keys).
func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// testConfigForRequest serves the legacy GET test endpoints that also accept POST:
// GET keeps testing the saved live config, POST tests {"draft": ...} through
// draftTestConfig. On failure it has already answered the request.
func (s *Server) testConfigForRequest(w http.ResponseWriter, r *http.Request) (*config.Config, bool) {
	switch r.Method {
	case http.MethodPost:
		cfg, err := s.draftTestConfig(r)
		if err != nil {
			writeDraftTestConfigError(w, err)
			return nil, false
		}
		return cfg, true
	case http.MethodGet:
		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()
		return cfg, true
	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return nil, false
	}
}

// draftTargetFieldNames lists the config leaves (lower case, '_' and '-'
// removed) that decide WHERE a section's credentials are sent. When any of them
// differs between the saved and the drafted section, saved secrets of that
// section are not reused (see draftChangedTargetSections). Matching is recursive
// within the section. Struct-valued matches (for example a "tls" block) are
// compared as a whole. Lists of entries (printers, remote agents, MCP servers,
// SQL connections) are NOT compared: converted modules with list-based tests
// must fall back per entry id and apply the same rule themselves; masked
// secrets inside list items are dropped by the draft overlay.
var draftTargetFieldNames = map[string]bool{
	"url": true, "baseurl": true, "apiurl": true, "serverurl": true, "host": true,
	"hostname": true, "endpoint": true, "server": true, "address": true, "addr": true,
	"port": true, "internalport": true, "webport": true, "scheme": true, "usehttps": true,
	"https": true, "ssl": true, "usessl": true, "tls": true, "insecure": true,
	"insecureskipverify": true, "verifytls": true, "tlsskipverify": true, "username": true,
	"user": true, "userid": true, "tokenid": true, "region": true, "bucket": true,
	"account": true, "accountid": true, "broker": true, "domain": true, "tenant": true,
	"tenantid": true, "inbox": true, "usetls": true, "tlsmode": true, "starttls": true, "pathstyle": true, "usepathstyle": true,
	"deployport": true, "deployuser": true, "cafile": true,
}

// draftSecretNameFragments mark yaml:"-" string fields without vault tag as secrets.
var draftSecretNameFragments = []string{
	"password", "token", "secret", "apikey", "accesskey", "authkey",
	"privatekey", "passphrase", "sharedkey", "deploykey", "credential",
}

// draftTargetFieldSuffixes catch prefixed target leaves such as hub_base_url,
// webhook_url or smtp_host that the exact list above does not name.
var draftTargetFieldSuffixes = []string{"url", "host", "hostname", "endpoint"}

func isDraftTargetFieldName(name string) bool {
	if draftTargetFieldNames[name] {
		return true
	}
	for _, suffix := range draftTargetFieldSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func normalizeDraftFieldName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "")
	return strings.ReplaceAll(name, "-", "")
}

func draftFieldName(field reflect.StructField) string {
	name := strings.Split(field.Tag.Get("yaml"), ",")[0]
	if name == "" || name == "-" {
		name = field.Name
	}
	return normalizeDraftFieldName(name)
}

func isDraftSecretField(field reflect.StructField) bool {
	if field.Type.Kind() != reflect.String {
		return false
	}
	name := draftFieldName(field)
	if isDraftTargetFieldName(name) {
		return false
	}
	yamlName := strings.Split(field.Tag.Get("yaml"), ",")[0]
	if field.Tag.Get("vault") != "" || sensitiveKeys[yamlName] {
		return true
	}
	// An identifier such as credential_id points at a secret, it is not one.
	if strings.HasSuffix(name, "id") {
		return false
	}
	for _, fragment := range draftSecretNameFragments {
		if strings.Contains(name, fragment) {
			return true
		}
	}
	return false
}

func draftSectionValue(cfg *config.Config, section string) (reflect.Value, bool) {
	root := reflect.ValueOf(cfg).Elem()
	typ := root.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		name := strings.Split(field.Tag.Get("yaml"), ",")[0]
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		if name == section && field.Type.Kind() == reflect.Struct {
			return root.Field(i), true
		}
	}
	return reflect.Value{}, false
}

// draftTargetChanged reports whether a target leaf differs between two values
// of the same struct type.
func draftTargetChanged(saved, drafted reflect.Value) bool {
	typ := saved.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() || field.Type.Kind() == reflect.Func {
			continue
		}
		if isDraftTargetFieldName(draftFieldName(field)) {
			if !reflect.DeepEqual(saved.Field(i).Interface(), drafted.Field(i).Interface()) {
				return true
			}
			continue
		}
		if field.Type.Kind() == reflect.Struct && draftTargetChanged(saved.Field(i), drafted.Field(i)) {
			return true
		}
	}
	return false
}

// draftChangedTargetSections returns the draft sections whose target changed.
func draftChangedTargetSections(saved, drafted *config.Config, sections []string) []string {
	var changed []string
	for _, section := range sections {
		savedValue, ok := draftSectionValue(saved, section)
		if !ok {
			continue
		}
		draftedValue, _ := draftSectionValue(drafted, section)
		if draftTargetChanged(savedValue, draftedValue) {
			changed = append(changed, section)
		}
	}
	return changed
}

// visitDraftSecretFields calls fn for every secret string field of a section.
func visitDraftSecretFields(value reflect.Value, path string, fn func(path string, field reflect.Value)) {
	typ := value.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		fieldPath := path + "." + draftFieldName(field)
		switch {
		case isDraftSecretField(field):
			fn(fieldPath, value.Field(i))
		case field.Type.Kind() == reflect.Struct:
			visitDraftSecretFields(value.Field(i), fieldPath, fn)
		}
	}
}

func zeroDraftSectionSecrets(cfg *config.Config, section string) {
	value, ok := draftSectionValue(cfg, section)
	if !ok {
		return
	}
	visitDraftSecretFields(value, section, func(_ string, field reflect.Value) {
		if field.CanSet() {
			field.SetString("")
		}
	})
}

// draftSectionNeedsSecret says whether a connection test of the section uses the
// secret at path (normalized, as visitDraftSecretFields reports it). Sections
// without a rule need every secret. Secrets a test does not use must not be
// demanded after a target change, least of all vault-only ones the user cannot
// type: the virtual computers storage test needs the S3 keys in external mode
// and the generated Garage keys in managed mode.
func draftSectionNeedsSecret(drafted *config.Config, section, path string) bool {
	if section != "virtual_computers" {
		return true
	}
	external := drafted.VirtualComputers.Storage.Mode == config.VirtualComputersStorageModeExternalS3
	switch path {
	case "virtual_computers.s3accesskeyid", "virtual_computers.s3secretkey":
		return external
	case "virtual_computers.garageaccesskeyid", "virtual_computers.garagesecretkey":
		return !external
	}
	return false
}

// draftSecretSaveFirst reports a needed secret that has no editable field, so the
// user cannot enter it again: the target must be saved (and set up) first.
func draftSecretSaveFirst(section, path string) bool {
	return section == "virtual_computers" && strings.HasPrefix(path, "virtual_computers.garage")
}

// missingDraftSectionSecret returns the path of a secret that was set in the
// saved section but is empty in the draft (target changed, nothing typed) and
// that a test of the section needs (draftSectionNeedsSecret).
func missingDraftSectionSecret(saved, drafted *config.Config, section string) string {
	savedValue, ok := draftSectionValue(saved, section)
	if !ok {
		return ""
	}
	draftedValue, _ := draftSectionValue(drafted, section)
	savedSecrets := map[string]string{}
	visitDraftSecretFields(savedValue, section, func(path string, field reflect.Value) {
		savedSecrets[path] = field.String()
	})
	missing := ""
	visitDraftSecretFields(draftedValue, section, func(path string, field reflect.Value) {
		if savedSecrets[path] != "" && field.String() == "" && missing == "" && draftSectionNeedsSecret(drafted, section, path) {
			missing = path
		}
	})
	return missing
}
