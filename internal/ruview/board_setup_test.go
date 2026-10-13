package ruview

import (
	"reflect"
	"testing"

	"aurago/internal/config"
)

func TestBoardSetupReceiverCandidatesRespectRuntimePlacement(t *testing.T) {
	for _, tc := range []struct {
		name, mode, endpoint, bind string
		inDocker                   bool
		want                       []string
		localCalls                 int
	}{
		{"local Docker", "docker", "", "", false, []string{"192.168.4.20", "10.1.0.2"}, 1},
		{"native host", "native", "", "", false, []string{"192.168.4.20", "10.1.0.2"}, 1},
		{"saved LAN binding", "docker", "", "192.168.8.20", false, []string{"192.168.8.20"}, 0},
		{"container interfaces are not the host", "docker", "", "", true, []string{}, 0},
		{"container configured binding", "docker", "", "192.168.8.20", true, []string{"192.168.8.20"}, 0},
		{"container socket proxy", "docker", "tcp://172.18.0.2:2375", "", true, []string{}, 0},
		{"remote Docker host", "docker", "tcp://192.168.8.20:2375", "", false, []string{"192.168.8.20"}, 0},
		{"remote hostname is not resolved", "docker", "tcp://engine.example:2375", "", false, []string{}, 0},
		{"external receiver", "external", "http://10.2.0.3:3000", "192.168.8.20", false, []string{"10.2.0.3"}, 0},
		{"external DNS", "external", "https://receiver.example", "", false, []string{}, 0},
		{"external loopback", "external", "http://127.0.0.1:3000", "", false, []string{}, 0},
		{"public endpoint", "external", "http://8.8.8.8:3000", "", false, []string{}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &config.Config{RuView: config.DefaultRuViewConfig()}
			c.RuView.Mode, c.RuView.URL, c.RuView.UDPBind = tc.mode, tc.endpoint, tc.bind
			c.RuView.ReceiverIP, c.RuView.UDPPort = "192.168.7.10", 5010
			c.Docker.Host, c.Runtime.IsDocker = tc.endpoint, tc.inDocker
			calls := 0
			got := boardSetup(c, func() []string {
				calls++
				return []string{"192.168.4.20", "10.1.0.2", "192.168.4.20", "127.0.0.1", "8.8.8.8", "fe80::1"}
			})
			if !reflect.DeepEqual(got.Candidates, tc.want) || calls != tc.localCalls {
				t.Fatalf("candidates %v, local interface reads %d; want %v, %d", got.Candidates, calls, tc.want, tc.localCalls)
			}
			if got.ReceiverIP != c.RuView.ReceiverIP || got.UDPPort != 5010 || got.UDPEnabled || c.RuView.UDPEnabled || !c.RuView.ReadOnly {
				t.Fatal("setup must preserve saved values without enabling UDP or writes")
			}
		})
	}
}

func TestBoardSetupSkipsKnownVirtualInterfaces(t *testing.T) {
	for _, name := range []string{"docker0", "br-123", "veth123", "tailscale0", "wg0", "tun0", "utun1", "vEthernet (Default Switch)"} {
		if !receiverVirtualInterface(name) {
			t.Errorf("virtual interface %q was offered as a LAN interface", name)
		}
	}
	for _, name := range []string{"enp2s0", "eth0", "wlan0", "en0", "Ethernet", "Wi-Fi", "br0"} {
		if receiverVirtualInterface(name) {
			t.Errorf("LAN interface %q was excluded", name)
		}
	}
}
