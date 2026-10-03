package bluetooth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testHeadsetAddress = "AA:BB:CC:DD:EE:FF"
	handsfreeUUID      = "0000111e-0000-1000-8000-00805f9b34fb"
	headsetUUID        = "00001108-0000-1000-8000-00805f9b34fb"
	a2dpSinkUUID       = "0000110b-0000-1000-8000-00805f9b34fb"
)

func TestHasMicrophoneRecognizesHandsfreeAndHeadset(t *testing.T) {
	for _, tc := range []struct {
		uuids []string
		want  bool
	}{
		{[]string{a2dpSinkUUID}, false},
		{[]string{a2dpSinkUUID, handsfreeUUID}, true},
		{[]string{"00001108-0000-1000-8000-00805F9B34FB"}, true},
		// Handsfree Audio Gateway: the remote side is a phone, not a headset.
		{[]string{"0000111f-0000-1000-8000-00805f9b34fb"}, false},
	} {
		if got := hasMicrophone(Device{UUIDs: tc.uuids}); got != tc.want {
			t.Fatalf("hasMicrophone(%v) = %v, want %v", tc.uuids, got, tc.want)
		}
	}
	device := deviceFromProperties(map[string]interface{}{
		"Address": "aa:bb:cc:dd:ee:ff", "Paired": true,
		"UUIDs": []string{a2dpSinkUUID, headsetUUID},
	}, nil)
	if !device.Microphone || !device.Audio {
		t.Fatalf("device = %+v, want audio with microphone", device)
	}
}

// headsetDump renders a pw-dump snapshot of the test headset in one profile.
// Only the HFP profiles have a microphone (bluez_input) node.
func headsetDump(profile string) []byte {
	indexes := map[string]int{"off": 0, "a2dp-sink": 1, "headset-head-unit-cvsd": 2, "headset-head-unit": 3, "headset-head-unit-msbc": 4}
	nodes := ""
	switch {
	case strings.HasPrefix(profile, "headset-head-unit"):
		nodes = `,
	{"id": 80, "type": "PipeWire:Interface:Node", "info": {"props": {"media.class": "Audio/Source", "node.name": "bluez_input.AA_BB_CC_DD_EE_FF.0", "api.bluez5.address": "AA:BB:CC:DD:EE:FF"}}},
	{"id": 81, "type": "PipeWire:Interface:Node", "info": {"props": {"media.class": "Audio/Sink", "node.name": "bluez_output.AA_BB_CC_DD_EE_FF.0", "api.bluez5.address": "AA:BB:CC:DD:EE:FF"}}}`
	case profile == "a2dp-sink":
		nodes = `,
	{"id": 82, "type": "PipeWire:Interface:Node", "info": {"props": {"media.class": "Audio/Sink", "node.name": "bluez_output.AA_BB_CC_DD_EE_FF.1", "api.bluez5.address": "AA:BB:CC:DD:EE:FF"}}}`
	}
	return []byte(fmt.Sprintf(`[
	{"id": 50, "type": "PipeWire:Interface:Node", "info": {"props": {"media.class": "Audio/Sink", "node.name": "alsa_output.pci"}}},
	{"id": 60, "type": "PipeWire:Interface:Device", "info": {"props": {"device.api": "bluez5", "api.bluez5.address": "11:22:33:44:55:66"}, "params": {"EnumProfile": [{"index": 2, "name": "headset-head-unit", "priority": 9, "available": "yes"}], "Profile": [{"index": 2, "name": "headset-head-unit"}]}}},
	{"id": 71, "type": "PipeWire:Interface:Device", "info": {"props": {"device.api": "bluez5", "device.name": "bluez_card.AA_BB_CC_DD_EE_FF", "api.bluez5.address": "AA:BB:CC:DD:EE:FF"}, "params": {
		"EnumProfile": [
			{"index": 0, "name": "off", "priority": 0, "available": "yes"},
			{"index": 1, "name": "a2dp-sink", "priority": 16, "available": "yes"},
			{"index": 2, "name": "headset-head-unit-cvsd", "priority": 1, "available": "yes"},
			{"index": 3, "name": "headset-head-unit", "priority": 2, "available": "yes"},
			{"index": 4, "name": "headset-head-unit-msbc", "priority": 3, "available": "no"}
		],
		"Profile": [{"index": %d, "name": %q}]
	}}}%s
]`, indexes[profile], profile, nodes))
}

const noMicrophoneDump = `[
	{"id": 71, "type": "PipeWire:Interface:Device", "info": {"props": {"device.api": "bluez5", "api.bluez5.address": "AA:BB:CC:DD:EE:FF"}, "params": {
		"EnumProfile": [{"index": 0, "name": "off", "priority": 0, "available": "yes"}, {"index": 1, "name": "a2dp-sink", "priority": 16, "available": "yes"}],
		"Profile": [{"index": 1, "name": "a2dp-sink"}]}}}
]`

func TestParseHeadsetGraphFindsDeviceProfilesAndNodes(t *testing.T) {
	graph, found, err := parseHeadsetGraph(headsetDump("headset-head-unit"), "aa:bb:cc:dd:ee:ff")
	if err != nil || !found {
		t.Fatalf("parseHeadsetGraph found=%v err=%v", found, err)
	}
	if graph.DeviceID != 71 || graph.Current.Name != "headset-head-unit" {
		t.Fatalf("graph = %+v", graph)
	}
	if graph.Source != "bluez_input.AA_BB_CC_DD_EE_FF.0" || graph.Sink != "bluez_output.AA_BB_CC_DD_EE_FF.0" {
		t.Fatalf("nodes = %q %q", graph.Source, graph.Sink)
	}
	// mSBC is unavailable here, so the next best HFP profile wins over CVSD.
	if best, ok := graph.bestHeadsetProfile(); !ok || best.Index != 3 {
		t.Fatalf("best profile = %+v ok=%v", best, ok)
	}
	if profile, ok := graph.profileByName("a2dp-sink"); !ok || profile.Index != 1 {
		t.Fatalf("a2dp-sink = %+v ok=%v", profile, ok)
	}

	a2dp, found, err := parseHeadsetGraph(headsetDump("a2dp-sink"), testHeadsetAddress)
	if err != nil || !found || a2dp.Source != "" || a2dp.Sink != "bluez_output.AA_BB_CC_DD_EE_FF.1" || isHeadsetProfile(a2dp.Current.Name) {
		t.Fatalf("a2dp graph = %+v found=%v err=%v", a2dp, found, err)
	}
	if _, found, _ := parseHeadsetGraph(headsetDump("a2dp-sink"), "AA:BB:CC:DD:EE:00"); found {
		t.Fatal("another address must not match")
	}
	if graph, _, _ := parseHeadsetGraph([]byte(noMicrophoneDump), testHeadsetAddress); func() bool { _, ok := graph.bestHeadsetProfile(); return ok }() {
		t.Fatal("a device without HFP profiles has no headset profile")
	}
	if _, _, err := parseHeadsetGraph([]byte("not json"), testHeadsetAddress); err == nil {
		t.Fatal("invalid JSON must fail")
	}
}

// fakeHeadsetRunner models PipeWire: pw-dump reports the current profile and
// wpctl set-profile changes it.
type fakeHeadsetRunner struct {
	mu        sync.Mutex
	profile   string
	dump      func(profile string) []byte
	calls     []string
	pipes     []*fakeHeadsetProcess
	blockPlay bool
	// profileNames maps wpctl profile indexes to names; nil uses headsetDump's.
	profileNames map[string]string
}

func newFakeHeadsetRunner(profile string) *fakeHeadsetRunner {
	return &fakeHeadsetRunner{profile: profile, dump: headsetDump}
}

func (f *fakeHeadsetRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	switch name {
	case "pw-dump":
		return f.dump(f.profile), nil
	case "wpctl":
		names := f.profileNames
		if names == nil {
			names = map[string]string{"0": "off", "1": "a2dp-sink", "2": "headset-head-unit-cvsd", "3": "headset-head-unit", "4": "headset-head-unit-msbc"}
		}
		if len(args) == 3 && args[0] == "set-profile" {
			f.profile = names[args[2]]
		}
		return nil, nil
	}
	return nil, errors.New("unexpected command " + name)
}

func (f *fakeHeadsetRunner) Pipe(_ context.Context, name string, args ...string) (headsetProcess, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	process := newFakeHeadsetProcess(name, name == "pw-play" && f.blockPlay)
	f.pipes = append(f.pipes, process)
	return process, nil
}

func (f *fakeHeadsetRunner) called(command string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, call := range f.calls {
		if call == command {
			return true
		}
	}
	return false
}

func (f *fakeHeadsetRunner) countPrefix(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, call := range f.calls {
		if strings.HasPrefix(call, prefix) {
			count++
		}
	}
	return count
}

// pipe waits for the index-th process started with the given tool.
func (f *fakeHeadsetRunner) pipe(t *testing.T, tool string, index int) *fakeHeadsetProcess {
	t.Helper()
	var found *fakeHeadsetProcess
	waitForHeadset(t, fmt.Sprintf("%s #%d", tool, index), func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		seen := 0
		for _, process := range f.pipes {
			if process.tool != tool {
				continue
			}
			if seen == index {
				found = process
				return true
			}
			seen++
		}
		return false
	})
	return found
}

func (f *fakeHeadsetRunner) callLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

type fakeHeadsetProcess struct {
	tool     string
	stdoutR  *io.PipeReader
	stdoutW  *io.PipeWriter
	block    chan struct{}
	writing  chan struct{}
	killed   chan struct{}
	exited   chan struct{}
	killOnce sync.Once
	exitOnce sync.Once
	mu       sync.Mutex
	written  bytes.Buffer
}

func newFakeHeadsetProcess(tool string, block bool) *fakeHeadsetProcess {
	reader, writer := io.Pipe()
	process := &fakeHeadsetProcess{tool: tool, stdoutR: reader, stdoutW: writer, writing: make(chan struct{}, 1),
		killed: make(chan struct{}), exited: make(chan struct{})}
	if block {
		process.block = make(chan struct{})
	}
	return process
}

func (p *fakeHeadsetProcess) Stdin() io.WriteCloser { return fakeHeadsetStdin{p} }
func (p *fakeHeadsetProcess) Stdout() io.ReadCloser { return p.stdoutR }

func (p *fakeHeadsetProcess) Wait() error {
	select {
	case <-p.killed:
	case <-p.exited:
	}
	return nil
}

func (p *fakeHeadsetProcess) Kill() error {
	p.killOnce.Do(func() {
		close(p.killed)
		_ = p.stdoutW.CloseWithError(io.ErrClosedPipe)
	})
	return nil
}

// exit simulates the tool ending on its own.
func (p *fakeHeadsetProcess) exit() {
	p.exitOnce.Do(func() {
		close(p.exited)
		_ = p.stdoutW.Close()
	})
}

func (p *fakeHeadsetProcess) wasKilled() bool {
	select {
	case <-p.killed:
		return true
	default:
		return false
	}
}

func (p *fakeHeadsetProcess) bytesWritten() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.written.Len()
}

type fakeHeadsetStdin struct{ p *fakeHeadsetProcess }

func (s fakeHeadsetStdin) Write(data []byte) (int, error) {
	select {
	case s.p.writing <- struct{}{}:
	default:
	}
	if s.p.block != nil {
		select {
		case <-s.p.block:
		case <-s.p.killed:
			return 0, io.ErrClosedPipe
		}
	}
	if s.p.wasKilled() {
		return 0, io.ErrClosedPipe
	}
	s.p.mu.Lock()
	defer s.p.mu.Unlock()
	return s.p.written.Write(data)
}

func (s fakeHeadsetStdin) Close() error { return nil }

func waitForHeadset(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func connectedHeadset() (Device, bool) {
	return Device{Address: testHeadsetAddress, Paired: true, Connected: true, UUIDs: []string{a2dpSinkUUID, handsfreeUUID}}, true
}

func startTestHeadsetLink(t *testing.T, runner *fakeHeadsetRunner, lookup func() (Device, bool), changes <-chan Change) *HeadsetLink {
	t.Helper()
	link := startHeadsetLink(context.Background(), headsetLinkConfig{
		address:       testHeadsetAddress,
		runner:        runner,
		lookup:        func(context.Context) (Device, bool) { return lookup() },
		changes:       changes,
		pollInterval:  5 * time.Millisecond,
		readyTimeout:  500 * time.Millisecond,
		retryInterval: 20 * time.Millisecond,
	})
	t.Cleanup(func() { _ = link.Close() })
	return link
}

func expectHeadsetEvent(t *testing.T, link *HeadsetLink, kind, code string) {
	t.Helper()
	select {
	case event := <-link.Events():
		if event.Type != kind || event.Code != code {
			t.Fatalf("event = %+v, want %s %q", event, kind, code)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no %s event", kind)
	}
}

func TestHeadsetLinkSwitchesToHeadsetProfileAndRestoresIt(t *testing.T) {
	runner := newFakeHeadsetRunner("a2dp-sink")
	link := startTestHeadsetLink(t, runner, connectedHeadset, nil)
	expectHeadsetEvent(t, link, HeadsetReady, "")
	if !runner.called("wpctl set-profile 71 3") {
		t.Fatalf("profile switch missing: %v", runner.callLog())
	}
	if !runner.called("pw-record --target bluez_input.AA_BB_CC_DD_EE_FF.0 --rate 16000 --channels 1 --format s16 -") {
		t.Fatalf("recorder missing: %v", runner.callLog())
	}
	recorder := runner.pipe(t, "pw-record", 0)
	frame := bytes.Repeat([]byte{1, 2}, HeadsetFrameBytes/2)
	go func() { _, _ = recorder.stdoutW.Write(frame) }()
	select {
	case got := <-link.Frames():
		if !bytes.Equal(got, frame) {
			t.Fatal("microphone frame changed on the way")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no microphone frame")
	}
	_ = link.Close()
	if !recorder.wasKilled() {
		t.Fatal("recorder still runs after Close")
	}
	if !runner.called("wpctl set-profile 71 1") {
		t.Fatalf("A2DP not restored: %v", runner.callLog())
	}
}

func TestHeadsetLinkKeepsAHeadsetProfileItDidNotSet(t *testing.T) {
	runner := newFakeHeadsetRunner("headset-head-unit")
	link := startTestHeadsetLink(t, runner, connectedHeadset, nil)
	expectHeadsetEvent(t, link, HeadsetReady, "")
	_ = link.Close()
	if count := runner.countPrefix("wpctl"); count != 0 {
		t.Fatalf("wpctl calls = %d, want none: %v", count, runner.callLog())
	}
}

func TestHeadsetLinkFollowsTheConnection(t *testing.T) {
	runner := newFakeHeadsetRunner("headset-head-unit")
	var connected atomic.Bool
	changes := make(chan Change, 8)
	link := startTestHeadsetLink(t, runner, func() (Device, bool) {
		device, _ := connectedHeadset()
		device.Connected = connected.Load()
		return device, true
	}, changes)
	expectHeadsetEvent(t, link, HeadsetLost, "")
	connected.Store(true)
	changes <- Change{Revision: 1}
	expectHeadsetEvent(t, link, HeadsetReady, "")
	first := runner.pipe(t, "pw-record", 0)
	connected.Store(false)
	changes <- Change{Revision: 2}
	expectHeadsetEvent(t, link, HeadsetLost, "")
	waitForHeadset(t, "recorder stop", first.wasKilled)
	connected.Store(true)
	changes <- Change{Revision: 3}
	expectHeadsetEvent(t, link, HeadsetReady, "")
	runner.pipe(t, "pw-record", 1)
}

func TestHeadsetLinkRestartsAnEndedRecorder(t *testing.T) {
	runner := newFakeHeadsetRunner("headset-head-unit")
	link := startTestHeadsetLink(t, runner, connectedHeadset, nil)
	expectHeadsetEvent(t, link, HeadsetReady, "")
	runner.pipe(t, "pw-record", 0).exit()
	runner.pipe(t, "pw-record", 1)
}

func TestHeadsetLinkReportsAMissingMicrophoneProfileOnce(t *testing.T) {
	runner := newFakeHeadsetRunner("a2dp-sink")
	runner.dump = func(string) []byte { return []byte(noMicrophoneDump) }
	link := startTestHeadsetLink(t, runner, connectedHeadset, nil)
	expectHeadsetEvent(t, link, HeadsetLost, "")
	expectHeadsetEvent(t, link, HeadsetError, ErrorHeadsetProfileUnavailable)
	select {
	case event := <-link.Events():
		t.Fatalf("repeated event %+v while nothing changed", event)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestHeadsetLinkPlaysBrowserAudioPerStream(t *testing.T) {
	runner := newFakeHeadsetRunner("headset-head-unit")
	var connected atomic.Bool
	link := startTestHeadsetLink(t, runner, func() (Device, bool) {
		device, _ := connectedHeadset()
		device.Connected = connected.Load()
		return device, true
	}, nil)
	expectHeadsetEvent(t, link, HeadsetLost, "")
	// While lost the browser plays locally; nothing reaches the headset.
	if err := link.Write(0, []byte{1, 2}); err != nil || runner.countPrefix("pw-play") != 0 {
		t.Fatalf("write while lost: err=%v calls=%v", err, runner.callLog())
	}
	connected.Store(true)
	expectHeadsetEvent(t, link, HeadsetReady, "")

	if err := link.Write(0, []byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	if !runner.called("pw-play --target bluez_output.AA_BB_CC_DD_EE_FF.0 --rate 24000 --channels 1 --format s16 -") {
		t.Fatalf("player missing: %v", runner.callLog())
	}
	reply := runner.pipe(t, "pw-play", 0)
	waitForHeadset(t, "reply audio", func() bool { return reply.bytesWritten() == 4 })
	if err := link.Write(1, []byte{5, 6}); err != nil {
		t.Fatal(err)
	}
	runner.pipe(t, "pw-play", 1)

	link.Flush(0)
	waitForHeadset(t, "flushed player", reply.wasKilled)
	if err := link.Write(0, []byte{7, 8}); err != nil {
		t.Fatal(err)
	}
	runner.pipe(t, "pw-play", 2)
	if err := link.Write(2, []byte{1}); ErrorCode(err) != ErrorInvalidArgument {
		t.Fatalf("unknown stream error = %v", err)
	}
}

func TestHeadsetLinkDropsAudioBeyondTwoSeconds(t *testing.T) {
	runner := newFakeHeadsetRunner("headset-head-unit")
	runner.blockPlay = true
	link := startTestHeadsetLink(t, runner, connectedHeadset, nil)
	expectHeadsetEvent(t, link, HeadsetReady, "")
	second := make([]byte, HeadsetOutputRate*2) // 1 s of s16le mono
	if err := link.Write(0, second); err != nil {
		t.Fatal(err)
	}
	player := runner.pipe(t, "pw-play", 0)
	select {
	case <-player.writing: // the first second is being written and blocks
	case <-time.After(2 * time.Second):
		t.Fatal("player never started writing")
	}
	for i := 0; i < 3; i++ {
		_ = link.Write(0, second)
	}
	close(player.block)
	waitForHeadset(t, "queued audio", func() bool { return player.bytesWritten() == 3*len(second) })
	time.Sleep(50 * time.Millisecond)
	if got := player.bytesWritten(); got != 3*len(second) {
		t.Fatalf("written = %d bytes, want %d (the fourth second must be dropped)", got, 3*len(second))
	}
}

func newHeadsetTestManager(runner *fakeHeadsetRunner, devices ...Device) *Manager {
	manager := newTestManager(&fakeAdapter{devices: devices})
	manager.status.Present = true
	manager.headsetRunner = runner
	return manager
}

func TestManagerListsAndReservesHeadsets(t *testing.T) {
	ctx := context.Background()
	manager := newHeadsetTestManager(newFakeHeadsetRunner("headset-head-unit"),
		Device{Address: testHeadsetAddress, Alias: "Earbuds", Paired: true, Connected: true, UUIDs: []string{a2dpSinkUUID, handsfreeUUID}},
		Device{Address: "AA:BB:CC:DD:EE:01", Alias: "Speaker", Paired: true, UUIDs: []string{a2dpSinkUUID}},
		Device{Address: "AA:BB:CC:DD:EE:02", Alias: "Stranger", UUIDs: []string{handsfreeUUID}},
	)
	devices, reason := manager.HeadsetDevices(ctx)
	if reason != "" || len(devices) != 1 || devices[0] != (HeadsetDevice{Address: testHeadsetAddress, Name: "Earbuds", Connected: true}) {
		t.Fatalf("devices = %+v reason = %q", devices, reason)
	}

	link, err := manager.OpenHeadset(ctx, "aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatal(err)
	}
	if devices, _ := manager.HeadsetDevices(ctx); !devices[0].Busy {
		t.Fatalf("open headset not busy: %+v", devices)
	}
	if _, err := manager.OpenHeadset(ctx, testHeadsetAddress); ErrorCode(err) != ErrorHeadsetBusy {
		t.Fatalf("second open error = %v", err)
	}
	_ = link.Close()
	again, err := manager.OpenHeadset(ctx, testHeadsetAddress)
	if err != nil {
		t.Fatalf("reopen after Close: %v", err)
	}
	_ = again.Close()

	if _, err := manager.OpenHeadset(ctx, "AA:BB:CC:DD:EE:01"); ErrorCode(err) != ErrorHeadsetUnknown {
		t.Fatalf("speaker without microphone error = %v", err)
	}
	if _, err := manager.OpenHeadset(ctx, "not-an-address"); ErrorCode(err) != ErrorInvalidArgument {
		t.Fatalf("invalid address error = %v", err)
	}

	manager.status.Audio = AudioStatus{Usable: true, Backend: "pulse"}
	if devices, reason := manager.HeadsetDevices(ctx); len(devices) != 0 || !strings.Contains(reason, "PipeWire") {
		t.Fatalf("pulse devices = %+v reason = %q", devices, reason)
	}
	if _, err := manager.OpenHeadset(ctx, testHeadsetAddress); ErrorCode(err) != ErrorHeadsetAudioUnavailable {
		t.Fatalf("pulse open error = %v", err)
	}
	manager.options.Enabled = false
	if _, reason := manager.HeadsetDevices(ctx); !strings.Contains(reason, "disabled") {
		t.Fatalf("disabled reason = %q", reason)
	}
}

// pipeWire16Dump replays a real PipeWire 1.6 / WirePlumber 0.5 snapshot
// (anonymized) with the device switched to the given profile. Since PipeWire
// 1.x the usable nodes are loopbacks named bluez_input.<MAC> and
// bluez_output.<MAC> that carry device.id instead of an address property.
func pipeWire16Dump(t *testing.T) func(profile string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "pipewire-1.6-headset-a2dp.json"))
	if err != nil {
		t.Fatal(err)
	}
	return func(profile string) []byte {
		var objects []map[string]interface{}
		if err := json.Unmarshal(raw, &objects); err != nil {
			panic(err)
		}
		for _, object := range objects {
			if object["type"] != "PipeWire:Interface:Device" {
				continue
			}
			params := object["info"].(map[string]interface{})["params"].(map[string]interface{})
			for _, entry := range params["EnumProfile"].([]interface{}) {
				if entry.(map[string]interface{})["name"] == profile {
					params["Profile"] = []interface{}{entry}
				}
			}
		}
		out, err := json.Marshal(objects)
		if err != nil {
			panic(err)
		}
		return out
	}
}

var pipeWire16ProfileNames = map[string]string{
	"0": "off", "131073": "a2dp-sink", "131074": "a2dp-sink-sbc_xq", "196864": "headset-head-unit-cvsd", "196865": "headset-head-unit",
}

func TestParseHeadsetGraphReadsPipeWire16LoopbackNodes(t *testing.T) {
	dump := pipeWire16Dump(t)
	graph, found, err := parseHeadsetGraph(dump("a2dp-sink"), testHeadsetAddress)
	if err != nil || !found || graph.DeviceID != 62 || graph.Current.Name != "a2dp-sink" {
		t.Fatalf("graph = %+v found=%v err=%v", graph, found, err)
	}
	// The loopbacks, not the *_internal nodes, are what clients record from and play to.
	if graph.Source != "bluez_input.AA:BB:CC:DD:EE:FF" || graph.Sink != "bluez_output.AA:BB:CC:DD:EE:FF" {
		t.Fatalf("nodes = %q %q", graph.Source, graph.Sink)
	}
	if best, ok := graph.bestHeadsetProfile(); !ok || best.Index != 196865 {
		t.Fatalf("best profile = %+v ok=%v", best, ok)
	}
}

func TestHeadsetLinkUsesPipeWire16LoopbackNodes(t *testing.T) {
	runner := newFakeHeadsetRunner("a2dp-sink")
	runner.dump = pipeWire16Dump(t)
	runner.profileNames = pipeWire16ProfileNames
	link := startTestHeadsetLink(t, runner, connectedHeadset, nil)
	expectHeadsetEvent(t, link, HeadsetReady, "")
	if !runner.called("wpctl set-profile 62 196865") {
		t.Fatalf("profile switch missing: %v", runner.callLog())
	}
	if !runner.called("pw-record --target bluez_input.AA:BB:CC:DD:EE:FF --rate 16000 --channels 1 --format s16 -") {
		t.Fatalf("recorder missing: %v", runner.callLog())
	}
	if err := link.Write(0, []byte{1, 2}); err != nil {
		t.Fatal(err)
	}
	if !runner.called("pw-play --target bluez_output.AA:BB:CC:DD:EE:FF --rate 24000 --channels 1 --format s16 -") {
		t.Fatalf("player missing: %v", runner.callLog())
	}
	_ = link.Close()
	if !runner.called("wpctl set-profile 62 131073") {
		t.Fatalf("A2DP not restored: %v", runner.callLog())
	}
}
