package server

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"aurago/internal/infra"
)

func TestDesktopInfraDerivedLiveUsesOnlyCurrentProviderPower(t *testing.T) {
	current := func(power string) infra.Source {
		return infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: "engine", Key: "container"}, State: infra.SourceCurrent, Observed: infra.SourceObserved{Power: power}}
	}
	stale := func(power string) infra.Source {
		return infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: "engine", Key: "container"}, State: infra.SourceStale, Observed: infra.SourceObserved{Power: power}}
	}
	for _, tc := range []struct {
		name    string
		sources []infra.Source
		want    string
	}{
		{name: "no source", sources: nil, want: ""},
		{name: "running", sources: []infra.Source{current(infra.PowerRunning)}, want: infra.LiveLive},
		{name: "stopped", sources: []infra.Source{current(infra.PowerStopped)}, want: infra.LiveOffline},
		{name: "paused", sources: []infra.Source{current(infra.PowerPaused)}, want: infra.LiveUnknown},
		{name: "stale source has no say", sources: []infra.Source{stale(infra.PowerRunning)}, want: ""},
		{name: "sources disagree", sources: []infra.Source{current(infra.PowerRunning), current(infra.PowerStopped)}, want: infra.LiveUnknown},
		{name: "one source without power", sources: []infra.Source{current("")}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := desktopInfraDerivedLive(tc.sources); got != tc.want {
				t.Fatalf("derived live = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDesktopInfraProjectLiveCombinesProviderAndProbe(t *testing.T) {
	now := time.Now()
	fresh := func(state string) desktopInfraLiveness {
		return desktopInfraLiveness{states: map[string]desktopInfraLivenessEntry{"192.0.2.10": {State: state, CheckedAt: now}}}
	}
	expired := desktopInfraLiveness{states: map[string]desktopInfraLivenessEntry{"192.0.2.10": {State: infra.LiveLive, CheckedAt: now.Add(-2 * desktopInfraLivenessTTL)}}}
	device := func(power string) infra.Node {
		return infra.Node{Kind: infra.NodeDevice, Name: "Probe Target", Profile: infra.Profile{IPAddress: "192.0.2.10"}, Sources: []infra.Source{{State: infra.SourceCurrent, Observed: infra.SourceObserved{Power: power}}}}
	}
	for _, tc := range []struct {
		name     string
		node     infra.Node
		liveness desktopInfraLiveness
		want     string
	}{
		{name: "provider says live", node: device(infra.PowerRunning), liveness: desktopInfraLiveness{}, want: infra.LiveLive},
		{name: "provider says offline", node: device(infra.PowerStopped), liveness: desktopInfraLiveness{}, want: infra.LiveOffline},
		{name: "probe wins over a stale provider", node: device(infra.PowerStopped), liveness: fresh(infra.LiveLive), want: infra.LiveLive},
		{name: "provider wins over a failing probe", node: device(infra.PowerRunning), liveness: fresh(infra.LiveOffline), want: infra.LiveLive},
		{name: "probe alone", node: device(""), liveness: fresh(infra.LiveOffline), want: infra.LiveOffline},
		{name: "expired probe is unknown", node: device(""), liveness: expired, want: infra.LiveUnknown},
		{name: "no signal at all", node: device(""), liveness: desktopInfraLiveness{}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := desktopInfraProjectLive(tc.node, tc.liveness); got != tc.want {
				t.Fatalf("projected live = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDesktopInfraProbeTCPAnswersForAnOpenPortOnly(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() { _ = listener.Close() })

	original := desktopInfraLivenessPorts
	desktopInfraLivenessPorts = []string{fmt.Sprint(port)}
	t.Cleanup(func() { desktopInfraLivenessPorts = original })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !desktopInfraProbeTCP(ctx, "127.0.0.1") {
		t.Fatal("the open port was not detected")
	}
	desktopInfraLivenessPorts = []string{fmt.Sprint(port + 1)}
	if desktopInfraProbeTCP(ctx, "127.0.0.1") {
		t.Fatal("a closed port was reported as open")
	}
}

func TestDesktopInfraLivenessPassAnswersForAnOpenPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() { _ = listener.Close() })

	original := desktopInfraLivenessPorts
	desktopInfraLivenessPorts = []string{fmt.Sprint(port)}
	t.Cleanup(func() { desktopInfraLivenessPorts = original })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results := desktopInfraProbeAddresses(ctx, []string{"127.0.0.1"})
	entry, ok := results["127.0.0.1"]
	if !ok || entry.State != infra.LiveLive {
		t.Fatalf("probe result = %#v, want a live verdict", entry)
	}
	if time.Since(entry.CheckedAt) > time.Minute {
		t.Fatal("the probe must stamp its verdict")
	}
}

func TestDesktopInfraSetLivenessDropsAnswersOfAReplacedConfiguration(t *testing.T) {
	runtime := &desktopInfraRuntime{cache: make(map[string]*desktopInfraProviderCache), liveness: map[string]desktopInfraLivenessEntry{}, configKey: "config-a"}
	now := time.Now()
	runtime.setLiveness(map[string]desktopInfraLivenessEntry{"192.0.2.10": {State: infra.LiveLive, CheckedAt: now}}, now, "config-a")
	if runtime.liveness["192.0.2.10"].State != infra.LiveLive {
		t.Fatal("the verdict of a pass must be cached")
	}
	runtime.mu.Lock()
	runtime.configKey = "config-b"
	runtime.mu.Unlock()
	runtime.setLiveness(map[string]desktopInfraLivenessEntry{"192.0.2.10": {State: infra.LiveOffline, CheckedAt: now}}, now, "config-a")
	if runtime.liveness["192.0.2.10"].State != infra.LiveLive {
		t.Fatal("the answers of a pass that ran under a replaced configuration must be dropped")
	}
}

func TestDesktopInfraRotateAddressesHandsTheStartToTheNextPass(t *testing.T) {
	addresses := []string{"192.0.2.10", "192.0.2.11", "192.0.2.12"}
	rotated := desktopInfraRotateAddresses(addresses, 1)
	want := []string{"192.0.2.11", "192.0.2.12", "192.0.2.10"}
	for index, address := range want {
		if rotated[index] != address {
			t.Fatalf("rotated addresses = %v, want %v", rotated, want)
		}
	}
	if same := desktopInfraRotateAddresses(addresses, 0); same[0] != addresses[0] {
		t.Fatalf("rotated addresses = %v, want the original order", same)
	}
}

func TestDesktopInfraLivenessAddressesSkipsLocalHostsAndPlaces(t *testing.T) {
	_, _, store, _ := newDesktopInfraRefreshFixture(t)
	ctx := context.Background()

	original := desktopInfraLocalAddresses
	desktopInfraLocalAddresses = func() []netip.Addr {
		return []netip.Addr{netip.MustParseAddr("192.0.2.11")}
	}
	t.Cleanup(func() { desktopInfraLocalAddresses = original })

	device, err := store.CreateManualNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "Reachable Device", Icon: "generic-device", Profile: infra.Profile{IPAddress: "192.0.2.10"}})
	if err != nil {
		t.Fatal(err)
	}
	if device.Profile.IPAddress != "192.0.2.10" {
		t.Fatalf("device address = %q", device.Profile.IPAddress)
	}
	if _, err := store.CreateManualNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "Agent Host", Icon: "generic-device", Profile: infra.Profile{IPAddress: "192.0.2.11"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeSite, Name: "Rack Row", Icon: "rack", Profile: infra.Profile{IPAddress: "192.0.2.12"}}); err != nil {
		t.Fatal(err)
	}

	addresses := desktopInfraLivenessAddresses(ctx, store)
	for _, address := range addresses {
		if address == "192.0.2.11" {
			t.Fatal("the address of the AuraGo host must not be probed")
		}
		if address == "192.0.2.12" {
			t.Fatal("a place must not be probed")
		}
	}
	found := false
	for _, address := range addresses {
		if address == "192.0.2.10" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the device address is missing from %v", addresses)
	}
}
