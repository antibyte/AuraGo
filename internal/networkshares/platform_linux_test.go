//go:build linux

package networkshares

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type linuxRunnerCall struct {
	name string
	args []string
}

type linuxFakeRunner struct {
	missing map[string]bool
	calls   []linuxRunnerCall
}

func (r *linuxFakeRunner) LookPath(file string) (string, error) {
	if r.missing[file] {
		return "", errors.New("not installed")
	}
	return "/usr/bin/" + file, nil
}

func (r *linuxFakeRunner) Run(_ context.Context, _ Options, _ bool, name string, args []string, _ []byte) ([]byte, error) {
	r.calls = append(r.calls, linuxRunnerCall{name: name, args: append([]string(nil), args...)})
	key := name + " " + strings.Join(args, " ")
	switch key {
	case "net --version":
		return []byte("Version 4.20.0"), nil
	case "testparm -s --parameter-name=registry shares":
		return []byte("yes"), nil
	default:
		return nil, nil
	}
}

func TestLinuxSMBProbeCapabilityMatrix(t *testing.T) {
	runner := &linuxFakeRunner{}
	adapter := &linuxAdapter{runner: runner}
	status, err := adapter.Probe(context.Background(), Options{
		SMBEnabled: true,
		ReadOnly:   true,
	})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !status.SMB.Installed || !status.SMB.Configured || !status.SMB.ServiceActive || !status.SMB.Readable {
		t.Fatalf("SMB status = %+v", status.SMB)
	}
	if status.SMB.Writable {
		t.Fatal("read-only Linux probe unexpectedly reported write access")
	}

	dockerStatus, err := adapter.Probe(context.Background(), Options{
		SMBEnabled:  true,
		AllowCreate: true,
		IsDocker:    true,
	})
	if err != nil {
		t.Fatalf("Docker Probe: %v", err)
	}
	if !dockerStatus.SMB.Readable || dockerStatus.SMB.Writable || !strings.Contains(dockerStatus.SMB.Reason, "Docker") {
		t.Fatalf("Docker SMB status = %+v", dockerStatus.SMB)
	}

	missing := &linuxAdapter{runner: &linuxFakeRunner{missing: map[string]bool{"net": true, "testparm": true}}}
	missingStatus, err := missing.Probe(context.Background(), Options{SMBEnabled: true})
	if err != nil {
		t.Fatalf("missing Probe: %v", err)
	}
	if missingStatus.SMB.Installed || missingStatus.SMB.Readable || missingStatus.SMB.Reason == "" {
		t.Fatalf("missing-package status = %+v", missingStatus.SMB)
	}
}

func TestLinuxSambaUsesTypedArgumentsWithoutShellComposition(t *testing.T) {
	runner := &linuxFakeRunner{}
	adapter := &linuxAdapter{runner: runner}
	name := `media; touch /tmp/owned`
	path := `/srv/shares/media $(whoami)`
	err := adapter.smbAdd(context.Background(), Options{}, ShareSpec{
		Protocol: ProtocolSMB,
		Name:     name,
		Path:     path,
		Comment:  "test",
		ReadOnly: true,
		Access:   ShareAccess{ACL: []ACLEntry{{Principal: "media users", Level: "read"}}},
	})
	if err != nil {
		t.Fatalf("smbAdd: %v", err)
	}
	foundTypedCreate := false
	for _, call := range runner.calls {
		if call.name == "sh" || call.name == "bash" {
			t.Fatalf("unexpected shell command: %+v", call)
		}
		if call.name == "net" && len(call.args) >= 4 && call.args[0] == "conf" && call.args[1] == "addshare" {
			foundTypedCreate = call.args[2] == name && call.args[3] == path
		}
	}
	if !foundTypedCreate {
		t.Fatalf("typed Samba create call not found: %+v", runner.calls)
	}
}

func TestLinuxSMBPrincipalValidationUsesTypedGetentArguments(t *testing.T) {
	runner := &linuxFakeRunner{}
	adapter := &linuxAdapter{runner: runner}
	principal := `media users; id`
	err := adapter.Validate(context.Background(), Options{}, ShareSpec{
		Protocol: ProtocolSMB,
		Access:   ShareAccess{ACL: []ACLEntry{{Principal: principal, Level: "read"}}},
	})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("getent calls = %+v", runner.calls)
	}
	call := runner.calls[0]
	if call.name != "getent" || len(call.args) != 2 || call.args[0] != "passwd" || call.args[1] != principal {
		t.Fatalf("typed getent call = %+v", call)
	}
}

func TestLinuxSMBFullIsNormalizedWithoutAdminUsers(t *testing.T) {
	root := t.TempDir()
	normalized, err := validateShareSpec(context.Background(), ShareSpec{
		Protocol: ProtocolSMB,
		Name:     "documents",
		Path:     root,
		ReadOnly: false,
		Access:   ShareAccess{ACL: []ACLEntry{{Principal: "share-users", Level: "full"}}},
	}, "create", Options{
		SMBEnabled:           true,
		AllowedRoots:         []string{root},
		SMBAllowedPrincipals: []string{"share-users"},
	}, Status{SMB: ProtocolStatus{Readable: true}})
	if err != nil {
		t.Fatalf("validateShareSpec: %v", err)
	}
	if got := normalized.Access.ACL[0].Level; got != "change" {
		t.Fatalf("Linux full ACL normalized to %q, want change", got)
	}

	runner := &linuxFakeRunner{}
	adapter := &linuxAdapter{runner: runner}
	if err := adapter.applySMBParameters(context.Background(), Options{}, ShareSpec{
		Protocol: ProtocolSMB,
		Name:     "documents",
		Path:     root,
		ReadOnly: false,
		Access:   ShareAccess{ACL: []ACLEntry{{Principal: "share-users", Level: "full"}}},
	}); err != nil {
		t.Fatalf("applySMBParameters: %v", err)
	}
	foundWriteList := false
	for _, call := range runner.calls {
		joined := strings.Join(call.args, "\x00")
		if strings.Contains(joined, "\x00admin users\x00") &&
			len(call.args) > 1 && call.args[1] == "setparm" {
			t.Fatalf("AuraGo attempted to emit Samba admin users: %+v", call)
		}
		if strings.Contains(joined, "\x00write list\x00share-users") {
			foundWriteList = true
		}
	}
	if !foundWriteList {
		t.Fatalf("safe Samba write list was not emitted: %+v", runner.calls)
	}
}

func TestLinuxSMBObservationFlagsAdminUsersAsUnsafe(t *testing.T) {
	share, ok := observedSMBShare("documents", map[string]string{
		"path":        "/srv/shares/documents",
		"comment":     nativeComment("", "11111111-1111-4111-8111-111111111111"),
		"read only":   "no",
		"valid users": "share-users",
		"admin users": "share-users",
	})
	if !ok {
		t.Fatal("observedSMBShare rejected file share")
	}
	if !share.MarkerSupported || !share.UnsafeAdminUsers {
		t.Fatalf("unsafe Samba ownership evidence = %+v", share)
	}
}

func TestRenderNFSExportUsesRestrictedOptionsAndEscapesPath(t *testing.T) {
	content, err := renderNFSExport(ShareSpec{
		ID:       "11111111-1111-4111-8111-111111111111",
		Protocol: ProtocolNFS,
		Name:     "media",
		Path:     "/srv/shares/team media",
		ReadOnly: false,
		Access:   ShareAccess{Clients: []string{"192.0.2.0/24"}},
	})
	if err != nil {
		t.Fatalf("renderNFSExport: %v", err)
	}
	text := string(content)
	for _, wanted := range []string{`/srv/shares/team\040media`, "192.0.2.0/24(sync,root_squash,no_subtree_check,rw)"} {
		if !strings.Contains(text, wanted) {
			t.Fatalf("NFS export missing %q: %s", wanted, text)
		}
	}
	if strings.Contains(text, "no_root_squash") {
		t.Fatalf("unsafe NFS option present: %s", text)
	}
}

func TestNFSExportPathEncodingPreventsGrammarInjectionAndRoundTrips(t *testing.T) {
	path := "/srv/shares/team #one\\two\tline\r\nü"
	content, err := renderNFSExport(ShareSpec{
		ID:       "11111111-1111-4111-8111-111111111111",
		Protocol: ProtocolNFS,
		Name:     "media",
		Path:     path,
		ReadOnly: true,
		Access:   ShareAccess{Clients: []string{"192.0.2.1"}},
	})
	if err != nil {
		t.Fatalf("renderNFSExport: %v", err)
	}
	text := string(content)
	if strings.Count(text, "\n") != 2 || strings.Contains(text, "\r") || strings.Contains(text, "\t") {
		t.Fatalf("NFS export contains injected line/control syntax: %q", text)
	}
	lines := strings.Split(text, "\n")
	if strings.Contains(lines[1], "#") {
		t.Fatalf("NFS export data line contains an unescaped comment marker: %q", lines[1])
	}
	for _, escaped := range []string{`\040`, `\043`, `\134`, `\011`, `\015`, `\012`, `\303\274`} {
		if !strings.Contains(lines[1], escaped) {
			t.Fatalf("NFS export missing escaped byte %q: %q", escaped, lines[1])
		}
	}
	parsed := parseExportFS(lines[1])
	if _, ok := parsed[path]; !ok {
		t.Fatalf("encoded NFS path did not round-trip: got keys %+v, want %q", parsed, path)
	}
}

func TestLinuxNFSExportPublishesWithSameFilesystemRename(t *testing.T) {
	runner := &linuxFakeRunner{}
	adapter := &linuxAdapter{runner: runner}
	id := "11111111-1111-4111-8111-111111111111"
	err := adapter.writeNFSShare(context.Background(), Options{}, ShareSpec{
		ID:       id,
		Protocol: ProtocolNFS,
		Name:     "media",
		Path:     "/srv/shares/media",
		ReadOnly: true,
		Access:   ShareAccess{Clients: []string{"192.0.2.1"}},
	})
	if err != nil {
		t.Fatalf("writeNFSShare: %v", err)
	}
	if len(runner.calls) < 3 {
		t.Fatalf("NFS publish calls = %+v", runner.calls)
	}
	install := runner.calls[0]
	publish := runner.calls[1]
	if install.name != "install" || install.args[len(install.args)-1] != nfsStagingFile(id) {
		t.Fatalf("staging call = %+v", install)
	}
	if publish.name != "mv" || len(publish.args) < 4 ||
		publish.args[len(publish.args)-2] != nfsStagingFile(id) ||
		publish.args[len(publish.args)-1] != nfsShareFile(id) {
		t.Fatalf("atomic publish call = %+v", publish)
	}
}

func TestLinuxSambaPrinterSharesAreNotListed(t *testing.T) {
	adapter := &linuxAdapter{runner: &linuxScriptedListRunner{}}
	shares, err := adapter.listSMB(context.Background(), Options{})
	if err != nil {
		t.Fatalf("listSMB: %v", err)
	}
	if len(shares) != 1 || shares[0].Name != "files" {
		t.Fatalf("file shares = %+v", shares)
	}
}

type linuxScriptedListRunner struct{}

func (*linuxScriptedListRunner) LookPath(file string) (string, error) {
	return "/usr/bin/" + file, nil
}

func (*linuxScriptedListRunner) Run(_ context.Context, _ Options, _ bool, name string, args []string, _ []byte) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	switch key {
	case "net conf listshares":
		return []byte("registry-files\n"), nil
	case "net conf showshare registry-files":
		return []byte("[registry-files]\npath = /srv/shares/registry\nread only = yes\n"), nil
	case "testparm -s --suppress-prompt":
		return []byte("[global]\nregistry shares = yes\n" +
			"[files]\npath = /srv/shares/files\nread only = yes\n" +
			"[printers]\npath = /var/spool/samba\nprintable = yes\n" +
			"[admin$]\npath = /srv/shares/admin\n"), nil
	default:
		return nil, nil
	}
}
