//go:build linux

package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNativeDesktopArgumentsAreBoundedAndTextUsesStdin(t *testing.T) {
	const text = "hello; $(touch /tmp/not-executed)\n--force"
	args, input, err := desktopArguments(desktopRequest{Operation: "type", ElementRef: "@s123:e4", Text: text, TimeoutMS: 500})
	if err != nil {
		t.Fatal(err)
	}
	if input != text || strings.Contains(strings.Join(args, " "), text) || !reflect.DeepEqual(args, []string{"type", "@s123:e4", "--stdin", "--timeout-ms", "500"}) {
		t.Fatalf("unsafe argument delivery: %v", args)
	}
	for _, request := range []desktopRequest{{Operation: "sandbox"}, {Operation: "launch", App: "--force"}, {Operation: "launch", App: "/bin/sh"}, {Operation: "click", ElementRef: "@e1"}, {Operation: "type", ElementRef: "@s1:e1", Text: strings.Repeat("x", 10001)}, {Operation: "scroll", DeltaY: 101}, {Operation: "press", WindowID: "w1", Key: "--force"}} {
		if _, _, err = desktopArguments(request); err == nil {
			t.Fatalf("invalid request accepted: %s", request.Operation)
		}
	}
	args, _, err = desktopArguments(desktopRequest{Operation: "wait", ElementRef: "@s1:e1", Predicate: "enabled", TimeoutMS: 400})
	if err != nil || !reflect.DeepEqual(args, []string{"wait", "--element", "@s1:e1", "--predicate", "enabled", "--timeout", "400"}) {
		t.Fatalf("v1.3.1 wait contract: %v %v", args, err)
	}
	args, _, err = desktopArguments(desktopRequest{Operation: "list_surfaces", App: "mousepad"})
	if err != nil || !reflect.DeepEqual(args, []string{"list-surfaces", "--app", "mousepad"}) {
		t.Fatalf("v1.3.1 surfaces contract: %v %v", args, err)
	}
}

func TestNativeDesktopPreservesUpstreamDispositionAndInvalidatesRefs(t *testing.T) {
	n := nativeDesktop{run: func(_ context.Context, args []string, _ string) (desktopEnvelope, error) {
		if args[0] == "snapshot" {
			return desktopEnvelope{OK: true, Data: json.RawMessage(`{"snapshot_id":"s123","complete":false,"tree":[]}`)}, nil
		}
		return desktopEnvelope{OK: false, Error: json.RawMessage(`{"code":"STALE_REF","disposition":{"delivery":"not_delivered","retry":"safe"}}`)}, nil
	}}
	if _, err := n.perform(context.Background(), desktopRequest{Operation: "snapshot"}); err != nil {
		t.Fatal(err)
	}
	r, err := n.perform(context.Background(), desktopRequest{Operation: "click", ElementRef: "@s123:e4"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(r.(desktopEnvelope).Error), `"retry":"safe"`) {
		t.Fatal("upstream error lost")
	}
	n.reset()
	if _, err = n.perform(context.Background(), desktopRequest{Operation: "click", ElementRef: "@s123:e4"}); err == nil {
		t.Fatal("reference survived control change")
	}
}

func TestNativeDesktopOutputIsBounded(t *testing.T) {
	var b cappedDesktopOutput
	p := []byte(strings.Repeat("a", nativeOutputLimit+1))
	n, err := b.Write(p)
	if err != nil || n != len(p) || !b.overflow || b.Len() != nativeOutputLimit {
		t.Fatal("output cap failed")
	}
}

func TestNativeDesktopProcessHelper(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "native-helper" {
		return
	}
	switch os.Args[len(os.Args)-1] {
	case "stdin":
		data, _ := io.ReadAll(os.Stdin)
		_ = json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"ok": true, "data": string(data)})
	case "error":
		_, _ = os.Stdout.WriteString(`{"ok":false,"error":{"code":"AMBIGUOUS_TARGET","disposition":{"delivery":"not_delivered","retry":"safe"}}}`)
		os.Exit(17)
	case "overflow":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", nativeOutputLimit+1))
	case "sleep":
		time.Sleep(time.Minute)
	}
	os.Exit(0)
}

func TestNativeDesktopProcessStdinErrorsLimitsAndCancellation(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"stdin", "error", "overflow", "sleep"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			result, err := runDesktopProcess(ctx, executable, []string{"-test.run=^TestNativeDesktopProcessHelper$", "native-helper", mode}, "literal $()\n--force")
			switch mode {
			case "stdin":
				var input string
				_ = json.Unmarshal(result.Data, &input)
				if err != nil || !result.OK || input != "literal $()\n--force" {
					t.Fatalf("stdin changed: %v", err)
				}
			case "error":
				if err != nil || !strings.Contains(string(result.Error), "AMBIGUOUS_TARGET") {
					t.Fatalf("upstream disposition lost: %v", err)
				}
			default:
				if err == nil {
					t.Fatal("unbounded or uncancelled process")
				}
				rpc, ok := err.(*rpcError)
				if !ok || rpc.Data["disposition"].(map[string]string)["retry"] != "unsafe" {
					t.Fatal("unknown delivery is replayable")
				}
			}
		})
	}
}

func TestNativeDesktopImageBindsWindowGeometryInstanceAndAge(t *testing.T) {
	for _, change := range []string{"valid", "expired", "window", "geometry", "instance", "resolution", "unfocused", "outside", "drag_outside", "ambiguous", "input_invalidates"} {
		t.Run(change, func(t *testing.T) {
			w := desktopWindow{ID: "w1", Instance: "pid-start", Bounds: desktopBounds{X: 10, Y: 10, Width: 200, Height: 100}, Focused: true, Visible: true}
			image := desktopImage{ID: "image", Window: w, Created: time.Now(), Width: 1280, Height: 800, Committed: true}
			windows := []desktopWindow{w}
			width := 1280
			r := desktopRequest{Operation: "click_xy", WindowID: w.ID, ImageRef: image.ID, X: 20, Y: 20}
			switch change {
			case "expired":
				image.Created = time.Now().Add(-31 * time.Second)
			case "window":
				r.WindowID = "other"
			case "geometry":
				windows[0].Bounds.X++
			case "instance":
				windows[0].Instance = "new-start"
			case "resolution":
				width = 1024
			case "unfocused":
				windows[0].Focused = false
			case "outside":
				r.X = 9
			case "drag_outside":
				r.Operation = "drag"
				r.ToX = 500
				r.ToY = 20
			case "ambiguous":
				windows = append(windows, w)
			}
			inputs := 0
			n := nativeDesktop{images: map[string]desktopImage{image.ID: image}, screenSize: func(context.Context) (int, int, error) { return width, 800, nil }, run: func(_ context.Context, args []string, _ string) (desktopEnvelope, error) {
				if args[0] == "list-windows" {
					data, _ := json.Marshal(windows)
					return desktopEnvelope{OK: true, Data: data}, nil
				}
				inputs++
				return desktopEnvelope{OK: true}, nil
			}}
			if change == "input_invalidates" {
				n.invalidate()
			}
			_, err := n.perform(context.Background(), r)
			if change == "valid" {
				if err != nil || inputs != 1 {
					t.Fatalf("valid action rejected: %v", err)
				}
				if len(n.images) != 0 {
					t.Fatal("input retained image")
				}
			} else if err == nil || inputs != 0 {
				t.Fatalf("unsafe action admitted: %s", change)
			}
		})
	}
}

func TestNativeDesktopControlCancelsAndDrains(t *testing.T) {
	var gate uiControl
	ctx, finish, err := gate.enter(context.Background(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- gate.change(context.Background(), "human", true) }()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("input was not cancelled")
	}
	select {
	case <-done:
		t.Fatal("control acknowledged before drain")
	default:
	}
	finish()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if _, _, err = gate.enter(context.Background(), "", true); err == nil {
		t.Fatal("late old request admitted")
	}
	if _, _, err = gate.enter(context.Background(), "human", true); err == nil {
		t.Fatal("human control admitted input")
	}
	_, finish, err = gate.enter(context.Background(), "human", false)
	if err != nil {
		t.Fatal(err)
	}
	finish()
}

//go:embed install-native-desktop.sh
var nativeInstallerTestScript string

func TestNativeDesktopInstallerRejectsCorruptArchives(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		for _, damage := range []string{"size", "hash"} {
			t.Run(arch+"_"+damage, func(t *testing.T) {
				dir := t.TempDir()
				scripts := map[string]string{
					"dpkg":       "if [ \"$1\" = --print-architecture ]; then echo \"$TEST_ARCH\"; else /usr/bin/dpkg \"$@\"; fi",
					"dpkg-query": "echo 2.56.2",
					"curl":       "while [ \"$1\" != -o ]; do shift; done; shift; size=3537152; [ \"$TEST_ARCH\" != arm64 ] || size=3407558; [ \"$TEST_DAMAGE\" != size ] || size=10; truncate -s \"$size\" \"$1\"",
					"tar":        "touch \"$TEST_UNPACKED\"; exit 99",
				}
				for name, body := range scripts {
					if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
						t.Fatal(err)
					}
				}
				script := filepath.Join(dir, "installer")
				if err := os.WriteFile(script, []byte(nativeInstallerTestScript), 0700); err != nil {
					t.Fatal(err)
				}
				marker := filepath.Join(dir, "unpacked")
				cmd := exec.Command("/bin/sh", script)
				cmd.Env = []string{"PATH=" + dir + ":/usr/bin:/bin", "TEST_ARCH=" + arch, "TEST_DAMAGE=" + damage, "TEST_UNPACKED=" + marker}
				output, err := cmd.CombinedOutput()
				if err == nil {
					t.Fatal("corrupt archive accepted")
				}
				if damage == "hash" && !strings.Contains(string(output), "FAILED") {
					t.Fatalf("hash verification did not run: %s", output)
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatal("corrupt archive reached extraction")
				}
			})
		}
	}
}
