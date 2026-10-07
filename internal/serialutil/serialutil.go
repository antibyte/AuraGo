package serialutil

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

const (
	OwnerMeshCore         = "meshcore"
	OwnerQuickConnectHost = "quick_connect"
)

var (
	ErrInvalidPort  = errors.New("serial_port_invalid")
	ErrPortNotFound = errors.New("serial_port_not_found")
	ErrPortBusy     = errors.New("serial_port_busy")
	ErrPortClosed   = errors.New("serial_port_closed")
)

type PortStatus struct {
	Name  string `json:"name"`
	Busy  bool   `json:"busy"`
	Owner string `json:"owner"`
}

var reservations = struct {
	sync.Mutex
	byName map[string]string
}{byName: make(map[string]string)}

// List reports only names returned by the serial package's native enumerator.
func List() ([]PortStatus, error) {
	return listWith(serial.GetPortsList)
}

func listWith(enumerate func() ([]string, error)) ([]PortStatus, error) {
	names, err := enumerate()
	if err != nil {
		return nil, fmt.Errorf("enumerate serial ports: %w", err)
	}
	sort.Strings(names)
	reservations.Lock()
	defer reservations.Unlock()
	ports := make([]PortStatus, 0, len(names))
	for _, name := range names {
		owner := reservations.byName[portKey(name)]
		ports = append(ports, PortStatus{Name: name, Busy: owner != "", Owner: owner})
	}
	return ports, nil
}

// Lease owns an open serial port and its process-wide reservation. Close always
// closes the device before releasing the reservation.
type Lease struct {
	serial.Port
	reservation *reservation
	closeOnce   sync.Once
	controlMu   sync.Mutex
	mu          sync.Mutex
	stopContext func() bool
	closed      bool
	closeErr    error
}

type reservation struct {
	key  string
	once sync.Once
}

func reserve(name, owner string, names []string) (*reservation, error) {
	return reservePort(name, owner, names, true)
}

func reservePort(name, owner string, names []string, requireEnumerated bool) (*reservation, error) {
	if name == "" || owner == "" {
		return nil, ErrInvalidPort
	}
	if requireEnumerated {
		found := false
		for _, candidate := range names {
			if name == candidate {
				found = true
				break
			}
		}
		if !found {
			return nil, ErrPortNotFound
		}
	}
	key := portKey(name)
	if key == "" {
		return nil, ErrInvalidPort
	}
	reservations.Lock()
	defer reservations.Unlock()
	if reservations.byName[key] != "" {
		return nil, ErrPortBusy
	}
	reservations.byName[key] = owner
	return &reservation{key: key}, nil
}

// portKey maps serial aliases to one process-wide reservation identity.
func portKey(name string) string {
	if runtime.GOOS == "windows" {
		name = strings.ToUpper(strings.ReplaceAll(name, "/", `\`))
		name = strings.TrimPrefix(name, `\\.\`)
		name = strings.TrimPrefix(name, `\\?\`)
		return filepath.Clean(name)
	}
	if resolved, err := filepath.EvalSymlinks(name); err == nil {
		name = resolved
	}
	if absolute, err := filepath.Abs(name); err == nil {
		name = absolute
	}
	return filepath.Clean(name)
}

func (r *reservation) release() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		reservations.Lock()
		delete(reservations.byName, r.key)
		reservations.Unlock()
	})
}

func (l *Lease) bindContext(ctx context.Context) {
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	l.mu.Lock()
	l.stopContext = stop
	closed := l.closed
	l.mu.Unlock()
	if closed {
		stop()
	}
}

func (l *Lease) Close() error {
	l.closeOnce.Do(func() {
		l.mu.Lock()
		l.closed = true
		stop := l.stopContext
		l.mu.Unlock()
		if stop != nil {
			stop()
		}
		l.controlMu.Lock()
		l.closeErr = l.Port.Close()
		l.controlMu.Unlock()
		l.reservation.release()
	})
	return l.closeErr
}

func (l *Lease) SetDTR(value bool) error {
	return l.withControl(func(port serial.Port) error { return port.SetDTR(value) })
}

func (l *Lease) SetRTS(value bool) error {
	return l.withControl(func(port serial.Port) error { return port.SetRTS(value) })
}

func (l *Lease) Break(duration time.Duration) error {
	return l.withControl(func(port serial.Port) error { return port.Break(duration) })
}

func (l *Lease) withControl(action func(serial.Port) error) error {
	l.controlMu.Lock()
	defer l.controlMu.Unlock()
	l.mu.Lock()
	closed := l.closed
	l.mu.Unlock()
	if closed {
		return ErrPortClosed
	}
	return action(l.Port)
}

// OpenContext reserves an enumerated port before opening it. If cancellation
// happens while the driver is opening, a late successful open is closed before
// its reservation is released.
func OpenContext(ctx context.Context, name, owner string, mode *serial.Mode) (*Lease, error) {
	return openContext(ctx, name, owner, mode, serial.GetPortsList, serial.Open)
}

// OpenConfiguredContext opens a trusted MeshCore configuration path. MeshCore
// accepts aliases such as /dev/serial/by-id; aliases still share one lease key.
func OpenConfiguredContext(ctx context.Context, name, owner string, mode *serial.Mode) (*Lease, error) {
	if owner != OwnerMeshCore {
		return nil, ErrInvalidPort
	}
	return openConfiguredContext(ctx, name, owner, mode, serial.Open)
}

type openResult struct {
	lease *Lease
	err   error
}

func openContext(ctx context.Context, name, owner string, mode *serial.Mode, enumerate func() ([]string, error), open func(string, *serial.Mode) (serial.Port, error)) (*Lease, error) {
	return openContextWithPolicy(ctx, name, owner, mode, enumerate, open, true)
}

func openConfiguredContext(ctx context.Context, name, owner string, mode *serial.Mode, open func(string, *serial.Mode) (serial.Port, error)) (*Lease, error) {
	if owner != OwnerMeshCore {
		return nil, ErrInvalidPort
	}
	return openContextWithPolicy(ctx, name, owner, mode, nil, open, false)
}

func openContextWithPolicy(ctx context.Context, name, owner string, mode *serial.Mode, enumerate func() ([]string, error), open func(string, *serial.Mode) (serial.Port, error), requireEnumerated bool) (*Lease, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var names []string
	if requireEnumerated {
		var err error
		names, err = enumerate()
		if err != nil {
			return nil, fmt.Errorf("enumerate serial ports: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	r, err := reservePort(name, owner, names, requireEnumerated)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		r.release()
		return nil, err
	}
	result := make(chan openResult)
	go func() {
		port, openErr := open(name, mode)
		if openErr != nil || port == nil {
			r.release()
			if openErr == nil {
				openErr = errors.New("serial opener returned no port")
			}
			select {
			case result <- openResult{err: fmt.Errorf("open serial port: %w", openErr)}:
			case <-ctx.Done():
			}
			return
		}
		lease := &Lease{Port: port, reservation: r}
		lease.bindContext(ctx)
		if ctx.Err() != nil {
			_ = lease.Close()
			return
		}
		select {
		case result <- openResult{lease: lease}:
		case <-ctx.Done():
			_ = lease.Close()
		}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case opened := <-result:
		if err := ctx.Err(); err != nil {
			if opened.lease != nil {
				_ = opened.lease.Close()
			}
			return nil, err
		}
		return opened.lease, opened.err
	}
}
