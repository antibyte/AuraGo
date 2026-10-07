package serialutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.bug.st/serial"
)

type fakePort struct {
	serial.Port
	closed  atomic.Int32
	onClose func()
	onBreak func(time.Duration) error
}

func (p *fakePort) Close() error {
	p.closed.Add(1)
	if p.onClose != nil {
		p.onClose()
	}
	return nil
}

func (p *fakePort) Break(duration time.Duration) error {
	if p.onBreak != nil {
		return p.onBreak(duration)
	}
	return nil
}

func TestOpenContextReservesBeforeOpenAndReleasesAfterClose(t *testing.T) {
	name := "serialutil-test-owner-order"
	var observedBusy atomic.Bool
	var observedReservation atomic.Bool
	port := &fakePort{onClose: func() {
		statuses, err := listWith(func() ([]string, error) { return []string{name}, nil })
		if err == nil && len(statuses) == 1 && statuses[0].Busy && statuses[0].Owner == "quick_connect" {
			observedBusy.Store(true)
		}
	}}
	lease, err := openContext(context.Background(), name, "quick_connect", nil,
		func() ([]string, error) { return []string{name}, nil },
		func(string, *serial.Mode) (serial.Port, error) {
			if _, err := reserve(name, "meshcore", []string{name}); errors.Is(err, ErrPortBusy) {
				observedReservation.Store(true)
			} else {
				return nil, errors.New("port was not reserved before open")
			}
			return port, nil
		})
	if err != nil {
		t.Fatalf("openContext: %v", err)
	}
	if !observedReservation.Load() {
		t.Fatal("port was not reserved before open")
	}
	statuses, err := listWith(func() ([]string, error) { return []string{name}, nil })
	if err != nil || len(statuses) != 1 || !statuses[0].Busy || statuses[0].Owner != "quick_connect" {
		t.Fatalf("port status = %#v, %v", statuses, err)
	}
	if err := lease.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if port.closed.Load() != 1 || !observedBusy.Load() {
		t.Fatalf("close count=%d reservation held during close=%v", port.closed.Load(), observedBusy.Load())
	}
	statuses, err = listWith(func() ([]string, error) { return []string{name}, nil })
	if err != nil || len(statuses) != 1 || statuses[0].Busy {
		t.Fatalf("port status after close = %#v, %v", statuses, err)
	}
}

func TestOpenContextRejectsUnenumeratedPathBeforeOpen(t *testing.T) {
	opened := false
	_, err := openContext(context.Background(), `C:\not-a-serial-device`, "quick_connect", nil,
		func() ([]string, error) { return []string{"COM17"}, nil },
		func(string, *serial.Mode) (serial.Port, error) { opened = true; return &fakePort{}, nil })
	if !errors.Is(err, ErrPortNotFound) || opened {
		t.Fatalf("err=%v opened=%v", err, opened)
	}
}

func TestOpenContextOpenFailureReleasesReservation(t *testing.T) {
	name := "serialutil-test-open-failure"
	_, err := openContext(context.Background(), name, "meshcore", nil,
		func() ([]string, error) { return []string{name}, nil },
		func(string, *serial.Mode) (serial.Port, error) { return nil, errors.New("driver failed") })
	if err == nil {
		t.Fatal("expected open failure")
	}
	lease, err := openContext(context.Background(), name, "quick_connect", nil,
		func() ([]string, error) { return []string{name}, nil },
		func(string, *serial.Mode) (serial.Port, error) { return &fakePort{}, nil })
	if err != nil {
		t.Fatalf("reservation retained after open failure: %v", err)
	}
	_ = lease.Close()
}

func TestOpenContextClosesLateOpenAfterCancellation(t *testing.T) {
	name := "serialutil-test-late-open"
	openStarted := make(chan struct{})
	finishOpen := make(chan struct{})
	portClosed := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := openContext(ctx, name, "quick_connect", nil,
			func() ([]string, error) { return []string{name}, nil },
			func(string, *serial.Mode) (serial.Port, error) {
				close(openStarted)
				<-finishOpen
				return &fakePort{onClose: func() { close(portClosed) }}, nil
			})
		result <- err
	}()
	<-openStarted
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("openContext error = %v", err)
	}
	statuses, err := listWith(func() ([]string, error) { return []string{name}, nil })
	if err != nil || len(statuses) != 1 || !statuses[0].Busy {
		t.Fatalf("reservation released before late open closed: %#v %v", statuses, err)
	}
	close(finishOpen)
	select {
	case <-portClosed:
	case <-time.After(time.Second):
		t.Fatal("late-opened port was not closed")
	}
	statuses, err = listWith(func() ([]string, error) { return []string{name}, nil })
	if err != nil || len(statuses) != 1 || statuses[0].Busy {
		t.Fatalf("reservation retained after late close: %#v %v", statuses, err)
	}
}

func TestConfiguredMeshCorePathSharesQuickConnectReservationAcrossAliases(t *testing.T) {
	var meshPath, qcPath string
	if runtime.GOOS == "windows" {
		meshPath, qcPath = `\\.\com7`, "COM7"
	} else {
		device := filepath.Join(t.TempDir(), "ttyUSB0")
		alias := filepath.Join(t.TempDir(), "by-id")
		if err := os.WriteFile(device, []byte("device"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(device, alias); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		meshPath, qcPath = alias, device
	}

	meshLease, err := openConfiguredContext(context.Background(), meshPath, OwnerMeshCore, nil,
		func(string, *serial.Mode) (serial.Port, error) { return &fakePort{}, nil })
	if err != nil {
		t.Fatalf("MeshCore configured alias open: %v", err)
	}
	opened := false
	_, err = openContext(context.Background(), qcPath, OwnerQuickConnectHost, nil,
		func() ([]string, error) { return []string{qcPath}, nil },
		func(string, *serial.Mode) (serial.Port, error) { opened = true; return &fakePort{}, nil })
	if !errors.Is(err, ErrPortBusy) || opened {
		t.Fatalf("Quick Connect alias conflict: err=%v opened=%v", err, opened)
	}
	statuses, err := listWith(func() ([]string, error) { return []string{qcPath}, nil })
	if err != nil || len(statuses) != 1 || !statuses[0].Busy || statuses[0].Owner != OwnerMeshCore {
		t.Fatalf("aliased MeshCore reservation status = %#v, %v", statuses, err)
	}
	if err := meshLease.Close(); err != nil {
		t.Fatalf("close MeshCore lease: %v", err)
	}
	qcLease, err := openContext(context.Background(), qcPath, OwnerQuickConnectHost, nil,
		func() ([]string, error) { return []string{qcPath}, nil },
		func(string, *serial.Mode) (serial.Port, error) { return &fakePort{}, nil })
	if err != nil {
		t.Fatalf("Quick Connect open after MeshCore close: %v", err)
	}
	_ = qcLease.Close()
}

func TestConcurrentOpenContextGrantsOneLease(t *testing.T) {
	const contenders = 12
	name := "serialutil-test-concurrent"
	start := make(chan struct{})
	results := make(chan *Lease, contenders)
	errorsOut := make(chan error, contenders)
	var opened atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			lease, err := openContext(context.Background(), name, OwnerQuickConnectHost, nil,
				func() ([]string, error) { return []string{name}, nil },
				func(string, *serial.Mode) (serial.Port, error) {
					opened.Add(1)
					return &fakePort{}, nil
				})
			if err != nil {
				errorsOut <- err
				return
			}
			results <- lease
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errorsOut)
	var leases []*Lease
	for lease := range results {
		leases = append(leases, lease)
	}
	var busy int
	for err := range errorsOut {
		if !errors.Is(err, ErrPortBusy) {
			t.Fatalf("unexpected open error: %v", err)
		}
		busy++
	}
	if len(leases) != 1 || busy != contenders-1 || opened.Load() != 1 {
		t.Fatalf("leases=%d busy=%d open calls=%d", len(leases), busy, opened.Load())
	}
	_ = leases[0].Close()
}

func TestWindowsPortKeyNormalizesCOMAliases(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows COM path normalization")
	}
	if got, want := portKey(`\\.\com12`), portKey("COM12"); got != want {
		t.Fatalf("COM alias keys differ: %q != %q", got, want)
	}
	if strings.Contains(portKey(`\\.\COM12`), `\\.\`) {
		t.Fatal("Windows device prefix was not normalized")
	}
}

func TestLeaseCancellationWaitsForBreakBeforeClosingAndReleasing(t *testing.T) {
	name := "serialutil-test-break-close-order"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	breakStarted := make(chan time.Duration, 1)
	finishBreak := make(chan struct{})
	deviceClosed := make(chan struct{})
	breakFinished := atomic.Bool{}
	closedAfterBreak := atomic.Bool{}
	port := &fakePort{
		onBreak: func(duration time.Duration) error {
			breakStarted <- duration
			<-finishBreak
			breakFinished.Store(true)
			return nil
		},
		onClose: func() {
			closedAfterBreak.Store(breakFinished.Load())
			close(deviceClosed)
		},
	}
	lease, err := openContext(ctx, name, OwnerQuickConnectHost, nil,
		func() ([]string, error) { return []string{name}, nil },
		func(string, *serial.Mode) (serial.Port, error) { return port, nil })
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	breakDone := make(chan error, 1)
	go func() { breakDone <- lease.Break(250 * time.Millisecond) }()
	if duration := <-breakStarted; duration != 250*time.Millisecond {
		t.Fatalf("break duration = %s", duration)
	}
	cancel()
	deadline := time.Now().Add(time.Second)
	for {
		lease.mu.Lock()
		closing := lease.closed
		lease.mu.Unlock()
		if closing {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("context cancellation did not start close")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case <-deviceClosed:
		t.Fatal("device closed while Break still used its handle")
	default:
	}
	statuses, err := listWith(func() ([]string, error) { return []string{name}, nil })
	if err != nil || len(statuses) != 1 || !statuses[0].Busy {
		t.Fatalf("reservation released before Break ended: %#v %v", statuses, err)
	}
	close(finishBreak)
	if err := <-breakDone; err != nil {
		t.Fatalf("Break: %v", err)
	}
	select {
	case <-deviceClosed:
	case <-time.After(time.Second):
		t.Fatal("device did not close after Break")
	}
	if !closedAfterBreak.Load() {
		t.Fatal("Close ran before Break finished")
	}
	statuses, err = listWith(func() ([]string, error) { return []string{name}, nil })
	if err != nil || len(statuses) != 1 || statuses[0].Busy {
		t.Fatalf("reservation retained after device close: %#v %v", statuses, err)
	}
	if err := lease.Break(time.Millisecond); !errors.Is(err, ErrPortClosed) {
		t.Fatalf("Break after close = %v", err)
	}
}
