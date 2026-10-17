package infra

import (
	"fmt"
	"math"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	iconPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	customKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
)

func validateNode(node *Node) error {
	normalizeNodeRuntime(node)
	if !validNodeKind(node.Kind) {
		return fmt.Errorf("%w: invalid node kind", ErrValidation)
	}
	if err := validateText("node name", node.Name, 200, true); err != nil {
		return err
	}
	node.Name = strings.TrimSpace(node.Name)
	node.ParentID = strings.TrimSpace(node.ParentID)
	if err := validateIcon(node.Icon); err != nil {
		return err
	}
	if err := validateText("parent id", node.ParentID, 128, false); err != nil {
		return err
	}
	if err := validateProfile(&node.Profile); err != nil {
		return err
	}
	if node.Placements == nil {
		node.Placements = Placements{}
	}
	if err := validatePlacements(node.Placements); err != nil {
		return err
	}
	if node.Sources == nil {
		node.Sources = []Source{}
	}
	return nil
}

func normalizeNodeRuntime(node *Node) {
	node.AvailableActions = []string{}
	node.AppLinks = []AppLink{}
	node.Power = ""
	node.ActionReasons = nil
	node.SourceActions = nil
	node.SourceActionReasons = nil
}

func validateProfile(profile *Profile) error {
	for name, value := range map[string]string{
		"role": profile.Role, "vendor": profile.Vendor, "model": profile.Model, "serial": profile.Serial,
		"os": profile.OS, "firmware": profile.Firmware, "cpu": profile.CPU, "ram": profile.RAM,
		"storage": profile.Storage, "rack position": profile.RackPosition,
	} {
		if err := validateText(name, value, 256, false); err != nil {
			return err
		}
	}
	if err := validateIP("ip address", profile.IPAddress); err != nil {
		return err
	}
	if err := validateSegmentFields(profile.CIDR, profile.VLAN); err != nil {
		return err
	}
	if profile.MACAddress != "" {
		if _, err := net.ParseMAC(profile.MACAddress); err != nil {
			return fmt.Errorf("%w: invalid MAC address", ErrValidation)
		}
	}
	if len(profile.Notes) > 8192 || !utf8.ValidString(profile.Notes) || strings.ContainsRune(profile.Notes, '\x00') {
		return fmt.Errorf("%w: notes exceed 8192 bytes or contain invalid text", ErrValidation)
	}
	if err := validateTags(profile.Tags); err != nil {
		return err
	}
	if err := validateInterfaces(profile.Interfaces, false); err != nil {
		return err
	}
	if len(profile.Links) > 32 {
		return fmt.Errorf("%w: too many links", ErrValidation)
	}
	for _, link := range profile.Links {
		if err := validateText("link label", link.Label, 128, true); err != nil {
			return err
		}
		if len(link.URL) > 2048 {
			return fmt.Errorf("%w: link URL is too long", ErrValidation)
		}
		u, err := url.Parse(link.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Opaque != "" {
			return fmt.Errorf("%w: links must be absolute HTTP or HTTPS URLs without user information", ErrValidation)
		}
	}
	if len(profile.AppBindings) > 64 {
		return fmt.Errorf("%w: too many application bindings", ErrValidation)
	}
	appBindings := make(map[AppLink]struct{}, len(profile.AppBindings))
	for _, binding := range profile.AppBindings {
		if !customKeyPattern.MatchString(binding.App) {
			return fmt.Errorf("%w: invalid application binding app", ErrValidation)
		}
		if err := validateText("application binding target", binding.TargetID, 128, true); err != nil {
			return err
		}
		if _, exists := appBindings[binding]; exists {
			return fmt.Errorf("%w: duplicate application binding", ErrValidation)
		}
		appBindings[binding] = struct{}{}
	}
	if len(profile.Custom) > 64 {
		return fmt.Errorf("%w: too many custom fields", ErrValidation)
	}
	for key, value := range profile.Custom {
		if !customKeyPattern.MatchString(key) || len(value) > 2048 || !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("%w: invalid custom field", ErrValidation)
		}
	}
	if profile.Tags == nil {
		profile.Tags = []string{}
	}
	if profile.Interfaces == nil {
		profile.Interfaces = []NetworkInterface{}
	}
	if profile.Links == nil {
		profile.Links = []Link{}
	}
	if profile.AppBindings == nil {
		profile.AppBindings = []AppLink{}
	}
	if profile.Custom == nil {
		profile.Custom = map[string]string{}
	}
	return nil
}

func validateTags(tags []string) error {
	if len(tags) > 64 {
		return fmt.Errorf("%w: too many tags", ErrValidation)
	}
	for _, tag := range tags {
		if err := validateText("tag", tag, 64, true); err != nil {
			return err
		}
	}
	return nil
}

func validateInterfaces(interfaces []NetworkInterface, forbidMAC bool) error {
	if len(interfaces) > 32 {
		return fmt.Errorf("%w: too many network interfaces", ErrValidation)
	}
	for _, iface := range interfaces {
		if err := validateText("interface name", iface.Name, 64, true); err != nil {
			return err
		}
		if err := validateIP("interface ip address", iface.IPAddress); err != nil {
			return err
		}
		if iface.MACAddress != "" {
			if forbidMAC {
				return fmt.Errorf("%w: Fritz!Box MAC addresses must stay in the source adapter", ErrValidation)
			}
			if _, err := net.ParseMAC(iface.MACAddress); err != nil {
				return fmt.Errorf("%w: invalid interface MAC address", ErrValidation)
			}
		}
		for name, value := range map[string]string{"VLAN": iface.VLAN, "speed": iface.Speed} {
			if err := validateText(name, value, 64, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePlacements(placements Placements) error {
	for view, placement := range placements {
		if view != ViewAtlas && view != ViewTopology {
			return fmt.Errorf("%w: unsupported placement view", ErrValidation)
		}
		if !validCoordinate(placement.X) || !validCoordinate(placement.Y) {
			return fmt.Errorf("%w: placement coordinates must be finite and within bounds", ErrValidation)
		}
	}
	if placements == nil {
		return fmt.Errorf("%w: placements must be an object", ErrValidation)
	}
	return nil
}

func validateEdge(edge *Edge) error {
	if err := validateText("from node id", edge.From, 128, true); err != nil {
		return err
	}
	if err := validateText("to node id", edge.To, 128, true); err != nil {
		return err
	}
	switch edge.Kind {
	case EdgeEthernet, EdgeFiber, EdgeWiFi, EdgeMemberOf, EdgeRunsOn, EdgeDependsOn, EdgePoweredBy:
	default:
		return fmt.Errorf("%w: invalid edge kind", ErrValidation)
	}
	for name, value := range map[string]string{"edge label": edge.Label, "from port": edge.FromPort, "to port": edge.ToPort, "edge speed": edge.Speed} {
		if err := validateText(name, value, 128, false); err != nil {
			return err
		}
	}
	return nil
}

func validateText(name, value string, maxRunes int, required bool) error {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxRunes {
		return fmt.Errorf("%w: invalid or oversized %s", ErrValidation, name)
	}
	value = strings.TrimSpace(value)
	if required && value == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, name)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: %s contains control characters", ErrValidation, name)
		}
	}
	return nil
}

func validateIcon(icon string) error {
	if !iconPattern.MatchString(icon) {
		return fmt.Errorf("%w: invalid icon key", ErrValidation)
	}
	return nil
}

func validateIP(name, value string) error {
	if value == "" {
		return nil
	}
	if len(value) > 64 || net.ParseIP(value) == nil {
		return fmt.Errorf("%w: invalid %s", ErrValidation, name)
	}
	return nil
}

func validateSegmentFields(cidr string, vlan int) error {
	if cidr != "" {
		if err := validateText("network CIDR", cidr, 64, false); err != nil {
			return err
		}
		if _, err := netip.ParsePrefix(cidr); err != nil {
			return fmt.Errorf("%w: invalid network CIDR", ErrValidation)
		}
	}
	if vlan < 0 || vlan > 4094 {
		return fmt.Errorf("%w: VLAN must be between 1 and 4094, or zero when unknown", ErrValidation)
	}
	return nil
}

func validNodeKind(kind NodeKind) bool {
	switch kind {
	case NodeDevice, NodeVM, NodeContainer, NodeService, NodeNetwork, NodeSite, NodeRoom, NodeRack, NodeGroup:
		return true
	default:
		return false
	}
}

func validCoordinate(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -100000 && value <= 100000
}
