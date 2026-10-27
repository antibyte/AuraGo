package dockerstacks

import (
	"errors"
	"strings"
	"testing"

	"aurago/internal/dockerhost"
)

func TestValidateName(t *testing.T) {
	valid := []string{"a", "0", "web", "my-app", "my_app", "app2", "9lives", strings.Repeat("a", 63)}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}
	invalid := []string{
		"", "-app", "_app", "App", "my app", "my.app", "a/b", "..", "../x", "a\\b", "a\nb", "ä",
		strings.Repeat("a", 64),
	}
	for _, name := range invalid {
		err := ValidateName(name)
		if err == nil {
			t.Errorf("ValidateName(%q) = nil, want an error", name)
			continue
		}
		if CodeOf(err) != CodeStackNameInvalid {
			t.Errorf("CodeOf(ValidateName(%q)) = %q, want %q", name, CodeOf(err), CodeStackNameInvalid)
		}
	}
}

func TestRevisionIsTheHexSHA256(t *testing.T) {
	// SHA-256 of the empty input and of "abc" (FIPS 180-4 test vectors).
	if got := Revision(nil); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("Revision(nil) = %s", got)
	}
	if got := Revision([]byte("abc")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("Revision(abc) = %s", got)
	}
}

func TestCodeOfFindsEveryErrorKind(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{errors.New("plain"), ""},
		{newError(CodeStackNotFound, "no stack"), CodeStackNotFound},
		{&ConflictError{File: "compose"}, CodeRevisionConflict},
		{&BusyError{OperationID: "op_1", Kind: KindUp}, CodeOperationBusy},
		{errors.Join(errors.New("outer"), &BusyError{}), CodeOperationBusy},
	}
	for _, c := range cases {
		if got := CodeOf(c.err); got != c.want {
			t.Errorf("CodeOf(%v) = %q, want %q", c.err, got, c.want)
		}
	}
	exists := &Error{Code: CodeStackExists, Message: "taken", Err: ErrStackExists}
	if !errors.Is(exists, ErrStackExists) {
		t.Fatal("an *Error wrapping ErrStackExists must match errors.Is")
	}
}

func summaryOf(state, status, health string) dockerhost.ContainerSummary {
	return dockerhost.ContainerSummary{State: state, Status: status, Health: health}
}

func TestDeriveStatus(t *testing.T) {
	up := summaryOf("running", "Up 2 hours", "")
	upHealthy := summaryOf("running", "Up 2 hours (healthy)", "healthy")
	upStarting := summaryOf("running", "Up 5 seconds (health: starting)", "starting")
	unhealthy := summaryOf("running", "Up 2 hours (unhealthy)", "unhealthy")
	exited0 := summaryOf("exited", "Exited (0) 3 minutes ago", "")
	exited1 := summaryOf("exited", "Exited (1) 3 minutes ago", "")
	exited137 := summaryOf("exited", "Exited (137) 3 minutes ago", "")
	created := summaryOf("created", "Created", "")
	restarting := summaryOf("restarting", "Restarting (1) 5 seconds ago", "")
	dead := summaryOf("dead", "Dead", "")
	paused := summaryOf("paused", "Up 2 hours (Paused)", "")

	cases := []struct {
		name       string
		containers []dockerhost.ContainerSummary
		status     Status
		total, run int
	}{
		{"no containers is a draft", nil, StatusDraft, 0, 0},
		{"all running", []dockerhost.ContainerSummary{up, upHealthy}, StatusRunning, 2, 2},
		{"a health check that is still starting counts as running", []dockerhost.ContainerSummary{up, upStarting}, StatusRunning, 2, 2},
		{"one unhealthy makes it partial", []dockerhost.ContainerSummary{up, unhealthy}, StatusPartial, 2, 2},
		{"only unhealthy ones are still partial", []dockerhost.ContainerSummary{unhealthy}, StatusPartial, 1, 1},
		{"running plus exited is partial", []dockerhost.ContainerSummary{up, exited0}, StatusPartial, 2, 1},
		{"running plus restarting is partial", []dockerhost.ContainerSummary{up, restarting}, StatusPartial, 2, 1},
		{"running plus a crashed one is partial", []dockerhost.ContainerSummary{up, exited1}, StatusPartial, 2, 1},
		{"nothing running, clean exits is stopped", []dockerhost.ContainerSummary{exited0, exited0}, StatusStopped, 2, 0},
		{"created containers are stopped", []dockerhost.ContainerSummary{created}, StatusStopped, 1, 0},
		{"paused only is stopped", []dockerhost.ContainerSummary{paused}, StatusStopped, 1, 0},
		{"nothing running, one non-zero exit is an error", []dockerhost.ContainerSummary{exited0, exited1}, StatusError, 2, 0},
		{"a killed container is an error", []dockerhost.ContainerSummary{exited137}, StatusError, 1, 0},
		{"a restart loop is an error", []dockerhost.ContainerSummary{restarting}, StatusError, 1, 0},
		{"a dead container is an error", []dockerhost.ContainerSummary{dead, created}, StatusError, 2, 0},
	}
	for _, c := range cases {
		status, count := DeriveStatus(c.containers)
		if status != c.status || count.Total != c.total || count.Running != c.run {
			t.Errorf("%s: DeriveStatus = (%s, %+v), want (%s, total %d running %d)", c.name, status, count, c.status, c.total, c.run)
		}
	}
}

func TestPortLinksCollapseTheIPv4AndIPv6Binding(t *testing.T) {
	web := dockerhost.ContainerSummary{
		ComposeService: "web",
		Ports: []dockerhost.Port{
			{IP: "::", PrivatePort: 80, PublicPort: 8080, Type: "tcp"},
			{IP: "0.0.0.0", PrivatePort: 80, PublicPort: 8080, Type: "tcp"},
			{PrivatePort: 443, Type: "tcp"}, // exposed, not published
			{IP: "127.0.0.1", PrivatePort: 9000, PublicPort: 9000, Type: "tcp"},
		},
	}
	dns := dockerhost.ContainerSummary{
		ComposeService: "dns",
		Ports:          []dockerhost.Port{{IP: "0.0.0.0", PrivatePort: 53, PublicPort: 53, Type: "udp"}},
	}
	got := portLinks([]dockerhost.ContainerSummary{web, dns})
	want := []PortLink{
		{Service: "dns", PublicPort: 53, PrivatePort: 53, Protocol: "udp"},
		{Service: "web", PublicPort: 8080, PrivatePort: 80, Protocol: "tcp"},
		{Service: "web", IP: "127.0.0.1", PublicPort: 9000, PrivatePort: 9000, Protocol: "tcp"},
	}
	if len(got) != len(want) {
		t.Fatalf("portLinks = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("portLinks[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	if empty := portLinks(nil); empty == nil || len(empty) != 0 {
		t.Fatalf("portLinks(nil) = %v, want an empty non-nil slice", empty)
	}
}
