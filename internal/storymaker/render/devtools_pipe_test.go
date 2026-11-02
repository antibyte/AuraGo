package render

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
	"github.com/shirou/gopsutil/v4/process"
)

// TestMain lets the test binary stand in for the browser: the tests below start it with
// SM_FAKE_BROWSER set, and it speaks the DevTools pipe protocol (or fails) instead of running tests.
func TestMain(m *testing.M) {
	if mode := os.Getenv("SM_FAKE_BROWSER"); mode != "" {
		os.Exit(runFakeBrowser(mode))
	}
	os.Exit(m.Run())
}

// fakePipes finds the browser's end of the DevTools pipe like Chromium does: file descriptors 3 and 4,
// or the handles named by --remote-debugging-io-pipes on Windows.
func fakePipes() (in, out *os.File) {
	if runtime.GOOS != "windows" {
		return os.NewFile(3, "devtools-in"), os.NewFile(4, "devtools-out")
	}
	for _, a := range os.Args {
		if v, ok := strings.CutPrefix(a, "--remote-debugging-io-pipes="); ok {
			parts := strings.Split(v, ",")
			if len(parts) == 2 {
				r, _ := strconv.ParseUint(parts[0], 10, 64)
				w, _ := strconv.ParseUint(parts[1], 10, 64)
				return os.NewFile(uintptr(r), "devtools-in"), os.NewFile(uintptr(w), "devtools-out")
			}
		}
	}
	return nil, nil
}

func runFakeBrowser(mode string) int {
	if mode == "sandbox" {
		fmt.Fprintln(os.Stderr, "[1:1:0101/000000.000:FATAL:zygote_host_impl_linux.cc(132)] No usable sandbox! Update your kernel.")
		return 1
	}
	in, out := fakePipes()
	if in == nil || out == nil {
		fmt.Fprintln(os.Stderr, "no devtools pipe")
		return 2
	}
	if mode == "silent" {
		time.Sleep(time.Hour)
		return 0
	}
	pipe, port := false, false
	for _, a := range os.Args {
		pipe = pipe || a == "--remote-debugging-pipe"
		port = port || strings.HasPrefix(a, "--remote-debugging-port")
	}
	r := bufio.NewReader(in)
	for {
		raw, err := r.ReadBytes(0)
		if err != nil {
			if mode == "stubborn" {
				time.Sleep(time.Hour) // ignores the end of its input
			}
			return 0
		}
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.Unmarshal(raw[:len(raw)-1], &req)
		result := map[string]any{}
		if req.Method == "Browser.getVersion" {
			result = map[string]any{"protocolVersion": "1.3", "product": fmt.Sprintf("Fake/1 pipe=%v port=%v", pipe, port), "revision": "r", "userAgent": "u", "jsVersion": "j"}
		}
		reply, _ := json.Marshal(map[string]any{"id": req.ID, "result": result})
		_, _ = out.Write(append(reply, 0))
		if req.Method == "Browser.close" && mode != "stubborn" {
			return 0
		}
	}
}

func TestPipeConnFramesMessagesWithNUL(t *testing.T) {
	c := newPipeConn(strings.NewReader("{\"a\":1}\x00{\"b\":[2]}\x00{\"cut"), io.Discard)
	for _, want := range []string{`{"a":1}`, `{"b":[2]}`} {
		got, err := c.Read()
		if err != nil || string(got) != want {
			t.Fatalf("Read = %q %v, want %q", got, err, want)
		}
	}
	if _, err := c.Read(); err == nil {
		t.Fatal("a message without its terminator must not be delivered")
	}
	if err := c.Send([]byte(`{"id":1}`)); err == nil {
		t.Fatal("a call after the end of the input must fail instead of waiting for an answer")
	}

	var sent bytes.Buffer
	c = newPipeConn(strings.NewReader(""), &sent)
	for _, msg := range []string{`{"id":1}`, `{"id":2}`} {
		if err := c.Send([]byte(msg)); err != nil {
			t.Fatal(err)
		}
	}
	if got := sent.String(); got != "{\"id\":1}\x00{\"id\":2}\x00" {
		t.Fatalf("sent %q", got)
	}
}

func TestBrowserArgsUseThePipeAndNoPort(t *testing.T) {
	args := strings.Join(browserArgs(launchOptions{Bin: "chrome", UserDataDir: t.TempDir(), ProxyAddr: "127.0.0.1:1", WindowSize: [2]int{640, 360}}), " ")
	if !strings.Contains(args, "--remote-debugging-pipe") || strings.Contains(args, "remote-debugging-port") {
		t.Fatalf("args = %s", args)
	}
	for _, want := range []string{"--headless", "--proxy-server=127.0.0.1:1", "--window-size=640,360", "--user-data-dir="} {
		if !strings.Contains(args, want) {
			t.Errorf("args lack %s: %s", want, args)
		}
	}
	if strings.Contains(args, "--no-sandbox") || strings.Contains(args, "rod-") {
		t.Errorf("args = %s", args)
	}
}

func fakeBrowser(t *testing.T, mode string) launchOptions {
	t.Helper()
	t.Setenv("SM_FAKE_BROWSER", mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return launchOptions{Bin: exe, UserDataDir: t.TempDir()}
}

func TestBrowserProcessSpeaksDevToolsOverThePipe(t *testing.T) {
	browser, proc, err := startBrowserProcess(context.Background(), fakeBrowser(t, "answer"))
	if err != nil {
		t.Fatal(err)
	}
	version, err := proto.BrowserGetVersion{}.Call(browser)
	if err != nil {
		t.Fatal(err)
	}
	if version.Product != "Fake/1 pipe=true port=false" {
		t.Fatalf("the browser was started with %q", version.Product)
	}
	pid := proc.PID()
	if err := browser.Close(); err != nil {
		t.Fatal(err)
	}
	proc.Close()
	proc.Close() // a second call is harmless
	select {
	case <-proc.exited:
	default:
		t.Fatal("Close returned while the browser was still running")
	}
	if exists, _ := process.PidExists(int32(pid)); exists {
		t.Fatalf("process %d is still there", pid)
	}
}

func TestBrowserProcessReportsWhyItExited(t *testing.T) {
	_, _, err := startBrowserProcess(context.Background(), fakeBrowser(t, "sandbox"))
	if err == nil || !strings.Contains(err.Error(), "No usable sandbox") {
		t.Fatalf("error = %v", err)
	}
	if issue := launchIssue(err); !strings.Contains(issue, "sandbox") || !strings.Contains(issue, "browser_path") {
		t.Fatalf("issue = %q", issue)
	}
	if _, _, err := startBrowserProcess(context.Background(), launchOptions{Bin: "", UserDataDir: t.TempDir()}); err == nil {
		t.Fatal("no executable must fail")
	}
	if _, _, err := startBrowserProcess(context.Background(), launchOptions{Bin: "/does/not/exist/chrome", UserDataDir: t.TempDir()}); err == nil {
		t.Fatal("a missing executable must fail")
	}
}

func TestBrowserProcessGivesUpOnASilentBrowser(t *testing.T) {
	old := browserStartTimeout
	browserStartTimeout = 400 * time.Millisecond
	t.Cleanup(func() { browserStartTimeout = old })
	start := time.Now()
	_, _, err := startBrowserProcess(context.Background(), fakeBrowser(t, "silent"))
	if err == nil || !strings.Contains(err.Error(), "did not answer") {
		t.Fatalf("error = %v", err)
	}
	if took := time.Since(start); took > 20*time.Second {
		t.Fatalf("a silent browser held the caller for %s", took)
	}
}

func TestBrowserProcessCloseKillsABrowserThatIgnoresTheEndOfItsInput(t *testing.T) {
	old := browserExitWait
	browserExitWait = 300 * time.Millisecond
	t.Cleanup(func() { browserExitWait = old })
	_, proc, err := startBrowserProcess(context.Background(), fakeBrowser(t, "stubborn"))
	if err != nil {
		t.Fatal(err)
	}
	pid := proc.PID()
	start := time.Now()
	proc.Close()
	if took := time.Since(start); took > 15*time.Second {
		t.Fatalf("Close took %s", took)
	}
	select {
	case <-proc.exited:
	default:
		t.Fatal("the browser was not killed")
	}
	if exists, _ := process.PidExists(int32(pid)); exists {
		t.Fatalf("process %d is still there", pid)
	}
}
