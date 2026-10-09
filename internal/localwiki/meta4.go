package localwiki

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	meta4BodyLimit = 8 << 20

	// maxEditionBytes bounds the size a metalink or a state file may claim.
	// The largest Kiwix Wikipedia edition is about 127 GB; anything beyond a
	// terabyte is hostile or corrupt, and the disk-space math must never see it.
	maxEditionBytes int64 = 1 << 40

	// maxMirrors bounds how many mirror URLs one edition keeps.
	maxMirrors = 16

	// defaultMirrorPriority sorts mirrors without a usable priority last.
	defaultMirrorPriority int64 = 1 << 20
)

// zimFileNamePattern is the only file name an edition may have on disk.
var zimFileNamePattern = regexp.MustCompile(`^wikipedia_[a-z]{2,3}_all_(maxi|nopic)_\d{4}-\d{2}[a-z]?\.zim$`)

type meta4File struct {
	FileName string
	Size     int64
	SHA256   string
	URLs     []string
}

type metalinkDoc struct {
	Files []metalinkFile `xml:"file"`
}

type metalinkFile struct {
	Name   string `xml:"name,attr"`
	Size   int64  `xml:"size"`
	Hashes []struct {
		Type  string `xml:"type,attr"`
		Value string `xml:",chardata"`
	} `xml:"hash"`
	URLs []struct {
		// Priority is parsed by hand: one unusable value must not abort the
		// whole metalink (and a 32-bit int cannot hold every valid one).
		Priority string `xml:"priority,attr"`
		Value    string `xml:",chardata"`
	} `xml:"url"`
}

// parseMeta4 reads the RFC 5854 metalink Kiwix publishes for an edition: exact
// size (at most maxEditionBytes), SHA-256 and the mirror URLs.
//
// A mirror is kept when it is an HTTPS URL without credentials whose path ends
// in the edition's file name and whose host is not a loopback, private,
// link-local or otherwise local address. Hosts of the trusted URLs are exempt
// from the local-address rule: the manager passes the catalog URL, which tests
// point at a local fake Kiwix. Mirrors are de-duplicated, ordered by ascending
// priority and capped at maxMirrors. Conflicting SHA-256 values and a file
// listed twice are rejected.
func parseMeta4(data []byte, wantFile string, trusted ...*url.URL) (meta4File, error) {
	var doc metalinkDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return meta4File{}, fmt.Errorf("parse meta4: %w", err)
	}
	var file *metalinkFile
	for i := range doc.Files {
		if strings.TrimSpace(doc.Files[i].Name) != wantFile {
			continue
		}
		if file != nil {
			return meta4File{}, fmt.Errorf("meta4 lists %s more than once", wantFile)
		}
		file = &doc.Files[i]
	}
	if file == nil {
		return meta4File{}, fmt.Errorf("meta4 does not describe %s", wantFile)
	}
	name := wantFile
	if !zimFileNamePattern.MatchString(name) {
		return meta4File{}, fmt.Errorf("meta4 names an unexpected file %q", name)
	}
	if file.Size <= 0 {
		return meta4File{}, fmt.Errorf("meta4 for %s has no size", name)
	}
	if file.Size > maxEditionBytes {
		return meta4File{}, fmt.Errorf("meta4 for %s claims an implausible size of %d bytes", name, file.Size)
	}
	out := meta4File{FileName: name, Size: file.Size}
	for _, hash := range file.Hashes {
		if !strings.EqualFold(strings.TrimSpace(hash.Type), "sha-256") {
			continue
		}
		value := strings.ToLower(strings.TrimSpace(hash.Value))
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != 32 {
			continue
		}
		if out.SHA256 != "" && out.SHA256 != value {
			return meta4File{}, fmt.Errorf("meta4 for %s lists conflicting SHA-256 values", name)
		}
		out.SHA256 = value
	}
	if out.SHA256 == "" {
		return meta4File{}, fmt.Errorf("meta4 for %s has no SHA-256", name)
	}
	type mirror struct {
		priority int64
		url      string
	}
	var mirrors []mirror
	seen := make(map[string]bool)
	for _, candidate := range file.URLs {
		value := strings.TrimSpace(candidate.Value)
		if seen[value] || checkMirrorURL(value, name, trusted) != nil {
			continue
		}
		seen[value] = true
		priority, err := strconv.ParseInt(strings.TrimSpace(candidate.Priority), 10, 64)
		if err != nil || priority <= 0 {
			priority = defaultMirrorPriority
		}
		mirrors = append(mirrors, mirror{priority: priority, url: value})
	}
	sort.SliceStable(mirrors, func(i, j int) bool { return mirrors[i].priority < mirrors[j].priority })
	if len(mirrors) > maxMirrors {
		mirrors = mirrors[:maxMirrors]
	}
	for _, m := range mirrors {
		out.URLs = append(out.URLs, m.url)
	}
	return out, nil
}

// checkMirrorURL accepts a download URL for fileName: HTTPS without
// credentials, a path ending in "/<fileName>" and a public host. Hosts of the
// trusted URLs may be local addresses (see parseMeta4).
func checkMirrorURL(raw, fileName string, trusted []*url.URL) error {
	parsed, err := requireHTTPS(raw)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(parsed.Path, "/"+fileName) {
		return fmt.Errorf("localwiki: mirror %q does not serve %s", raw, fileName)
	}
	if !isLocalHost(parsed.Hostname()) || isTrustedHost(parsed, trusted) {
		return nil
	}
	return fmt.Errorf("localwiki: mirror %q is a local address", raw)
}

// carrierGradeNAT is 100.64.0.0/10 (RFC 6598), which netip does not count as private.
var carrierGradeNAT = netip.MustParsePrefix("100.64.0.0/10")

// thisNetwork is 0.0.0.0/8: "this host" in many stacks.
var thisNetwork = netip.MustParsePrefix("0.0.0.0/8")

// siteLocal is fec0::/10, the deprecated IPv6 site-local range (RFC 3879).
var siteLocal = netip.MustParsePrefix("fec0::/10")

// localUseNAT64 is 64:ff9b:1::/48, the local-use IPv4/IPv6 translation prefix
// (RFC 8215). Where the IPv4 address sits depends on the prefix length a site
// chose, so the whole range counts as local.
var localUseNAT64 = netip.MustParsePrefix("64:ff9b:1::/48")

// IPv6 ranges that carry an IPv4 address: IPv4-compatible (::a.b.c.d),
// IPv4-translated (::ffff:0:a.b.c.d, RFC 2765) and NAT64 (64:ff9b::/96) in
// the last 32 bits, and 6to4 (2002::/16) in bits 16 to 47. IPv4-mapped
// addresses (::ffff:a.b.c.d) are unmapped first.
var (
	ipv4Compatible = netip.MustParsePrefix("::/96")
	ipv4Translated = netip.MustParsePrefix("::ffff:0:0:0/96")
	nat64          = netip.MustParsePrefix("64:ff9b::/96")
	sixToFour      = netip.MustParsePrefix("2002::/16")
)

// isLocalHost reports hosts a public mirror never has: localhost names, IP
// literals in loopback, private, link-local, site-local, multicast,
// unspecified or carrier-grade NAT ranges (also when embedded in an
// IPv4-mapped, IPv4-compatible, IPv4-translated, NAT64 or 6to4 IPv6 address),
// the local-use NAT64 range 64:ff9b:1::/48, and numeric spellings of IPv4
// addresses ("2130706433", "0x7f.1") that resolvers expand but netip does not
// parse. It is purely lexical; the default HTTP client repeats it on the
// resolved address of every connection (see dialGuard).
func isLocalHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return isLocalAddr(addr)
	}
	last := host[strings.LastIndex(host, ".")+1:]
	return strings.HasPrefix(last, "0x") || strings.Trim(last, "0123456789") == ""
}

func isLocalAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsMulticast() ||
		addr.IsUnspecified() || carrierGradeNAT.Contains(addr) || thisNetwork.Contains(addr) {
		return true
	}
	if !addr.Is6() {
		return false
	}
	if siteLocal.Contains(addr) || localUseNAT64.Contains(addr) {
		return true
	}
	b := addr.As16()
	switch {
	case ipv4Compatible.Contains(addr), ipv4Translated.Contains(addr), nat64.Contains(addr):
		return isLocalAddr(netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}))
	case sixToFour.Contains(addr):
		return isLocalAddr(netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]}))
	}
	return false
}
