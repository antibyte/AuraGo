package dockerhost

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data", "docker_manager.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

func TestStoreCRUD(t *testing.T) {
	ctx := context.Background()
	s, path := openTestStore(t)

	if hosts, err := s.List(ctx); err != nil || hosts == nil || len(hosts) != 0 {
		t.Fatalf("List on an empty store = (%v, %v), want an empty non-nil slice", hosts, err)
	}

	zeta := validTCPHost()
	zeta.Name, zeta.Sort = "zeta", 1
	alpha := validTCPHost()
	alpha.Name, alpha.Sort = "Alpha", 1
	first := validTCPHost()
	first.Name, first.Sort = "first", 0

	created := map[string]Host{}
	for _, h := range []Host{zeta, alpha, first} {
		got, err := s.Create(ctx, h)
		if err != nil {
			t.Fatalf("Create %s: %v", h.Name, err)
		}
		if !regexp.MustCompile(`^h_[0-9a-f]{12}$`).MatchString(got.ID) {
			t.Fatalf("id = %q, want h_ and 12 lowercase hex characters", got.ID)
		}
		if got.CreatedAt.IsZero() || !got.CreatedAt.Equal(got.UpdatedAt) {
			t.Fatalf("timestamps = %v / %v", got.CreatedAt, got.UpdatedAt)
		}
		created[h.Name] = got
	}

	hosts, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, h := range hosts {
		order = append(order, h.Name)
	}
	if want := "first,Alpha,zeta"; strings.Join(order, ",") != want {
		t.Fatalf("list order = %s, want %s (sort, then name ignoring case)", strings.Join(order, ","), want)
	}

	got, err := s.Get(ctx, created["zeta"].ID)
	if err != nil || got.Name != "zeta" || got.Address != "nas.lan" || got.Port != 2375 || got.Color != "teal" {
		t.Fatalf("Get = (%+v, %v)", got, err)
	}

	edit := got
	edit.Name, edit.Kind, edit.SSHUser, edit.Port = "zeta-ssh", KindSSH, "root", 2222
	updated, err := s.Update(ctx, edit)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated.CreatedAt.Equal(got.CreatedAt) || updated.UpdatedAt.Before(got.UpdatedAt) {
		t.Fatalf("Update timestamps: created %v -> %v, updated %v -> %v", got.CreatedAt, updated.CreatedAt, got.UpdatedAt, updated.UpdatedAt)
	}

	// The database survives a reopen.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	again, err := reopened.Get(ctx, got.ID)
	if err != nil || again.Name != "zeta-ssh" || again.Kind != KindSSH || again.SSHUser != "root" || again.Port != 2222 {
		t.Fatalf("after reopen = (%+v, %v)", again, err)
	}

	if err := reopened.Delete(ctx, got.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := reopened.Get(ctx, got.ID); !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("Get after Delete = %v, want ErrHostNotFound", err)
	}
	if err := reopened.Delete(ctx, got.ID); !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("second Delete = %v, want ErrHostNotFound", err)
	}
	if _, err := reopened.Update(ctx, edit); !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("Update of a deleted host = %v, want ErrHostNotFound", err)
	}
}

func TestStoreRejectsInvalidHostsAndDuplicateNames(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)

	bad := validTCPHost()
	bad.Address = "tcp://nas.lan"
	var ve *ValidationError
	if _, err := s.Create(ctx, bad); !errors.As(err, &ve) || ve.Field != "address" {
		t.Fatalf("Create with a scheme in the address = %v, want a ValidationError for address", err)
	}

	first, err := s.Create(ctx, validTCPHost())
	if err != nil {
		t.Fatal(err)
	}
	dup := validTCPHost()
	dup.Name = "nas" // same as "NAS" ignoring case
	if _, err := s.Create(ctx, dup); !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("Create with a duplicate name = %v, want a ValidationError for name", err)
	}

	other := validTCPHost()
	other.Name = "other"
	second, err := s.Create(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	clash := second
	clash.Name = "NAS"
	if _, err := s.Update(ctx, clash); !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("Update to a taken name = %v, want a ValidationError for name", err)
	}
	same := first
	same.Port = 2376
	if _, err := s.Update(ctx, same); err != nil {
		t.Fatalf("Update keeping the own name = %v, want success", err)
	}
}

// A concurrent writer can slip past the name check; the unique index then
// answers, and the store reports it like the check does.
func TestStoreMapsTheUniqueNameIndexToAValidationError(t *testing.T) {
	s, _ := openTestStore(t)
	if _, err := s.Create(context.Background(), validTCPHost()); err != nil {
		t.Fatal(err)
	}
	_, err := s.db.Exec(`INSERT INTO docker_hosts (id, name, kind, address, port, stacks_dir, host_socket_path, color, ssh_user, sort, created_at, updated_at)
		VALUES ('h_000000000000', 'nas', 'tcp', 'a', 0, '/a', '/b', 'teal', '', 0, '', '')`)
	if err == nil {
		t.Fatal("the unique name index let a duplicate name in")
	}
	var ve *ValidationError
	if mapped := storeError("store docker host", err); !errors.As(mapped, &ve) || ve.Field != "name" {
		t.Fatalf("storeError(%v) = %v, want a ValidationError for name", err, mapped)
	}
}
