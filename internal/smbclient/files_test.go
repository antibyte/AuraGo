package smbclient

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestWriteMethodsRejectBadPathsBeforeTouchingTheShare(t *testing.T) {
	c := &Client{} // nil share: any call that passed validation would panic
	ctx := context.Background()
	bad := []string{"", ".", "..", "../x", "/abs", `a\b`, "C:x", "a/../../b", "a:stream"}
	for _, p := range bad {
		if err := c.Mkdir(ctx, p); err == nil {
			t.Errorf("Mkdir(%q) accepted", p)
		}
		if err := c.Remove(ctx, p); err == nil {
			t.Errorf("Remove(%q) accepted", p)
		}
		if err := c.Rename(ctx, p, "ok"); err == nil {
			t.Errorf("Rename(%q, ok) accepted", p)
		}
		if err := c.Rename(ctx, "ok", p); err == nil {
			t.Errorf("Rename(ok, %q) accepted", p)
		}
		if _, err := c.WriteFrom(ctx, p, strings.NewReader("x")); err == nil {
			t.Errorf("WriteFrom(%q) accepted", p)
		}
		if _, _, err := c.OpenFile(ctx, p); err == nil {
			t.Errorf("OpenFile(%q) accepted", p)
		}
	}
}

func TestConfigDefaults(t *testing.T) {
	c := &Client{}
	if c.listLimit() != 10000 {
		t.Fatalf("default list limit %d", c.listLimit())
	}
	c.maxList = 50000
	if c.listLimit() != 50000 {
		t.Fatalf("custom list limit %d", c.listLimit())
	}
	cfg := Config{Host: "h", Share: "s", Username: "u", Password: "p", Port: 70000}
	if _, err := Dial(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "port") {
		t.Fatalf("expected port error, got %v", err)
	}
}

func TestIsLogonFailure(t *testing.T) {
	if IsLogonFailure(nil) {
		t.Fatal("nil is not a logon failure")
	}
	if !IsLogonFailure(errors.New("connect: response error: STATUS_LOGON_FAILURE")) {
		t.Fatal("text fallback missed")
	}
	if IsLogonFailure(errors.New("access denied")) {
		t.Fatal("false positive")
	}
}
