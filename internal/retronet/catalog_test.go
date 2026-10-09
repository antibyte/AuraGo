package retronet

import (
	"regexp"
	"strings"
	"testing"
)

// catalogSpec mirrors the spec table (docs/superpowers/specs/2026-10-09-retro-net-terminal-design.md)
// row by row: id, name, category, address, protocol, kind, charset, host key.
var catalogSpec = []struct {
	id, name, category, address string
	protocol                    Protocol
	kind                        Kind
	charset                     Charset
	hostKey                     string
}{
	{"telehack", "Telehack", "classics", "telehack.com:23", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"towel", "Star Wars ASCII", "classics", "towel.blinkenlights.nl:23", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"mapscii", "MapSCII", "classics", "mapscii.me:23", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"fics", "Free Internet Chess Server", "classics", "freechess.org:5000", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"telehack-ssh", "Telehack SSH", "classics", "telehack.com:2222", ProtocolSSH, "", "", "SHA256:PBE5P0vD6J9s3Exw/zWoZ4DGRla1ultRexuGF7Zm/ek"},
	{"vertrauen", "Vertrauen", "bbs", "vert.synchro.net:23", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"digital-distortion", "Digital Distortion", "bbs", "digitaldistortionbbs.com:23", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"diamond-mine", "Diamond Mine Online", "bbs", "sbbs.dmine.net:24", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"diamond-mine-wwiv", "Diamond Mine WWIV", "bbs", "wwiv.dmine.net:2323", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"retroboard", "RetroBoard", "bbs", "bbs.retroboardbbs.com:2323", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"futureland", "Futureland", "bbs", "futureland.today:23", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"vague", "Vague", "bbs", "vague.ddns.net:23", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"the-dungeon", "The Dungeon", "bbs", "dungeon.synchro.net:23", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"funtopia", "Funtopia", "bbs", "funtopia.synchro.net:3023", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"imzadi-box", "Imzadi Box", "bbs", "box.imzadi.de:23", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"redghost", "RedGhost", "bbs", "87.106.7.15:23", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"discworld", "Discworld MUD", "muds", "discworld.atuin.net:4242", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"aardwolf", "Aardwolf", "muds", "aardwolf.org:4000", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"batmud", "BatMUD", "muds", "bat.org:23", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"mume", "MUME", "muds", "mume.org:4242", ProtocolTelnet, KindWorld, CharsetLatin1, ""},
	{"genesis", "Genesis MUD", "muds", "mud.genesismud.org:3011", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"alter-aeon", "Alter Aeon", "muds", "alteraeon.com:3000", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"miriani", "Miriani", "muds", "toastsoft.net:1234", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"cosmic-rage", "Cosmic Rage", "muds", "cosmicrage.earth:7777", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"star-conquest", "Star Conquest", "muds", "moo.squidsoft.net:7777", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"furrymuck", "FurryMUCK", "muds", "furrymuck.com:8888", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"spindizzy", "SpinDizzy", "muds", "spindizzy.org:7072", ProtocolTelnet, KindWorld, CharsetUTF8, ""},
	{"sshtron", "SSHTron", "games", "sshtron.zachlatta.com:22", ProtocolSSH, "", "", "SHA256:cxRrWvXUmeIL3j/fJQclNpLj9sHYFX8cC5P+QhTILRA"},
	{"netris", "Netris", "games", "netris.rocketnine.space:22", ProtocolSSH, "", "", "SHA256:aOOORrmRW4l88GHLFfyFPI/dJFqpE/iwZs3soZHr5LY"},
	{"digital-highway", "Digital Highway", "games", "digitalhighway.com.se:1084", ProtocolTelnet, KindBBS, CharsetLatin1, ""},
	{"digicom", "Digicom", "games", "bbs.digicombbs.com:2323", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"digital-warfare", "Digital Warfare", "games", "bbs.digital-warfare.net:1337", ProtocolTelnet, KindBBS, CharsetCP437, ""},
	{"disconnected-by-peer", "Disconnected by Peer", "games", "bbs.disconnected-by-peer.at:23", ProtocolTelnet, KindBBS, CharsetCP437, ""},
}

func TestDefaultCatalogMatchesSpecTable(t *testing.T) {
	catalog := DefaultCatalog()
	if len(catalog) != 33 || len(catalogSpec) != 33 {
		t.Fatalf("catalog has %d entries (spec table %d), want 33", len(catalog), len(catalogSpec))
	}
	for i, want := range catalogSpec {
		got := catalog[i]
		if got.ID != want.id || got.Name != want.name || got.Category != want.category || got.Address() != want.address ||
			got.Protocol != want.protocol || got.Kind != want.kind || got.Charset != want.charset || got.HostKey != want.hostKey {
			t.Errorf("row %d = %+v, want %+v", i, got, want)
		}
	}
}

func TestDefaultCatalogInvariants(t *testing.T) {
	catalogIDPattern := regexp.MustCompile(`^[a-z0-9-]{2,40}$`)
	rank := map[string]int{CategoryClassics: 0, CategoryBBS: 1, CategoryMUDs: 2, CategoryGames: 3}
	seen := make(map[string]bool)
	lastRank := 0
	for _, e := range DefaultCatalog() {
		if seen[e.ID] {
			t.Errorf("duplicate catalog ID %q", e.ID)
		}
		seen[e.ID] = true
		if !catalogIDPattern.MatchString(e.ID) {
			t.Errorf("%s: ID does not match ^[a-z0-9-]{2,40}$", e.ID)
		}
		if strings.HasPrefix(e.ID, "own-") || ownIDPattern.MatchString(e.ID) {
			t.Errorf("%s: catalog IDs must never collide with own-entry IDs", e.ID)
		}
		if want := "desktop.terminal_retronet_entry_" + strings.ReplaceAll(e.ID, "-", "_"); e.DescriptionKey != want {
			t.Errorf("%s: DescriptionKey = %q, want %q", e.ID, e.DescriptionKey, want)
		}
		if e.Own || e.Description != "" || strings.TrimSpace(e.Name) == "" {
			t.Errorf("%s: catalog entries need a name, no free-text description and Own=false", e.ID)
		}
		r, ok := rank[e.Category]
		if !ok {
			t.Errorf("%s: unknown category %q", e.ID, e.Category)
		} else if r < lastRank {
			t.Errorf("%s: category %q breaks the render order classics, bbs, muds, games", e.ID, e.Category)
		} else {
			lastRank = r
		}
		if !validTarget(e) {
			t.Errorf("%s: host, port, protocol or protocol fields are invalid", e.ID)
		}
		switch e.Protocol {
		case ProtocolSSH:
			if e.User != "guest" || !hostKeyPattern.MatchString(e.HostKey) {
				t.Errorf("%s: SSH catalog entries need user guest and a pinned SHA256 host key", e.ID)
			}
		case ProtocolTelnet:
			if !e.Kind.valid() || !e.Charset.valid() {
				t.Errorf("%s: invalid kind %q or charset %q", e.ID, e.Kind, e.Charset)
			}
		}
	}
}

func TestDefaultCatalogReturnsFreshCopy(t *testing.T) {
	first := DefaultCatalog()
	first[0].Host = "evil.example"
	first[0].Port = 25
	second := DefaultCatalog()
	if second[0].Host != "telehack.com" || second[0].Port != 23 {
		t.Fatalf("DefaultCatalog shares state between calls: %+v", second[0])
	}
}

func TestLookup(t *testing.T) {
	catalog := DefaultCatalog()
	own := []Entry{
		{ID: "own-abcd1234", Name: "Mine", Category: CategoryOwn, Protocol: ProtocolTelnet, Host: "bbs.example.org", Port: 23, Kind: KindBBS, Charset: CharsetCP437, Own: true},
		{ID: "telehack", Name: "Shadow", Own: true},
	}
	if e, ok := Lookup(catalog, own, "netris"); !ok || e.Host != "netris.rocketnine.space" {
		t.Fatalf("Lookup(netris) = %+v, %v", e, ok)
	}
	if e, ok := Lookup(catalog, own, "own-abcd1234"); !ok || !e.Own || e.Name != "Mine" {
		t.Fatalf("Lookup(own) = %+v, %v", e, ok)
	}
	if e, ok := Lookup(catalog, own, "telehack"); !ok || e.Own || e.Name != "Telehack" {
		t.Fatalf("Lookup(telehack) = %+v, %v; catalog must win", e, ok)
	}
	for _, id := range []string{"", "local-shell", "own-missing00", "TELEHACK"} {
		if e, ok := Lookup(catalog, own, id); ok {
			t.Fatalf("Lookup(%q) = %+v, want not found", id, e)
		}
	}
	if _, ok := Lookup(nil, nil, "telehack"); ok {
		t.Fatal("Lookup with empty lists found an entry")
	}
}
