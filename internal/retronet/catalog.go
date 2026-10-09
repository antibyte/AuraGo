package retronet

import "strings"

// descriptionKeyPrefix starts the i18n key of every catalog description.
const descriptionKeyPrefix = "desktop.terminal_retronet_entry_"

// catalogUser is the login name of the anonymous SSH catalog entries.
const catalogUser = "guest"

// catalogEntries holds the curated services in display order. The live
// acceptance test (AURAGO_RETRONET_LIVE=1) is the authority for kind and
// charset; correct a row or remove it when a service renders wrongly.
var catalogEntries = []Entry{
	telnetEntry("telehack", "Telehack", CategoryClassics, "telehack.com", 23, KindWorld, CharsetUTF8),
	telnetEntry("towel", "Star Wars ASCII", CategoryClassics, "towel.blinkenlights.nl", 23, KindWorld, CharsetUTF8),
	telnetEntry("mapscii", "MapSCII", CategoryClassics, "mapscii.me", 23, KindWorld, CharsetUTF8),
	telnetEntry("fics", "Free Internet Chess Server", CategoryClassics, "freechess.org", 5000, KindWorld, CharsetUTF8),
	sshEntry("telehack-ssh", "Telehack SSH", CategoryClassics, "telehack.com", 2222, "SHA256:PBE5P0vD6J9s3Exw/zWoZ4DGRla1ultRexuGF7Zm/ek"),

	telnetEntry("vertrauen", "Vertrauen", CategoryBBS, "vert.synchro.net", 23, KindBBS, CharsetCP437),
	telnetEntry("digital-distortion", "Digital Distortion", CategoryBBS, "digitaldistortionbbs.com", 23, KindBBS, CharsetCP437),
	telnetEntry("diamond-mine", "Diamond Mine Online", CategoryBBS, "sbbs.dmine.net", 24, KindBBS, CharsetCP437),
	telnetEntry("diamond-mine-wwiv", "Diamond Mine WWIV", CategoryBBS, "wwiv.dmine.net", 2323, KindBBS, CharsetCP437),
	telnetEntry("retroboard", "RetroBoard", CategoryBBS, "bbs.retroboardbbs.com", 2323, KindBBS, CharsetCP437),
	telnetEntry("futureland", "Futureland", CategoryBBS, "futureland.today", 23, KindBBS, CharsetCP437),
	telnetEntry("vague", "Vague", CategoryBBS, "vague.ddns.net", 23, KindBBS, CharsetCP437),
	telnetEntry("the-dungeon", "The Dungeon", CategoryBBS, "dungeon.synchro.net", 23, KindBBS, CharsetCP437),
	telnetEntry("funtopia", "Funtopia", CategoryBBS, "funtopia.synchro.net", 3023, KindBBS, CharsetCP437),
	telnetEntry("imzadi-box", "Imzadi Box", CategoryBBS, "box.imzadi.de", 23, KindBBS, CharsetCP437),
	telnetEntry("redghost", "RedGhost", CategoryBBS, "87.106.7.15", 23, KindBBS, CharsetCP437),

	telnetEntry("discworld", "Discworld MUD", CategoryMUDs, "discworld.atuin.net", 4242, KindWorld, CharsetUTF8),
	telnetEntry("aardwolf", "Aardwolf", CategoryMUDs, "aardwolf.org", 4000, KindWorld, CharsetUTF8),
	telnetEntry("batmud", "BatMUD", CategoryMUDs, "bat.org", 23, KindWorld, CharsetUTF8),
	telnetEntry("mume", "MUME", CategoryMUDs, "mume.org", 4242, KindWorld, CharsetLatin1),
	telnetEntry("genesis", "Genesis MUD", CategoryMUDs, "mud.genesismud.org", 3011, KindWorld, CharsetUTF8),
	telnetEntry("alter-aeon", "Alter Aeon", CategoryMUDs, "alteraeon.com", 3000, KindWorld, CharsetUTF8),
	telnetEntry("miriani", "Miriani", CategoryMUDs, "toastsoft.net", 1234, KindWorld, CharsetUTF8),
	telnetEntry("cosmic-rage", "Cosmic Rage", CategoryMUDs, "cosmicrage.earth", 7777, KindWorld, CharsetUTF8),
	telnetEntry("star-conquest", "Star Conquest", CategoryMUDs, "moo.squidsoft.net", 7777, KindWorld, CharsetUTF8),
	telnetEntry("furrymuck", "FurryMUCK", CategoryMUDs, "furrymuck.com", 8888, KindWorld, CharsetUTF8),
	telnetEntry("spindizzy", "SpinDizzy", CategoryMUDs, "spindizzy.org", 7072, KindWorld, CharsetUTF8),

	sshEntry("sshtron", "SSHTron", CategoryGames, "sshtron.zachlatta.com", 22, "SHA256:cxRrWvXUmeIL3j/fJQclNpLj9sHYFX8cC5P+QhTILRA"),
	sshEntry("netris", "Netris", CategoryGames, "netris.rocketnine.space", 22, "SHA256:aOOORrmRW4l88GHLFfyFPI/dJFqpE/iwZs3soZHr5LY"),
	telnetEntry("digital-highway", "Digital Highway", CategoryGames, "digitalhighway.com.se", 1084, KindBBS, CharsetLatin1),
	telnetEntry("digicom", "Digicom", CategoryGames, "bbs.digicombbs.com", 2323, KindBBS, CharsetCP437),
	telnetEntry("digital-warfare", "Digital Warfare", CategoryGames, "bbs.digital-warfare.net", 1337, KindBBS, CharsetCP437),
	telnetEntry("disconnected-by-peer", "Disconnected by Peer", CategoryGames, "bbs.disconnected-by-peer.at", 23, KindBBS, CharsetCP437),
}

// DefaultCatalog returns a fresh copy of the 33 curated entries in display order.
func DefaultCatalog() []Entry {
	out := make([]Entry, len(catalogEntries))
	copy(out, catalogEntries)
	return out
}

// Lookup finds id among catalog then own entries.
func Lookup(catalog, own []Entry, id string) (Entry, bool) {
	for _, list := range [][]Entry{catalog, own} {
		for _, e := range list {
			if e.ID == id {
				return e, true
			}
		}
	}
	return Entry{}, false
}

// descriptionKey returns the i18n key of a catalog entry description.
func descriptionKey(id string) string {
	return descriptionKeyPrefix + strings.ReplaceAll(id, "-", "_")
}

func telnetEntry(id, name, category, host string, port int, kind Kind, charset Charset) Entry {
	return Entry{
		ID:             id,
		Name:           name,
		DescriptionKey: descriptionKey(id),
		Category:       category,
		Protocol:       ProtocolTelnet,
		Host:           host,
		Port:           port,
		Kind:           kind,
		Charset:        charset,
	}
}

func sshEntry(id, name, category, host string, port int, hostKey string) Entry {
	return Entry{
		ID:             id,
		Name:           name,
		DescriptionKey: descriptionKey(id),
		Category:       category,
		Protocol:       ProtocolSSH,
		Host:           host,
		Port:           port,
		User:           catalogUser,
		HostKey:        hostKey,
	}
}
