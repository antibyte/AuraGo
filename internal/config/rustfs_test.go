package config

import (
	"context"
	"encoding/json"
	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func TestRustFSDefaultsAndLiteralPermissions(t *testing.T) {
	var c Config
	if err := NormalizeRustFSConfig(&c); err != nil {
		t.Fatal(err)
	}
	if c.S3.Provider != "external" || c.RustFS.Bucket != "aurago" || c.RustFS.Enabled || c.RustFS.AgentAccess {
		t.Fatal("unsafe defaults")
	}
	c.RustFS.Permissions = RustFSPermissions{AllowRead: true, AllowedLocations: []RustFSLocation{{Bucket: "aurago", Prefixes: []string{"agent/"}}}}
	for _, tc := range []struct {
		key     string
		allowed bool
	}{{"agent/a", true}, {"agent2/a", false}, {"Agent/a", false}, {"agent%2Fa", false}, {"agent/../literal", true}} {
		if got := c.RustFS.Permissions.Allows("aurago", tc.key, "read"); got != tc.allowed {
			t.Errorf("%q: %v", tc.key, got)
		}
	}
	if c.RustFS.Permissions.Allows("other", "agent/a", "read") || c.RustFS.Permissions.Allows("aurago", "agent/a", "write") {
		t.Fatal("permission widened")
	}
	if prefix, err := c.RustFS.Permissions.ListingPrefix("aurago", ""); err != nil || prefix != "agent/" {
		t.Fatalf("prefix %q: %v", prefix, err)
	}
	c.RustFS.Permissions.AllowedLocations[0].Prefixes = append(c.RustFS.Permissions.AllowedLocations[0].Prefixes, "second/")
	if _, err := c.RustFS.Permissions.ListingPrefix("aurago", ""); err == nil {
		t.Fatal("ambiguous prefix accepted")
	}
	for _, bad := range []string{"*", "agent/?", "${aws:username}/", "agent/\n"} {
		c.RustFS.Permissions.AllowedLocations[0].Prefixes = []string{bad}
		if NormalizeRustFSConfig(&c) == nil {
			t.Errorf("accepted prefix %q", bad)
		}
	}
}

func TestRustFSRuntimeCallbacksStayPrivateAndCloneable(t *testing.T) {
	var c Config
	calls := 0
	c.AcquireS3 = func(context.Context, *Config, bool) (*RustFSS3Lease, error) {
		calls++
		return &RustFSS3Lease{AccessKey: "private-access", SecretKey: "private-secret"}, nil
	}
	c.S3Availability = func(*Config) FeatureAvailability { return FeatureAvailability{Available: true} }
	clone := c.Clone()
	if _, err := clone.AcquireS3(context.Background(), clone, true); err != nil || calls != 1 {
		t.Fatal("callback not preserved")
	}
	for _, marshal := range []func(any) ([]byte, error){json.Marshal, yaml.Marshal} {
		encoded, err := marshal(&c)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "AcquireS3") || strings.Contains(string(encoded), "private-secret") {
			t.Fatal("runtime disclosure")
		}
	}
	encoded, err := json.Marshal(&RustFSS3Lease{AccessKey: "private-access", SecretKey: "private-secret"})
	if err != nil || string(encoded) != "{}" {
		t.Fatalf("lease disclosure: %s %v", encoded, err)
	}
}
