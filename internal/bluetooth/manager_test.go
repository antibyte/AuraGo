package bluetooth

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type fakeAdapter struct {
	status       AdapterStatus
	devices      []Device
	probeErr     error
	discoverErr  error
	paired       string
	connected    string
	disconnected string
}

func (f *fakeAdapter) Probe(context.Context) (AdapterStatus, error) {
	return f.status, f.probeErr
}

func (f *fakeAdapter) List(context.Context) ([]Device, error) {
	return append([]Device(nil), f.devices...), nil
}

func (f *fakeAdapter) Discover(context.Context, time.Duration) ([]Device, error) {
	return append([]Device(nil), f.devices...), f.discoverErr
}

func (f *fakeAdapter) Pair(_ context.Context, address, _ string) error {
	f.paired = address
	return nil
}

func (f *fakeAdapter) Connect(_ context.Context, address string) error {
	f.connected = address
	return nil
}

func (f *fakeAdapter) Disconnect(_ context.Context, address string) error {
	f.disconnected = address
	return nil
}

type fakeRunner struct {
	paths     map[string]bool
	outputs   map[string][]byte
	errs      map[string]error
	process   runningProcess
	processes []runningProcess
	starts    []string
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if f.paths[name] {
		return name, nil
	}
	return "", errors.New("not found")
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	key := name
	for _, arg := range args {
		key += " " + arg
	}
	return f.outputs[key], f.errs[key]
}

func (f *fakeRunner) Start(_ context.Context, backend, target, source string) (runningProcess, error) {
	f.starts = append(f.starts, backend+"|"+target+"|"+source)
	if len(f.processes) > 0 {
		process := f.processes[0]
		f.processes = f.processes[1:]
		return process, nil
	}
	return f.process, nil
}

type blockingProcess struct {
	done    chan error
	stopped chan struct{}
	once    sync.Once
}

func newBlockingProcess() *blockingProcess {
	return &blockingProcess{done: make(chan error, 1), stopped: make(chan struct{})}
}

func (p *blockingProcess) Wait() error {
	return <-p.done
}

func (p *blockingProcess) Stop() error {
	p.once.Do(func() {
		close(p.stopped)
		p.done <- context.Canceled
	})
	return nil
}

func newTestManager(adapter platformAdapter) *Manager {
	return &Manager{
		adapter: adapter,
		runner:  &fakeRunner{},
		logger:  slog.Default(),
		status: Status{
			Supported: true,
			Usable:    true,
			Audio:     AudioStatus{Usable: true, Backend: "pipewire"},
		},
		options: normalizeOptions(Options{Enabled: true, ScanTimeout: time.Second}),
	}
}

func TestNormalizeAddress(t *testing.T) {
	got, err := NormalizeAddress("aa-bb-cc-dd-ee-ff")
	if err != nil {
		t.Fatalf("NormalizeAddress returned error: %v", err)
	}
	if got != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("NormalizeAddress = %q", got)
	}
	if _, err := NormalizeAddress("not-an-address"); ErrorCode(err) != ErrorInvalidArgument {
		t.Fatalf("invalid address error code = %q, want %q", ErrorCode(err), ErrorInvalidArgument)
	}
}

func TestManagerPermissions(t *testing.T) {
	adapter := &fakeAdapter{}
	manager := newTestManager(adapter)
	manager.Configure(Options{Enabled: true, ReadOnly: true})
	if err := manager.Pair(context.Background(), "AA:BB:CC:DD:EE:FF", ""); ErrorCode(err) != ErrorReadOnly {
		t.Fatalf("Pair error = %v, want read-only", err)
	}
	if err := manager.Connect(context.Background(), "AA:BB:CC:DD:EE:FF"); ErrorCode(err) != ErrorReadOnly {
		t.Fatalf("Connect error = %v, want read-only", err)
	}
}

func TestResolveTargetSelection(t *testing.T) {
	audioUUID := "0000110b-0000-1000-8000-00805f9b34fb"
	adapter := &fakeAdapter{devices: []Device{
		{Address: "AA:BB:CC:DD:EE:01", Name: "Living Room", Connected: true, UUIDs: []string{audioUUID}},
		{Address: "AA:BB:CC:DD:EE:02", Name: "Kitchen", Connected: false, UUIDs: []string{audioUUID}},
	}}
	manager := newTestManager(adapter)

	got, err := manager.ResolveTarget(context.Background(), "")
	if err != nil {
		t.Fatalf("ResolveTarget returned error: %v", err)
	}
	if got.Address != "AA:BB:CC:DD:EE:01" {
		t.Fatalf("ResolveTarget address = %q", got.Address)
	}

	manager.Configure(Options{Enabled: true, DefaultDevice: "Kitchen"})
	got, err = manager.ResolveTarget(context.Background(), "")
	if err != nil {
		t.Fatalf("ResolveTarget default returned error: %v", err)
	}
	if got.Address != "AA:BB:CC:DD:EE:02" {
		t.Fatalf("ResolveTarget default address = %q", got.Address)
	}
}

func TestParseAudioTargetsByBluetoothAddress(t *testing.T) {
	pipeWire := []byte(`[
		{"type":"PipeWire:Interface:Node","info":{"props":{
			"media.class":"Audio/Sink",
			"node.name":"bluez_output.AA_BB_CC_DD_EE_FF.1",
			"api.bluez5.address":"AA:BB:CC:DD:EE:FF"
		}}}
	]`)
	target, err := parsePipeWireTarget(pipeWire, "AABBCCDDEEFF")
	if err != nil {
		t.Fatalf("parsePipeWireTarget: %v", err)
	}
	if target != "bluez_output.AA_BB_CC_DD_EE_FF.1" {
		t.Fatalf("PipeWire target = %q", target)
	}

	pulse := []byte(`[
		{"name":"bluez_output.AA_BB_CC_DD_EE_FF.1","description":"Speaker","properties":{"api.bluez5.address":"AA:BB:CC:DD:EE:FF"}}
	]`)
	target, err = parsePulseTarget(pulse, "AABBCCDDEEFF")
	if err != nil {
		t.Fatalf("parsePulseTarget: %v", err)
	}
	if target != "bluez_output.AA_BB_CC_DD_EE_FF.1" {
		t.Fatalf("Pulse target = %q", target)
	}
}

func TestProbeAudioBackendFallsBackToPulse(t *testing.T) {
	runner := &fakeRunner{
		paths: map[string]bool{"ffmpeg": true, "pactl": true},
		outputs: map[string][]byte{
			"ffmpeg -hide_banner -muxers": []byte(" E  pulse           Pulse audio output"),
			"pactl info":                  []byte("Server Name: PulseAudio"),
		},
		errs: map[string]error{},
	}
	status := probeAudioBackend(context.Background(), runner, "auto")
	if !status.Usable || status.Backend != "pulse" {
		t.Fatalf("probeAudioBackend = %+v", status)
	}
}

func TestPlaybackIsAsynchronousAndReplacesAuraGoStream(t *testing.T) {
	address := "AA:BB:CC:DD:EE:FF"
	audioUUID := "0000110b-0000-1000-8000-00805f9b34fb"
	adapter := &fakeAdapter{devices: []Device{{
		Address: address, Name: "Speaker", Paired: true, Connected: true, UUIDs: []string{audioUUID},
	}}}
	firstProcess := newBlockingProcess()
	secondProcess := newBlockingProcess()
	runner := &fakeRunner{
		outputs: map[string][]byte{
			"pw-dump": []byte(`[{"type":"PipeWire:Interface:Node","info":{"props":{"media.class":"Audio/Sink","node.name":"bluez_output.AA_BB_CC_DD_EE_FF.1","api.bluez5.address":"AA:BB:CC:DD:EE:FF"}}}]`),
		},
		errs:      map[string]error{},
		processes: []runningProcess{firstProcess, secondProcess},
	}
	manager := newTestManager(adapter)
	manager.runner = runner
	manager.Configure(Options{Enabled: true, AllowPlayback: true})

	dir := t.TempDir()
	firstSource := filepath.Join(dir, "first.mp3")
	secondSource := filepath.Join(dir, "second.mp3")
	if err := os.WriteFile(firstSource, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondSource, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := manager.Play(context.Background(), firstSource, address)
	if err != nil {
		t.Fatalf("first Play: %v", err)
	}
	second, err := manager.Play(context.Background(), secondSource, address)
	if err != nil {
		t.Fatalf("second Play: %v", err)
	}
	if first.ID == second.ID || second.State != "playing" {
		t.Fatalf("playback IDs/states = first %+v, second %+v", first, second)
	}
	select {
	case <-firstProcess.stopped:
	case <-time.After(time.Second):
		t.Fatal("previous playback was not stopped")
	}
	if err := manager.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	select {
	case <-secondProcess.stopped:
	case <-time.After(time.Second):
		t.Fatal("current playback was not stopped")
	}
	time.Sleep(10 * time.Millisecond)
	if got := manager.PlaybackStatus().State; got != "stopped" {
		t.Fatalf("playback state = %q, want stopped", got)
	}
}
