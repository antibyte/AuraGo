package agent

import (
	"aurago/internal/config"
	"reflect"
	"strings"
)

// dispatchAuthorization keeps run-local providers, schemas, role and scope
// intact. Only authorization gates are intersected with the current snapshot.
// Compound policy changes cannot safely be unioned: require a new run instead.
func dispatchAuthorization(cfg *config.Config) (*config.Config, bool) {
	if cfg == nil || cfg.AuthorizationSnapshots == nil {
		return cfg, true
	}
	baseline, current := cfg.AuthorizationSnapshots()
	if baseline == nil || current == nil {
		return cfg, false
	}
	if current == cfg {
		return cfg, true
	}
	if !sameS3AuthorizationTarget(baseline, current) {
		return cfg, false
	}
	merged := *cfg
	merged.Treg.MaxCallCostMicro = min(cfg.Treg.MaxCallCostMicro, current.Treg.MaxCallCostMicro)
	ok := intersectAuthorizationFields(reflect.ValueOf(&merged).Elem(), reflect.ValueOf(current).Elem(), reflect.ValueOf(baseline).Elem())
	return &merged, ok
}

func intersectAuthorizationFields(dst, current, baseline reflect.Value) bool {
	for i := 0; i < dst.NumField(); i++ {
		field := dst.Type().Field(i)
		a, b, original := dst.Field(i), current.Field(i), baseline.Field(i)
		if !a.CanSet() || field.PkgPath != "" {
			continue
		}
		name := field.Name
		// Config's policy naming contract covers both legacy readonly spellings.
		restriction := name == "ReadOnly" || name == "Readonly"
		grant := name == "Enabled" || strings.HasPrefix(name, "Allow") || name == "AgentAccess" || name == "SudoEnabled" || name == "SudoUnrestricted"
		compound := strings.Contains(name, "Allowed") || strings.HasPrefix(name, "Blocked") || strings.HasPrefix(name, "Denied") || strings.Contains(name, "Allowlist") || strings.Contains(name, "Blocklist") || name == "Permissions" || name == "ExportedTools"
		if compound && !reflect.DeepEqual(original.Interface(), b.Interface()) {
			return false
		}
		switch a.Kind() {
		case reflect.Struct:
			if !intersectAuthorizationFields(a, b, original) {
				return false
			}
		case reflect.Bool:
			if restriction {
				a.SetBool(a.Bool() || b.Bool())
			} else if grant {
				a.SetBool(a.Bool() && b.Bool())
			}
		case reflect.Pointer:
			if a.Type().Elem().Kind() == reflect.Bool && (grant || restriction) {
				// Nil means unspecified. An explicit denial always wins.
				deny := func(v reflect.Value) bool { return !v.IsNil() && v.Elem().Bool() == restriction }
				if deny(a) || deny(b) {
					value := reflect.New(a.Type().Elem())
					value.Elem().SetBool(restriction)
					a.Set(value)
				}
			}
		case reflect.Slice, reflect.Map:
			// Integration/server/account lists carry identity-bound permissions.
			// Changing their membership or policy requires a fresh run snapshot.
			if (name == "Servers" || name == "Accounts" || name == "Toolkits") && !reflect.DeepEqual(original.Interface(), b.Interface()) {
				return false
			}
		}
	}
	return true
}

const authorizationChangedOutput = `Tool Output: {"status":"policy_denied","code":"authorization_changed","message":"Authorization changed during this run. Start a new request to use the current policy."}`

// Schema removal is also an execution revocation, including handlers supplied
// by isolated runners. Ordinary operation-level read-only checks stay in their
// handlers, using the intersected config.
func revokedNativeTools(original, effective *config.Config) map[string]bool {
	if original == nil || effective == nil || original == effective {
		return nil
	}
	before := buildToolFeatureFlags(RunConfig{Config: original}, BuildToolingPolicy(original, ""))
	after := buildToolFeatureFlags(RunConfig{Config: effective}, BuildToolingPolicy(effective, ""))
	if reflect.DeepEqual(before, after) {
		return nil
	}
	revoked := map[string]bool{}
	for _, schema := range builtinToolSchemasCached(before) {
		if schema.Function != nil {
			revoked[schema.Function.Name] = true
		}
	}
	for _, schema := range builtinToolSchemasCached(after) {
		if schema.Function != nil {
			delete(revoked, schema.Function.Name)
		}
	}
	return revoked
}

// Hook closures may retain their runner's original config. A tightened gate
// requires a fresh runner rather than executing with that captured authority.
func authorizationGatesDiffer(a, b reflect.Value) bool {
	for i := 0; i < a.NumField(); i++ {
		// Match the intersection's exported-policy boundary. Private config
		// bookkeeping cannot be accessed through reflect.Value.Interface.
		if a.Type().Field(i).PkgPath != "" {
			continue
		}
		x, y := a.Field(i), b.Field(i)
		switch x.Kind() {
		case reflect.Struct:
			if authorizationGatesDiffer(x, y) {
				return true
			}
		case reflect.Bool:
			if x.Bool() != y.Bool() {
				return true
			}
		case reflect.Pointer:
			if x.Type().Elem().Kind() == reflect.Bool && !reflect.DeepEqual(x.Interface(), y.Interface()) {
				return true
			}
		}
	}
	return false
}

// A target change cannot redirect an already admitted tool chain to another store.
func sameS3AuthorizationTarget(a, b *config.Config) bool {
	if a == nil || b == nil {
		return a == b
	}
	// Dormant S3 settings cannot redirect this run; permission changes are
	// still checked by intersectAuthorizationFields.
	if !a.S3.Enabled && !b.S3.Enabled {
		return true
	}
	provider := func(c *config.Config) string {
		if c.S3.Provider == "" {
			return config.S3ProviderExternal
		}
		return c.S3.Provider
	}
	if provider(a) != provider(b) {
		return false
	}
	if provider(a) == config.S3ProviderRustFS {
		return a.RustFS.Bucket == b.RustFS.Bucket
	}
	return a.S3.Endpoint == b.S3.Endpoint && a.S3.Region == b.S3.Region && a.S3.Bucket == b.S3.Bucket && a.S3.AccessKey == b.S3.AccessKey && a.S3.SecretKey == b.S3.SecretKey && a.S3.Insecure == b.S3.Insecure && a.S3.UsePathStyle == b.S3.UsePathStyle
}
