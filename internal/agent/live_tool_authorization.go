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
	merged := *cfg
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
		grant := name == "Enabled" || strings.HasPrefix(name, "Allow") || name == "SudoEnabled" || name == "SudoUnrestricted"
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
