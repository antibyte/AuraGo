package atlaspdf

import (
	"fmt"

	"aurago/internal/infra"
)

// fixture builds snapshots with plausible homelab content.
type fixture struct {
	nodes []infra.Node
	edges []infra.Edge
	mac   int
}

func (f *fixture) add(n infra.Node) *infra.Node {
	if n.Profile.Tags == nil {
		n.Profile.Tags = []string{}
	}
	f.nodes = append(f.nodes, n)
	return &f.nodes[len(f.nodes)-1]
}

func (f *fixture) macAddr() string {
	f.mac++
	return fmt.Sprintf("a4:2b:b0:%02x:%02x:%02x", f.mac>>16&255, f.mac>>8&255, f.mac&255)
}

func (f *fixture) place(kind infra.NodeKind, id, name, parent string, x, y, w, h float64) {
	f.add(infra.Node{
		ID: id, Kind: kind, Name: name, ParentID: parent, Icon: string(kind),
		Placements: infra.Placements{infra.ViewAtlas: {X: x, Y: y, W: w, H: h}},
	})
}

// asset adds a placed (x, y are the card centre) or, with placed=false, unplaced asset.
func (f *fixture) asset(kind infra.NodeKind, id, name, icon, parent, ip, power string, x, y float64, placed bool) {
	n := infra.Node{
		ID: id, Kind: kind, Name: name, Icon: icon, ParentID: parent, Power: power,
		Profile: infra.Profile{IPAddress: ip, MACAddress: f.macAddr(), OS: "Debian 12", CPU: "4 cores", RAM: "8 GB", VLAN: 10},
		Sources: []infra.Source{{SourceIdentity: infra.SourceIdentity{Kind: "proxmox", Instance: "pve01", Key: id}, State: infra.SourceCurrent}},
	}
	if placed {
		n.Placements = infra.Placements{infra.ViewAtlas: {X: x, Y: y}}
	}
	f.add(n)
}

func (f *fixture) guest(kind infra.NodeKind, id, name, icon, host, ip, power string) {
	f.add(infra.Node{
		ID: id, Kind: kind, Name: name, Icon: icon, HostID: host, Power: power,
		Profile: infra.Profile{IPAddress: ip, Role: "guest"},
	})
}

func (f *fixture) link(from, to string, kind infra.EdgeKind, fromPort, toPort, speed string) {
	f.edges = append(f.edges, infra.Edge{
		ID: fmt.Sprintf("e%d", len(f.edges)), From: from, To: to, Kind: kind, FromPort: fromPort, ToPort: toPort, Speed: speed,
	})
}

func (f *fixture) snapshot() infra.Snapshot {
	return infra.Snapshot{Nodes: f.nodes, Edges: f.edges}
}

// smallLab is a one-room lab that fits one readable page.
func smallLab() infra.Snapshot {
	f := &fixture{}
	f.place(infra.NodeSite, "site", "Zuhause", "", 0, 0, 1000, 520)
	f.asset(infra.NodeDevice, "fritz", "Fritz!Box 7590", "router", "site", "192.168.178.1", "running", 160, 140, true)
	f.asset(infra.NodeDevice, "sw", "UniFi Switch 24", "managed-switch", "site", "192.168.178.2", "running", 460, 140, true)
	f.asset(infra.NodeDevice, "ap", "UniFi AP Wohnzimmer", "access-point", "site", "192.168.178.3", "running", 760, 140, true)
	f.asset(infra.NodeDevice, "nas", "TrueNAS Scale", "nas", "site", "192.168.178.20", "running", 160, 330, true)
	f.asset(infra.NodeDevice, "pi", "Raspberry Pi 4", "sbc", "site", "192.168.178.30", "running", 460, 330, true)
	f.asset(infra.NodeDevice, "pve", "Proxmox pve01", "hypervisor", "site", "192.168.178.10", "running", 760, 330, true)
	f.asset(infra.NodeNetwork, "lan", "LAN 192.168.178.0/24", "lan", "", "", "", 460, 640, true)
	f.guest(infra.NodeVM, "ha", "Home Assistant", "home-automation", "pve", "192.168.178.40", "running")
	f.guest(infra.NodeContainer, "pihole", "Pi-hole", "dns-dhcp", "pi", "192.168.178.31", "running")
	f.guest(infra.NodeContainer, "jelly", "Jellyfin", "media-server", "nas", "192.168.178.21", "stopped")
	f.link("fritz", "sw", infra.EdgeEthernet, "LAN1", "Port 24", "1 Gbit/s")
	f.link("sw", "ap", infra.EdgeEthernet, "Port 3", "eth0", "1 Gbit/s")
	f.link("sw", "nas", infra.EdgeEthernet, "Port 5", "eno1", "2.5 Gbit/s")
	f.link("sw", "pi", infra.EdgeEthernet, "Port 7", "eth0", "")
	f.link("sw", "pve", infra.EdgeFiber, "SFP+ 1", "enp3s0", "10 Gbit/s")
	f.link("ap", "pi", infra.EdgeWiFi, "", "", "")
	f.link("lan", "sw", infra.EdgeMemberOf, "", "", "")
	return f.snapshot()
}

// largeLab spreads about 120 assets over three sites with rooms and racks.
func largeLab() infra.Snapshot {
	f := &fixture{}
	f.place(infra.NodeSite, "home", "Zuhause", "", 0, 0, 1900, 1300)
	f.place(infra.NodeRoom, "keller", "Keller", "home", 60, 110, 1100, 1000)
	f.place(infra.NodeRack, "rack1", "Serverschrank", "keller", 100, 170, 1000, 880)
	f.place(infra.NodeRoom, "buero", "Büro", "home", 1240, 110, 600, 700)
	f.place(infra.NodeSite, "cabin", "Ferienhaus", "", 2500, 0, 900, 600)
	f.place(infra.NodeSite, "colo", "Colo Frankfurt", "", 3700, 0, 1300, 900)
	f.place(infra.NodeRack, "colorack", "Rack 12", "colo", 3760, 80, 1180, 780)
	f.place(infra.NodeGroup, "iot", "IoT", "", 100, 1500, 1500, 400)

	f.asset(infra.NodeDevice, "fritz", "Fritz!Box 7590", "router", "buero", "192.168.178.1", "running", 1540, 240, true)
	f.asset(infra.NodeDevice, "ap", "UniFi AP Büro", "access-point", "buero", "192.168.178.3", "running", 1540, 400, true)
	f.asset(infra.NodeDevice, "mesh", "Mesh Repeater", "mesh-node", "buero", "192.168.178.4", "stopped", 1540, 560, true)
	f.asset(infra.NodeDevice, "ws", "Workstation Andi", "workstation", "buero", "192.168.178.50", "running", 1540, 700, true)

	names := []struct{ id, name, icon, ip, power string }{
		{"sw", "UniFi Switch Pro 48", "managed-switch", "192.168.10.2", "running"},
		{"pve", "Proxmox pve01", "hypervisor", "192.168.10.10", "running"},
		{"nas", "TrueNAS Scale", "nas", "192.168.10.20", "running"},
		{"dock", "docker01", "mini-pc", "192.168.10.11", "running"},
		{"pi1", "Raspberry Pi 4 (Pi-hole)", "sbc", "192.168.10.31", "running"},
		{"pi2", "Raspberry Pi 5 (Klipper)", "sbc", "192.168.10.32", "paused"},
		{"ups", "APC Smart-UPS 1500", "ups", "192.168.10.5", "running"},
		{"pdu", "PDU Rack", "pdu", "192.168.10.6", "unknown"},
		{"bak", "Backup NAS Synology", "nas", "192.168.10.21", "stopped"},
	}
	for i, n := range names {
		f.asset(infra.NodeDevice, n.id, n.name, n.icon, "rack1", n.ip, n.power, 340+float64(i%3)*300, 300+float64(i/3)*240, true)
	}
	for i := 0; i < 6; i++ {
		f.asset(infra.NodeDevice, fmt.Sprintf("iot%d", i), fmt.Sprintf("Shelly Plug %d", i+1), "smart-plug", "iot", fmt.Sprintf("192.168.40.%d", 20+i), "running", 250+float64(i)*250, 1700, true)
	}

	f.asset(infra.NodeDevice, "cabinrouter", "GL.iNet Router", "router", "cabin", "10.20.0.1", "running", 2900, 200, true)
	f.asset(infra.NodeDevice, "cabinnas", "Synology DS220", "nas", "cabin", "10.20.0.20", "running", 3200, 200, true)
	f.asset(infra.NodeDevice, "cabincam", "Reolink Cam", "camera", "cabin", "10.20.0.30", "running", 2900, 400, true)

	f.asset(infra.NodeDevice, "coloedge", "pfSense Edge", "firewall", "colorack", "203.0.113.2", "running", 4000, 220, true)
	f.asset(infra.NodeDevice, "colosw", "Mikrotik CRS326", "managed-switch", "colorack", "203.0.113.3", "running", 4400, 220, true)
	for i := 0; i < 4; i++ {
		f.asset(infra.NodeDevice, fmt.Sprintf("srv%d", i), fmt.Sprintf("Dell R630 node%d", i+1), "rack-server", "colorack", fmt.Sprintf("203.0.113.%d", 10+i), "running", 4000+float64(i%2)*420, 440+float64(i/2)*200, true)
	}
	f.asset(infra.NodeNetwork, "wan", "Internet", "internet", "", "", "", -500, 300, true)
	f.asset(infra.NodeNetwork, "vlan10", "VLAN 10 Server", "vlan", "", "", "", -500, 700, true)

	for i := 0; i < 8; i++ {
		f.guest(infra.NodeVM, fmt.Sprintf("vm%d", i), []string{"Home Assistant", "Nextcloud", "Pi-hole Secondary", "Windows 11", "GitLab Runner", "Mailcow", "Vaultwarden", "Test-VM"}[i], "vm", "pve", fmt.Sprintf("192.168.10.%d", 40+i), []string{"running", "running", "running", "stopped", "running", "running", "running", "paused"}[i])
	}
	services := []string{"Jellyfin", "Grafana", "Prometheus", "Traefik", "Portainer", "Uptime Kuma", "Paperless-ngx", "Immich", "Mosquitto", "Node-RED", "Gitea", "Authentik", "Vault", "Loki", "Watchtower", "Syncthing", "Homepage", "ntfy", "Zigbee2MQTT", "ESPHome", "Frigate", "Searxng", "Wallabag", "Audiobookshelf", "Navidrome", "Tdarr", "Radarr", "Sonarr", "Prowlarr", "Bazarr", "Overseerr", "qBittorrent", "AdGuard Home", "Duplicati", "Dozzle"}
	for i, s := range services {
		power := "running"
		if i%11 == 7 {
			power = "stopped"
		}
		f.guest(infra.NodeContainer, fmt.Sprintf("c%d", i), s, "container", "dock", fmt.Sprintf("172.18.0.%d", 10+i), power)
	}
	f.guest(infra.NodeService, "svc-dns", "DNS", "dns-dhcp", "pi1", "", "running")
	f.guest(infra.NodeContainer, "orphan", "old-test-container", "container", "", "", "stopped")
	f.asset(infra.NodeDevice, "loose1", "Drucker Brother", "printer", "", "192.168.178.60", "running", 1200, -400, true)
	f.asset(infra.NodeDevice, "new1", "Neuer Switch (unsortiert)", "switch", "", "192.168.178.99", "", 0, 0, false)
	f.asset(infra.NodeService, "new2", "Offline Wiki", "web-service", "", "", "", 0, 0, false)

	f.link("wan", "fritz", infra.EdgeFiber, "", "WAN", "1 Gbit/s")
	f.link("fritz", "sw", infra.EdgeEthernet, "LAN1", "Port 48", "1 Gbit/s")
	f.link("fritz", "ap", infra.EdgeEthernet, "LAN2", "eth0", "1 Gbit/s")
	f.link("ap", "mesh", infra.EdgeWiFi, "", "", "")
	f.link("ap", "ws", infra.EdgeWiFi, "", "", "")
	f.link("sw", "pve", infra.EdgeFiber, "SFP+ 1", "enp3s0", "10 Gbit/s")
	f.link("sw", "nas", infra.EdgeFiber, "SFP+ 2", "ix0", "10 Gbit/s")
	f.link("sw", "dock", infra.EdgeEthernet, "Port 12", "eno1", "2.5 Gbit/s")
	f.link("sw", "pi1", infra.EdgeEthernet, "Port 14", "eth0", "")
	f.link("sw", "pi2", infra.EdgeEthernet, "Port 15", "eth0", "")
	f.link("sw", "bak", infra.EdgeEthernet, "Port 20", "LAN 1", "1 Gbit/s")
	for _, id := range []string{"pve", "nas", "dock", "sw", "bak"} {
		f.link(id, "ups", infra.EdgePoweredBy, "", "", "")
	}
	f.link("ups", "pdu", infra.EdgePoweredBy, "", "", "")
	f.link("fritz", "cabinrouter", infra.EdgeFiber, "WAN", "WAN", "VPN")
	f.link("cabinrouter", "cabinnas", infra.EdgeEthernet, "LAN", "LAN 1", "1 Gbit/s")
	f.link("cabinrouter", "cabincam", infra.EdgeEthernet, "LAN", "eth0", "")
	f.link("fritz", "coloedge", infra.EdgeFiber, "WAN", "wg0", "WireGuard")
	f.link("coloedge", "colosw", infra.EdgeEthernet, "igb1", "Port 1", "10 Gbit/s")
	for i := 0; i < 4; i++ {
		f.link("colosw", fmt.Sprintf("srv%d", i), infra.EdgeEthernet, fmt.Sprintf("Port %d", i+2), "eno1", "10 Gbit/s")
	}
	for i := 0; i < 6; i++ {
		f.link("ap", fmt.Sprintf("iot%d", i), infra.EdgeWiFi, "", "", "")
	}
	f.link("vlan10", "sw", infra.EdgeMemberOf, "", "", "")
	f.link("vlan10", "pve", infra.EdgeMemberOf, "", "", "")
	f.link("vlan10", "dock", infra.EdgeMemberOf, "", "", "")
	f.link("c1", "c2", infra.EdgeDependsOn, "", "", "")
	f.link("vm1", "nas", infra.EdgeDependsOn, "", "NFS", "")
	f.link("c0", "nas", infra.EdgeDependsOn, "", "NFS", "")
	f.link("new1", "sw", infra.EdgeEthernet, "", "Port 30", "")
	f.link("loose1", "ap", infra.EdgeWiFi, "", "", "")
	f.link("ws", "sw", infra.EdgeEthernet, "enp5s0", "Port 40", "2.5 Gbit/s")
	return f.snapshot()
}
