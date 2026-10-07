//go:build linux

package networkshares

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type sambaAuditRunner struct {
	linuxFakeRunner
	names []string
}

func (r *sambaAuditRunner) Run(ctx context.Context, o Options, root bool, name string, args []string, input []byte) ([]byte, error) {
	if name == "net" && reflect.DeepEqual(args, []string{"conf", "listshares"}) {
		return []byte("global\r\nFamily Photos\r\n-leading name\n\n"), nil
	}
	if name == "net" && len(args) > 1 && args[1] == "showshare" {
		if len(args) != 4 || args[2] != "--" {
			return nil, fmt.Errorf("unprotected showshare")
		}
		r.names = append(r.names, args[3])
		return []byte("[" + args[3] + "]\npath = /srv/shared\nread only = yes\n"), nil
	}
	return r.linuxFakeRunner.Run(ctx, o, root, name, args, input)
}
func TestSambaFallbackKeepsNamesWithSpaces(t *testing.T) {
	runner := &sambaAuditRunner{linuxFakeRunner: linuxFakeRunner{missing: map[string]bool{"testparm": true}}}
	shares, err := (&linuxAdapter{runner: runner}).listSMB(context.Background(), Options{})
	if err != nil || len(shares) != 2 || !reflect.DeepEqual(runner.names, []string{"Family Photos", "-leading name"}) {
		t.Fatalf("shares=%v names=%v err=%v", shares, runner.names, err)
	}
}
func TestSambaMutationProtectsOptionLikeInputs(t *testing.T) {
	runner := &linuxFakeRunner{}
	adapter := &linuxAdapter{runner: runner}
	share := ShareSpec{Name: "--configfile=/tmp/evil", Path: "/srv/shared", Comment: "--option=include=/tmp/evil", ReadOnly: true}
	if err := adapter.smbAdd(context.Background(), Options{}, share); err != nil {
		t.Fatal(err)
	}
	if err := adapter.smbDeleteRaw(context.Background(), Options{}, share.Name); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, call := range runner.calls {
		if call.name != "net" {
			continue
		}
		if len(call.args) < 4 || call.args[0] != "conf" || call.args[2] != "--" || call.args[3] != share.Name {
			t.Fatalf("options may be interpreted: %+v", call)
		}
		seen[call.args[1]] = true
	}
	for _, action := range []string{"addshare", "setparm", "delparm", "delshare"} {
		if !seen[action] {
			t.Fatal("untested action", action)
		}
	}
}
