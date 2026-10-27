package dockerstacks

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// portSpec is one published (or only exposed) port binding of a service, after variable expansion.
type portSpec struct {
	HostIP         string
	HostLo, HostHi int // 0 = not published (the engine picks a port)
	CtrLo, CtrHi   int
	Protocol       string // tcp | udp | sctp
}

func (p portSpec) hostText() string      { return rangeText(p.HostLo, p.HostHi) }
func (p portSpec) containerText() string { return rangeText(p.CtrLo, p.CtrHi) }

// fixedHostPorts is true when every host port is chosen by the file (no host range that the engine allocates from).
func (p portSpec) fixedHostPorts() bool {
	return p.HostLo != 0 && (p.HostHi-p.HostLo == p.CtrHi-p.CtrLo)
}

func rangeText(lo, hi int) string {
	switch {
	case lo == 0:
		return ""
	case lo == hi:
		return strconv.Itoa(lo)
	}
	return strconv.Itoa(lo) + "-" + strconv.Itoa(hi)
}

// parsePortRange reads "80" or "8000-8010" (1..65535, low <= high).
func parsePortRange(s string) (lo, hi int, err error) {
	a, b, isRange := strings.Cut(strings.TrimSpace(s), "-")
	if lo, err = parsePortNumber(a); err != nil {
		return 0, 0, err
	}
	if !isRange {
		return lo, lo, nil
	}
	if hi, err = parsePortNumber(b); err != nil {
		return 0, 0, err
	}
	if hi < lo {
		return 0, 0, fmt.Errorf("the range %q runs backwards", s)
	}
	return lo, hi, nil
}

func parsePortNumber(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || strconv.Itoa(n) != s {
		return 0, fmt.Errorf("%q is not a port number", s)
	}
	if n < 1 || n > 65535 {
		return 0, fmt.Errorf("port %d is outside 1-65535", n)
	}
	return n, nil
}

// newPortSpec assembles and validates a binding from its textual parts (short and long syntax share it).
// published may be empty (not published), container may be a range.
func newPortSpec(hostIP, published, container, protocol string) (portSpec, error) {
	p := portSpec{HostIP: strings.TrimSpace(hostIP), Protocol: strings.ToLower(strings.TrimSpace(protocol))}
	if p.Protocol == "" {
		p.Protocol = "tcp"
	}
	switch p.Protocol {
	case "tcp", "udp", "sctp":
	default:
		return portSpec{}, fmt.Errorf("unknown protocol %q (use tcp, udp or sctp)", protocol)
	}
	if p.HostIP != "" && net.ParseIP(p.HostIP) == nil {
		return portSpec{}, fmt.Errorf("%q is not an IP address", p.HostIP)
	}
	var err error
	if p.CtrLo, p.CtrHi, err = parsePortRange(container); err != nil {
		return portSpec{}, fmt.Errorf("container port: %w", err)
	}
	if strings.TrimSpace(published) != "" {
		if p.HostLo, p.HostHi, err = parsePortRange(published); err != nil {
			return portSpec{}, fmt.Errorf("published port: %w", err)
		}
		// A host range may feed a single container port (the engine picks one); otherwise both must match.
		if p.CtrHi != p.CtrLo && p.HostHi-p.HostLo != p.CtrHi-p.CtrLo {
			return portSpec{}, errors.New("the host and container port ranges differ in length")
		}
	}
	return p, nil
}

// parsePortSpec reads the short syntax [[HOST_IP:](HOST_PORT[-RANGE]):]CONTAINER_PORT[-RANGE][/PROTOCOL].
func parsePortSpec(raw string) (portSpec, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return portSpec{}, errors.New("the port is empty")
	}
	proto := ""
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		proto, s = s[i+1:], s[:i]
	}
	ip, rest := "", s
	bracketed := strings.HasPrefix(s, "[")
	if bracketed {
		end := strings.IndexByte(s, ']')
		if end < 0 || !strings.HasPrefix(s[end+1:], ":") {
			return portSpec{}, errors.New("an IPv6 address needs the form [::1]:HOST:CONTAINER")
		}
		ip, rest = s[1:end], s[end+2:]
	}
	parts := strings.Split(rest, ":")
	switch {
	case bracketed && len(parts) == 2: // [::1]:8080:80
		return newPortSpec(ip, parts[0], parts[1], proto)
	case bracketed:
		return portSpec{}, errors.New("an IPv6 address needs the form [::1]:HOST:CONTAINER")
	case len(parts) == 1:
		return newPortSpec("", "", parts[0], proto)
	case len(parts) == 2:
		return newPortSpec("", parts[0], parts[1], proto)
	case len(parts) == 3:
		return newPortSpec(parts[0], parts[1], parts[2], proto)
	}
	return portSpec{}, errors.New("too many ':' separated parts")
}

// isWildcardIP is true for the addresses that mean "every interface".
func isWildcardIP(ip string) bool { return ip == "" || ip == "0.0.0.0" || ip == "::" }

// ipsOverlap reports whether two bind addresses can both want the same host port.
func ipsOverlap(a, b string) bool { return isWildcardIP(a) || isWildcardIP(b) || a == b }

// volumeSpec is one storage entry of a service, after variable expansion.
type volumeSpec struct {
	Kind     string // named | bind | relative | anonymous | tmpfs
	Source   string
	Target   string
	ReadOnly bool
}

var volumeNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// volumeKind classifies a volume source the way Compose does: paths start with "/", "~", "." or a drive letter,
// everything else is a named volume.
func volumeKind(source string) (string, error) {
	switch {
	case source == "":
		return "anonymous", nil
	case strings.HasPrefix(source, "."):
		return "relative", nil
	case strings.HasPrefix(source, "/"), strings.HasPrefix(source, "~"), strings.HasPrefix(source, `\\`), isDrivePath(source):
		return "bind", nil
	case volumeNamePattern.MatchString(source):
		return "named", nil
	}
	return "", fmt.Errorf("%q is neither a path nor a volume name", source)
}

func isDrivePath(s string) bool {
	return len(s) >= 3 && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z')) && s[1] == ':' && (s[2] == '\\' || s[2] == '/')
}

// parseVolumeShort reads SOURCE:TARGET[:MODE], a lone TARGET (anonymous volume), including Windows drive paths.
func parseVolumeShort(raw string) (volumeSpec, error) {
	s := strings.TrimSpace(raw)
	parts := strings.Split(s, ":")
	if len(parts) > 1 && isDrivePath(parts[0]+":"+parts[1]) {
		parts = append([]string{parts[0] + ":" + parts[1]}, parts[2:]...)
	}
	var v volumeSpec
	switch len(parts) {
	case 1:
		v.Target = parts[0]
	case 2, 3:
		v.Source, v.Target = parts[0], parts[1]
		if len(parts) == 3 {
			for _, m := range strings.Split(parts[2], ",") {
				v.ReadOnly = v.ReadOnly || m == "ro"
			}
		}
		if v.Source == "" {
			return volumeSpec{}, errors.New("the source is empty")
		}
	default:
		return volumeSpec{}, errors.New("too many ':' separated parts")
	}
	if v.Target == "" {
		return volumeSpec{}, errors.New("the target is empty")
	}
	var err error
	v.Kind, err = volumeKind(v.Source)
	return v, err
}

// volumeFromLong builds a volumeSpec from the long syntax fields.
func volumeFromLong(typ, source, target string, readOnly bool) (volumeSpec, error) {
	if target == "" {
		return volumeSpec{}, errors.New("the target is empty")
	}
	v := volumeSpec{Source: source, Target: target, ReadOnly: readOnly}
	switch typ {
	case "tmpfs":
		v.Kind, v.Source = "tmpfs", ""
	case "bind", "npipe":
		kind, err := volumeKind(source)
		if err != nil || kind == "named" || kind == "anonymous" {
			if err == nil {
				err = errors.New("a bind mount needs a path as its source")
			}
			return volumeSpec{}, err
		}
		v.Kind = kind
	default: // "volume", "image", "cluster" or unset
		if source == "" {
			v.Kind = "anonymous"
		} else if volumeNamePattern.MatchString(source) {
			v.Kind = "named"
		} else {
			return volumeSpec{}, fmt.Errorf("%q is not a volume name", source)
		}
	}
	return v, nil
}
