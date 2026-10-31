package atlaspdf

import (
	"sort"
	"strconv"
	"strings"

	"aurago/internal/infra"
)

// defaultTexts holds the English text of every label the PDF can print. The
// translation keys match ui/lang/desktop/*.json.
var defaultTexts = map[string]string{
	"desktop.infra_pdf_title":             "Homelab map",
	"desktop.infra_pdf_assets":            "{count} assets",
	"desktop.infra_pdf_overview":          "Overview",
	"desktop.infra_pdf_part":              "Part {n}/{total}",
	"desktop.infra_pdf_unplaced":          "Not on the map",
	"desktop.infra_pdf_page":              "Page {n} / {total}",
	"desktop.infra_pdf_page_ref":          "p. {n}",
	"desktop.infra_pdf_generated":         "Generated {time}",
	"desktop.infra_pdf_stat_devices":      "Devices",
	"desktop.infra_pdf_stat_vms":          "VMs",
	"desktop.infra_pdf_stat_containers":   "Containers",
	"desktop.infra_pdf_stat_services":     "Services",
	"desktop.infra_pdf_stat_networks":     "Networks",
	"desktop.infra_pdf_stat_connections":  "Connections",
	"desktop.infra_pdf_stat_running":      "Running",
	"desktop.infra_pdf_legend":            "Legend",
	"desktop.infra_pdf_hosts_title":       "Hosts & guests",
	"desktop.infra_pdf_hostless":          "Without host",
	"desktop.infra_pdf_continued":         "continued",
	"desktop.infra_pdf_guests":            "{count} guests",
	"desktop.infra_pdf_inventory_title":   "Inventory",
	"desktop.infra_pdf_connections_title": "Connections",
	"desktop.infra_pdf_no_place":          "No place",
	"desktop.infra_pdf_col_name":          "Name",
	"desktop.infra_pdf_col_kind":          "Type",
	"desktop.infra_pdf_col_address":       "Address",
	"desktop.infra_pdf_col_mac":           "MAC",
	"desktop.infra_pdf_col_host":          "Host",
	"desktop.infra_pdf_col_system":        "System / model",
	"desktop.infra_pdf_col_sources":       "Sources",
	"desktop.infra_pdf_col_power":         "Power",
	"desktop.infra_pdf_col_from":          "From",
	"desktop.infra_pdf_col_to":            "To",
	"desktop.infra_pdf_col_port":          "Port",
	"desktop.infra_pdf_col_speed":         "Speed",
	"desktop.infra_pdf_col_label":         "Label",
	"desktop.infra_pdf_empty":             "No assets yet. Add devices in the Infra app to draw your map.",

	"desktop.infra_kind_device":    "Device",
	"desktop.infra_kind_vm":        "Virtual machine",
	"desktop.infra_kind_container": "Container",
	"desktop.infra_kind_service":   "Service",
	"desktop.infra_kind_network":   "Network",
	"desktop.infra_kind_site":      "Site",
	"desktop.infra_kind_room":      "Room",
	"desktop.infra_kind_rack":      "Rack",
	"desktop.infra_kind_group":     "Group",

	"desktop.infra_edge_ethernet":   "Ethernet",
	"desktop.infra_edge_fiber":      "Fiber",
	"desktop.infra_edge_wifi":       "Wi-Fi",
	"desktop.infra_edge_member_of":  "Member of",
	"desktop.infra_edge_runs_on":    "Runs on",
	"desktop.infra_edge_depends_on": "Depends on",
	"desktop.infra_edge_powered_by": "Powered by",

	"desktop.infra_power_running": "Running",
	"desktop.infra_power_stopped": "Stopped",
	"desktop.infra_power_paused":  "Paused",
	"desktop.infra_power_unknown": "Unknown",
}

// labels resolves label keys through the caller's translator and falls back to
// the English defaults. Placeholders such as {count} are substituted here so any
// translator works.
type labels struct {
	tr      func(string) string
	english bool
	cache   map[string]string
}

func newLabels(tr func(string) string) *labels {
	return &labels{tr: tr, cache: map[string]string{}}
}

// useEnglish switches every label to its English default.
func (l *labels) useEnglish() {
	l.english = true
	l.cache = map[string]string{}
}

// get returns the label for key; blank answers and answers equal to the key
// (the translator's "missing" convention) use the English default.
func (l *labels) get(key string) string {
	if v, ok := l.cache[key]; ok {
		return v
	}
	def, known := defaultTexts[key]
	if !known {
		def = key
	}
	v := def
	if !l.english && l.tr != nil {
		if t := strings.TrimSpace(l.tr(key)); t != "" && t != key {
			v = t
		}
	}
	l.cache[key] = v
	return v
}

// format resolves key and replaces {name} placeholders from the pairs
// (name, value, name, value...).
func (l *labels) format(key string, pairs ...string) string {
	s := l.get(key)
	for i := 0; i+1 < len(pairs); i += 2 {
		s = strings.ReplaceAll(s, "{"+pairs[i]+"}", pairs[i+1])
	}
	return s
}

func (l *labels) kind(k infra.NodeKind) string { return l.get("desktop.infra_kind_" + string(k)) }
func (l *labels) edge(k infra.EdgeKind) string { return l.get("desktop.infra_edge_" + string(k)) }
func (l *labels) power(p string) string        { return l.get("desktop.infra_power_" + p) }
func (l *labels) pageRef(n int) string {
	return l.format("desktop.infra_pdf_page_ref", "n", strconv.Itoa(n))
}
func (l *labels) part(n, total int) string {
	return l.format("desktop.infra_pdf_part", "n", strconv.Itoa(n), "total", strconv.Itoa(total))
}
func (l *labels) assets(n int) string {
	return l.format("desktop.infra_pdf_assets", "count", strconv.Itoa(n))
}
func (l *labels) guests(n int) string {
	return l.format("desktop.infra_pdf_guests", "count", strconv.Itoa(n))
}
func (l *labels) generated(t string) string {
	return l.format("desktop.infra_pdf_generated", "time", t)
}
func (l *labels) pageOf(n, total int) string {
	return l.format("desktop.infra_pdf_page", "n", strconv.Itoa(n), "total", strconv.Itoa(total))
}

// corpus is every label text in the current language, used to pick fonts.
func (l *labels) corpus() string {
	keys := make([]string, 0, len(defaultTexts))
	for k := range defaultTexts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(l.get(k))
		b.WriteByte(' ')
	}
	return b.String()
}
