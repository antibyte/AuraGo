package security

import (
	"context"
	"errors"
	"os"
	"path"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
)

// egressStub replaces Chrome: egressStart returns an unconnected rod.Browser and
// a no-op stop function, egressProbe is the path check. No test below needs a
// real browser.
type egressStub struct {
	launchers []*launcher.Launcher
}

func stubEgressChrome(t *testing.T, probe func(checkURL string) (string, error)) *egressStub {
	t.Helper()
	s := &egressStub{}
	oldStart, oldProbe := egressStart, egressProbe
	egressStart = func(_ context.Context, l *launcher.Launcher) (*rod.Browser, func(), error) {
		s.launchers = append(s.launchers, l)
		return rod.New(), func() {}, nil
	}
	egressProbe = func(_ context.Context, _ *rod.Browser, checkURL string) (string, error) {
		return probe(checkURL)
	}
	t.Cleanup(func() { egressStart, egressProbe = oldStart, oldProbe })
	return s
}

// echoNonce is a path check that sees the nonce, as a real proxy path would.
func echoNonce(checkURL string) (string, error) { return path.Base(checkURL), nil }

func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatalf("%s did not happen within 3 s", what)
	}
}

func TestLaunchEgressBrowserReleasesTheSlotOnEveryFailure(t *testing.T) {
	cases := map[string]func(t *testing.T){
		"launch fails": func(t *testing.T) {
			stubEgressChrome(t, echoNonce)
			egressStart = func(context.Context, *launcher.Launcher) (*rod.Browser, func(), error) {
				return nil, nil, errors.New("forced launch failure")
			}
		},
		"path check fails": func(t *testing.T) {
			stubEgressChrome(t, func(string) (string, error) { return "", errors.New("forced check failure") })
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			setup(t)
			// More attempts than slots: a leaked slot would block or return ErrEgressBusy.
			for i := 0; i < egressMaxBrowsers+2; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				eb, err := LaunchEgressBrowser(ctx, EgressBrowserOptions{})
				cancel()
				if err == nil {
					eb.Close()
					t.Fatalf("attempt %d: want an error", i)
				}
				if errors.Is(err, ErrEgressBusy) {
					t.Fatalf("attempt %d: slot leaked: %v", i, err)
				}
			}
			if n := len(egressSlots); n != 0 {
				t.Fatalf("%d slots still held", n)
			}
		})
	}
}

func TestLaunchEgressBrowserWaitsForASlotAndGivesUpWithItsContext(t *testing.T) {
	stubEgressChrome(t, echoNonce)
	held := make([]*EgressBrowser, 0, egressMaxBrowsers)
	for i := 0; i < egressMaxBrowsers; i++ {
		eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{})
		if err != nil {
			t.Fatalf("launch %d: %v", i, err)
		}
		held = append(held, eb)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, err := LaunchEgressBrowser(ctx, EgressBrowserOptions{}); !errors.Is(err, ErrEgressBusy) {
		t.Fatalf("fifth launch = %v, want ErrEgressBusy", err)
	}
	held[0].Close()
	eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{})
	if err != nil {
		t.Fatalf("launch after a Close: %v", err)
	}
	held = append(held[1:], eb)
	for _, h := range held {
		h.Close()
	}
	if n := len(egressSlots); n != 0 {
		t.Fatalf("%d slots still held", n)
	}
}

func TestEgressBrowserReportsProxyDeath(t *testing.T) {
	stubEgressChrome(t, echoNonce)
	eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	defer eb.Close()
	_ = eb.proxy.Close()
	waitClosed(t, eb.Done, "Done after the proxy was closed")
	if err := eb.Err(); !errors.Is(err, ErrEgressProxyDied) {
		t.Fatalf("Err() = %v, want ErrEgressProxyDied", err)
	}
	if n := len(egressSlots); n != 0 {
		t.Fatalf("%d slots still held after the proxy died", n)
	}
	if got := eb.Wrap(errors.New("page failed")); !errors.Is(got, ErrEgressProxyDied) {
		t.Fatalf("Wrap = %v, want it to carry ErrEgressProxyDied", got)
	}
}

func TestLaunchEgressBrowserFailsClosedWhenTheCheckAnswersWithoutTheNonce(t *testing.T) {
	for name, answer := range map[string]string{"empty": "", "wrong": "not-the-nonce"} {
		t.Run(name, func(t *testing.T) {
			stubEgressChrome(t, func(string) (string, error) { return answer, nil })
			eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{})
			if eb != nil {
				eb.Close()
			}
			if !errors.Is(err, ErrEgressBypassed) {
				t.Fatalf("err = %v, want ErrEgressBypassed", err)
			}
		})
	}
}

func TestEgressBrowserEndsAfterItsLifetime(t *testing.T) {
	stubEgressChrome(t, echoNonce)
	eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{Lifetime: 60 * time.Millisecond})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	defer eb.Close()
	waitClosed(t, eb.Done, "Done after the lifetime")
	if err := eb.Err(); !errors.Is(err, ErrEgressLifetime) {
		t.Fatalf("Err() = %v, want ErrEgressLifetime", err)
	}
}

func TestEgressBrowserCloseIsCleanAndIdempotent(t *testing.T) {
	stubEgressChrome(t, echoNonce)
	eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	profile := eb.profile
	if _, err := os.Stat(profile); err != nil {
		t.Fatalf("profile directory missing before Close: %v", err)
	}
	if err := eb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_ = eb.Close()
	waitClosed(t, eb.Done, "Done after Close")
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Fatalf("profile directory not removed: %v", err)
	}
	if err := eb.Err(); err != nil {
		t.Fatalf("Err() after a plain Close = %v, want nil", err)
	}
}

func TestLaunchEgressBrowserSetsTheProxyFlags(t *testing.T) {
	stub := stubEgressChrome(t, echoNonce)
	eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{Sandbox: true})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	addr := eb.proxy.Addr()
	eb.Close()
	l := stub.launchers[0]
	if got, want := l.Get("proxy-server"), "http://"+addr; got != want {
		t.Fatalf("proxy-server = %q, want %q", got, want)
	}
	if got := l.Get("proxy-bypass-list"); got != "<-loopback>" {
		t.Fatalf("proxy-bypass-list = %q", got)
	}
	for _, flag := range []string{"force-webrtc-ip-handling-policy", "webrtc-ip-handling-policy"} {
		if got := l.Get(launcherFlag(flag)); got != "disable_non_proxied_udp" {
			t.Fatalf("%s = %q", flag, got)
		}
	}
	if l.Has("no-sandbox") {
		t.Fatal("Sandbox: true must remove --no-sandbox")
	}
}

func launcherFlag(name string) flags.Flag { return flags.Flag(name) }
