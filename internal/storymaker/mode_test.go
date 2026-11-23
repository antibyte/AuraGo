package storymaker

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeModeAndEffectiveMode(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"", ModeCatalog}, {"catalog", ModeCatalog}, {"free", ModeFree}} {
		got, err := NormalizeMode(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("NormalizeMode(%q) = %q, %v, want %q", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{"Free", "code", " free", "templates"} {
		if _, err := NormalizeMode(bad); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("NormalizeMode(%q) error = %v, want ErrInvalidArgument", bad, err)
		}
	}
	for in, want := range map[string]string{"": ModeCatalog, "catalog": ModeCatalog, "free": ModeFree, "junk": ModeCatalog} {
		if got := EffectiveMode(in); got != want {
			t.Errorf("EffectiveMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateWithOptionsStoresTheModeOnlyWhenFree(t *testing.T) {
	ctx := context.Background()
	ps, _, _ := newTestStore(t)
	free, _, _, err := ps.CreateWithOptions(ctx, "Frei", "", 0, nil, CreateOptions{Mode: ModeFree})
	if err != nil || free.Mode != ModeFree {
		t.Fatalf("free project = %+v, %v", free, err)
	}
	got, err := ps.Get(ctx, free.ID)
	if err != nil || got.Mode != ModeFree {
		t.Fatalf("stored project = %+v, %v", got, err)
	}
	plain, _, _, err := ps.CreateWithOptions(ctx, "Vorlagen", "", 0, nil, CreateOptions{Mode: ModeCatalog})
	if err != nil || plain.Mode != "" {
		t.Fatalf("catalog project = %+v, %v", plain, err)
	}
	raw, err := encodeProject(plain)
	if err != nil || strings.Contains(string(raw), `"mode"`) {
		t.Fatalf("a catalog project must not write a mode key: %s %v", raw, err)
	}
	if _, _, _, err := ps.CreateWithOptions(ctx, "Falsch", "", 0, nil, CreateOptions{Mode: "junk"}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("junk mode error = %v, want ErrInvalidArgument", err)
	}
}
