package render

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/launcher/flags"
)

// The render browser talks DevTools over a pipe (--remote-debugging-pipe), never over a port. A
// DevTools port listens on loopback, which every account of the host can reach: GET /json/list names
// the render URL (and with it the secret prefix of the listener) and the websocket drives the browser
// past the page CSP and the CDP guard. The pipe is inherited by the browser alone. rod's launcher
// only knows the websocket, so the browser is started here and handed to rod as a CDP client.

// browserStartTimeout bounds the wait for the first answer of a freshly started browser; browserExitWait
// is how long Close lets the browser quit by itself (it does when its input closes) before it is killed.
var (
	browserStartTimeout = 60 * time.Second
	browserExitWait     = 5 * time.Second
)

// pipeConn is the CDP transport over the pipe: JSON messages, each terminated by a NUL byte, in both
// directions. It implements cdp.WebSocketable.
type pipeConn struct {
	r      *bufio.Reader
	w      io.Writer
	mu     sync.Mutex
	closed atomic.Bool
}

func newPipeConn(r io.Reader, w io.Writer) *pipeConn {
	return &pipeConn{r: bufio.NewReaderSize(r, 64<<10), w: w}
}

// Read returns the next message without its terminator. Once it failed the connection is closed and
// every later Send fails too, so a call after the browser went away returns instead of waiting.
func (c *pipeConn) Read() ([]byte, error) {
	msg, err := c.r.ReadBytes(0)
	if err != nil {
		c.closed.Store(true)
		return nil, err
	}
	return msg[:len(msg)-1], nil
}

// Send writes one message and its terminator in a single write.
func (c *pipeConn) Send(data []byte) error {
	if c.closed.Load() {
		return io.ErrClosedPipe
	}
	frame := make([]byte, 0, len(data)+1)
	frame = append(append(frame, data...), 0)
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.w.Write(frame)
	return err
}

// browserArgs is the command line of one render browser: the flags of newLauncher, with the pipe in
// place of a DevTools port. The platform adds how the browser finds its end of the pipe.
func browserArgs(o launchOptions) []string {
	l := newLauncher(o)
	l.Delete(flags.RemoteDebuggingPort)
	l.Delete(flags.Leakless)
	l.Set("remote-debugging-pipe")
	return l.FormatArgs()
}

// browserProcess is a running render browser.
type browserProcess struct {
	cmd      *exec.Cmd
	in       *os.File // our end of the commands: the browser reads the other end
	out      *os.File // our end of the answers: the browser writes the other end
	exited   chan struct{}
	output   *tailBuffer // the last bytes the browser wrote: why it exited at start-up (no usable sandbox, missing libraries)
	profile  string
	once     sync.Once
	timedOut atomic.Bool
}

// PID is the process id of the browser.
func (p *browserProcess) PID() int { return p.cmd.Process.Pid }

// startBrowserProcess starts the browser with a DevTools pipe and connects rod to it. Nothing
// listens on any port. The caller closes the process (after browser.Close for a clean shutdown).
func startBrowserProcess(ctx context.Context, o launchOptions) (*rod.Browser, *browserProcess, error) {
	if o.Bin == "" {
		return nil, nil, errors.New("no browser executable")
	}
	inR, inW, err := os.Pipe() // the browser reads inR, we write inW
	if err != nil {
		return nil, nil, err
	}
	outR, outW, err := os.Pipe() // the browser writes outW, we read outR
	if err != nil {
		_ = inR.Close()
		_ = inW.Close()
		return nil, nil, err
	}
	closeAll := func() {
		for _, f := range []*os.File{inR, inW, outR, outW} {
			_ = f.Close()
		}
	}
	cmd := exec.Command(o.Bin, browserArgs(o)...)
	output := &tailBuffer{limit: 4096}
	cmd.Stdout, cmd.Stderr = output, output
	cmd.WaitDelay = 5 * time.Second // the browser's helper processes may hold stdout open
	if err := attachDevToolsPipe(cmd, inR, outW); err != nil {
		closeAll()
		return nil, nil, fmt.Errorf("prepare the browser pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		closeAll()
		return nil, nil, fmt.Errorf("start the browser: %w", err)
	}
	// The browser has its ends; ours must be the only other copies, so that it sees the end of its input
	// when we close ours.
	_ = inR.Close()
	_ = outW.Close()
	p := &browserProcess{cmd: cmd, in: inW, out: outR, exited: make(chan struct{}), output: output, profile: o.UserDataDir}
	go func() {
		_ = cmd.Wait()
		close(p.exited)
	}()

	client := cdp.New().Start(newPipeConn(outR, inW))
	browser := rod.New().Context(ctx).Client(client)
	timer := time.AfterFunc(browserStartTimeout, func() {
		p.timedOut.Store(true)
		killProcessTree(cmd.Process)
	})
	err = browser.Connect()
	timer.Stop()
	if err != nil {
		failure := p.startFailure(err)
		p.Close()
		return nil, nil, failure
	}
	return browser, p, nil
}

// startFailure says why the browser did not come up: what it wrote before it exited, or that it
// stayed silent.
func (p *browserProcess) startFailure(cause error) error {
	select {
	case <-p.exited:
	case <-time.After(2 * time.Second):
	}
	text := strings.TrimSpace(strings.ToValidUTF8(p.output.String(), "?"))
	switch {
	case p.timedOut.Load():
		return fmt.Errorf("the browser did not answer within %s", browserStartTimeout)
	case text != "":
		return fmt.Errorf("the browser exited at start-up: %s", text)
	}
	return fmt.Errorf("connect to the browser: %w", cause)
}

// Close shuts the browser down: it closes the commands pipe (a browser quits when its input ends),
// waits for it, kills what is left and removes the profile. It may be called more than once.
func (p *browserProcess) Close() {
	p.once.Do(func() {
		_ = p.in.Close()
		select {
		case <-p.exited:
		case <-time.After(browserExitWait):
			killProcessTree(p.cmd.Process)
			select {
			case <-p.exited:
			case <-time.After(5 * time.Second):
			}
		}
		_ = p.out.Close()
		if p.profile != "" {
			removeAllWithRetry(p.profile)
		}
	})
}

// removeAllWithRetry removes a folder the browser has just stopped using; on Windows its files can stay
// locked for a moment.
func removeAllWithRetry(dir string) {
	for i := 0; i < 20; i++ {
		if err := os.RemoveAll(dir); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
