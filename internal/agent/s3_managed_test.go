package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func managedS3AgentConfig() *config.Config {
	c := &config.Config{}
	c.S3.Enabled = true
	c.S3.Provider = config.S3ProviderRustFS
	c.RustFS = config.RustFSConfig{Enabled: true, Bucket: "aurago", AgentAccess: true, Permissions: config.RustFSPermissions{AllowRead: true, AllowedLocations: []config.RustFSLocation{{Bucket: "aurago", Prefixes: []string{"agent/"}}}}}
	c.S3Availability = func(*config.Config) config.FeatureAvailability { return config.FeatureAvailability{Available: true} }
	c.AcquireS3 = func(ctx context.Context, snapshot *config.Config, _ bool) (*config.RustFSS3Lease, error) {
		owner, cancel := context.WithCancel(ctx)
		return &config.RustFSS3Lease{Context: owner, Managed: true, Endpoint: "http://127.0.0.1:1", Region: "us-east-1", Bucket: "aurago", AccessKey: "s3-agent-test-access", SecretKey: "s3-agent-test-secret", Permissions: snapshot.Clone().RustFS.Permissions, Release: cancel, Check: owner.Err, Publish: func(commit func() error) error {
			if err := owner.Err(); err != nil {
				return err
			}
			return commit()
		}}, nil
	}
	return c
}

func TestS3ManagedDiscoveryUsesCurrentAccessAndPassiveReadiness(t *testing.T) {
	c := managedS3AgentConfig()
	if !buildToolFlagsFromConfig(c).S3Enabled || !resolveToolFeatureState(RunConfig{Config: c}, BuildToolingPolicy(c, "")).ToolFlags.S3Enabled {
		t.Fatal("ready managed source missing")
	}
	c.S3Availability = func(*config.Config) config.FeatureAvailability {
		return config.FeatureAvailability{Reason: "not provisioned"}
	}
	if !buildToolFlagsFromConfig(c).S3Enabled || resolveToolFeatureState(RunConfig{Config: c}, BuildToolingPolicy(c, "")).ToolFlags.S3Enabled {
		t.Fatal("runtime readiness did not gate resolved catalog")
	}
	if got := toolRuntimeUnavailable(&DispatchContext{Cfg: c}, "s3_storage"); got != "not provisioned" {
		t.Fatal("missing setup reason")
	}
	c.RustFS.AgentAccess = false
	if buildToolFlagsFromConfig(c).S3Enabled {
		t.Fatal("agent access off still exposed schema")
	}
	c.S3.Provider = "external"
	if !buildToolFlagsFromConfig(c).S3Enabled {
		t.Fatal("managed access changed legacy external availability")
	}
}

func TestS3ManagedPermissionsApplyToDirectAliasAndWrappedCalls(t *testing.T) {
	for _, route := range []string{"direct", "alias", "invoke", "skill"} {
		t.Run(route, func(t *testing.T) {
			c := managedS3AgentConfig()
			dc := &DispatchContext{Cfg: c, SessionID: t.Name(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			args := map[string]interface{}{"operation": "list_objects", "bucket": "aurago", "prefix": "forbidden/"}
			call := ToolCall{Action: "s3_storage", Params: args}
			switch route {
			case "alias":
				call.Action = "s3"
			case "invoke":
				setRunDiscoverToolsState(dc, dispatchCatalogSchemas(dc), nil)
				t.Cleanup(func() { ClearDiscoverToolsState(t.Name()) })
				call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "s3_storage", "arguments": args}}
			case "skill":
				call = ToolCall{Action: "execute_skill", Skill: "s3_storage", SkillArgs: args}
			}
			result := DispatchToolCallResult(context.Background(), &call, dc, "list allowed data")
			if !result.IsError || !strings.Contains(result.Output, "s3_permission_denied") {
				t.Fatalf("%s did not reach scoped leaf: %s", route, result.Output)
			}
		})
	}
}

func TestS3AuthorizationTargetAndPolicyChangesNeedFreshRun(t *testing.T) {
	for _, change := range []struct {
		name string
		edit func(*config.Config)
	}{
		{"provider", func(c *config.Config) { c.S3.Provider = "external" }},
		{"bucket", func(c *config.Config) { c.RustFS.Bucket = "another-bucket" }},
		{"prefix", func(c *config.Config) { c.RustFS.Permissions.AllowedLocations[0].Prefixes = []string{"other/"} }},
	} {
		t.Run(change.name, func(t *testing.T) {
			baseline := managedS3AgentConfig()
			current := baseline.Clone()
			change.edit(current)
			baseline.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return baseline, current }
			if _, ok := dispatchAuthorization(baseline); ok {
				t.Fatal("changed S3 authority reused an existing run")
			}
		})
	}
	for _, field := range []string{"endpoint", "credentials", "region", "path style"} {
		t.Run("external-"+field, func(t *testing.T) {
			c := &config.Config{}
			c.S3.Enabled = true
			current := c.Clone()
			switch field {
			case "endpoint":
				current.S3.Endpoint = "https://different.invalid"
			case "credentials":
				current.S3.SecretKey = "rotated-test-key"
			case "region":
				current.S3.Region = "changed"
			case "path style":
				current.S3.UsePathStyle = true
			}
			c.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return c, current }
			if _, ok := dispatchAuthorization(c); ok {
				t.Fatal("external target identity drift ignored")
			}
		})
	}
	c := managedS3AgentConfig()
	current := c.Clone()
	current.Agent.DebugMode = true
	c.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return c, current }
	if _, ok := dispatchAuthorization(c); !ok {
		t.Fatal("unrelated save revoked S3 authority")
	}
}

func TestS3ActionTargetsUseBothObjectIdentities(t *testing.T) {
	c := managedS3AgentConfig()
	call := ToolCall{Action: "s3_storage", Params: map[string]interface{}{"operation": "move", "key": "agent/source | tool:other", "destination_bucket": "other-bucket", "destination_key": "archive/end "}}
	a := DescribeAction(call, &DispatchContext{Cfg: c})
	for _, wanted := range []string{"s3_provider:rustfs", "s3_object:rustfs/aurago/agent%2Fsource%20%7C%20tool:other", "s3_object:rustfs/other-bucket/archive%2Fend%20"} {
		if !strings.Contains(a.Target, wanted) {
			t.Fatalf("missing exact object identity %q in %s", wanted, a.Target)
		}
	}
	if a.targetUnknown {
		t.Fatal("known S3 targets marked unknown")
	}
}

func TestS3DecodedTargetMatchesNormalizedLeafArguments(t *testing.T) {
	c := managedS3AgentConfig()
	call := ToolCall{Action: "s3_storage", Params: map[string]interface{}{"operation": " MOVE ", "key": "agent/source", "destination_bucket": "  ", "destination_key": "agent/end"}}
	req := decodeS3Args(call)
	if req.Operation != "move" {
		t.Fatal("operation not normalized")
	}
	a := DescribeAction(call, &DispatchContext{Cfg: c})
	if !strings.Contains(a.Target, "s3_object:rustfs/aurago/agent%2Fend") {
		t.Fatal("destination target differs from execution")
	}
}
