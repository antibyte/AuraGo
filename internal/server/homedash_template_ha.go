package server

import (
	"context"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass"
)

const (
	homeDashTemplateHomeAssistant = "home_assistant"
	homeDashHATemplateName        = "Home Assistant"
	homeDashHAUnassignedTitle     = "Unassigned"
	homeDashHAWidgetsPerPage      = 24
	homeDashHASlugBase            = "home-assistant"
	homeDashHAPageIDMax           = 28 // contract section 5
	homeDashHAUnassignedID        = "unassigned"
)

// homeDashHADomainOrder is the widget order inside a page (decision D4).
var homeDashHADomainOrder = []string{"light", "switch", "input_boolean", "scene", "script", "button", "input_button",
	"select", "input_select", "number", "input_number"}

func homeDashAllowedCap(e homedash.Entity, c homedash.Capability) (homedash.CapabilitySpec, bool) {
	for _, s := range e.Capabilities {
		if s.Name == c && s.Allowed {
			return s, true
		}
	}
	return homedash.CapabilitySpec{}, false
}

func homeDashAnyCommandAllowed(e homedash.Entity) bool {
	for _, s := range e.Capabilities {
		if info, ok := homedash.LookupCapability(s.Name); ok && info.Command && s.Allowed {
			return true
		}
	}
	return false
}

// homeDashHAWidget picks the widget for one Home Assistant entity (decision D4). ok is false outside
// the W1 domains or without an allowed command capability. "Allowed" is the adapter's service policy
// (home_assistant.readonly, allowed/blocked services); HomeDash's own read-only switches do not count.
func homeDashHAWidget(e homedash.Entity) (typ, variant string, rank int, ok bool) {
	src, native, err := homedash.ParseEntityID(string(e.ID))
	if err != nil || src != hass.SourceID {
		return "", "", 0, false
	}
	domain, _, _ := strings.Cut(native, ".")
	rank = slices.Index(homeDashHADomainOrder, domain)
	if rank < 0 || !homeDashAnyCommandAllowed(e) {
		return "", "", 0, false
	}
	switch domain {
	case "light":
		if _, dim := homeDashAllowedCap(e, homedash.CapBrightness); dim {
			return "light", "slider", rank, true
		}
		return "light", "tile", rank, true
	case "switch", "input_boolean":
		return "switch", "tile", rank, true
	case "scene", "script", "button", "input_button":
		return "action", "button", rank, true
	case "select", "input_select":
		if spec, ok := homeDashAllowedCap(e, homedash.CapSelectOption); ok && len(spec.Options) <= 4 {
			return "select", "segmented", rank, true
		}
		return "select", "list", rank, true
	default: // number, input_number
		if mode, _ := e.State.Attributes["mode"].(string); mode == "box" {
			return "number", "stepper", rank, true
		}
		return "number", "slider", rank, true
	}
}

// homeDashHAPageID derives a page id from an area id: [a-z0-9_-], at most 28 characters, unique.
func homeDashHAPageID(key string, used map[string]bool) string {
	var b strings.Builder
	for _, r := range strings.ToLower(key) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
		if b.Len() >= homeDashHAPageIDMax {
			break
		}
	}
	id := strings.TrimLeft(b.String(), "-_")
	if id == "" {
		id = "area"
	}
	base := id
	for n := 2; used[id]; n++ {
		suffix := "-" + strconv.Itoa(n)
		keep := base
		if len(keep) > homeDashHAPageIDMax-len(suffix) {
			keep = keep[:homeDashHAPageIDMax-len(suffix)]
		}
		id = keep + suffix
	}
	used[id] = true
	return id
}

func homeDashHAPageTitle(area string, i int) string {
	t := homedash.CleanText(area, 40)
	if t == "" || homedash.ContainsURL(t) {
		t = "Area " + strconv.Itoa(i+1)
	}
	return t
}

// homeDashTemplateHA builds the Home Assistant dashboard (decision D4): one page per area sorted by
// area name, a last page for entities without area, widgets in domain order then name, at most
// maxPages pages, 24 widgets per page and maxWidgets in total. omitted counts qualifying entities the
// caps left out; ok is false when no entity qualifies. Layouts stay empty: the save pipeline's
// Autopack places the widgets with the catalog default sizes.
func homeDashTemplateHA(entities []homedash.Entity, name, unassigned string, maxPages, maxWidgets int) (homedash.Dashboard, int, bool) {
	type item struct {
		e            homedash.Entity
		typ, variant string
		rank         int
	}
	groups, titles := map[string][]item{}, map[string]string{}
	for _, e := range entities {
		typ, variant, rank, ok := homeDashHAWidget(e)
		if !ok {
			continue
		}
		groups[e.AreaID] = append(groups[e.AreaID], item{e, typ, variant, rank})
		if e.AreaID != "" && titles[e.AreaID] == "" {
			titles[e.AreaID] = e.Area
		}
	}
	if len(groups) == 0 {
		return homedash.Dashboard{}, 0, false
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := strings.ToLower(titles[keys[i]]), strings.ToLower(titles[keys[j]])
		if a != b {
			return a < b
		}
		return keys[i] < keys[j]
	})
	if _, ok := groups[""]; ok {
		keys = append(keys, "")
	}
	used := map[string]bool{homeDashHAUnassignedID: true} // reserved: only the last page may have it
	var pages []homedash.Page
	placed, omitted := 0, 0
	for i, key := range keys {
		items := groups[key]
		sort.Slice(items, func(x, y int) bool {
			if items[x].rank != items[y].rank {
				return items[x].rank < items[y].rank
			}
			a, b := strings.ToLower(items[x].e.Name), strings.ToLower(items[y].e.Name)
			if a != b {
				return a < b
			}
			return items[x].e.ID < items[y].e.ID
		})
		if len(pages) >= maxPages {
			omitted += len(items)
			continue
		}
		title, pageID := unassigned, homeDashHAUnassignedID
		if key != "" {
			title, pageID = homeDashHAPageTitle(titles[key], i), homeDashHAPageID(key, used)
		}
		compact := true
		page := homedash.Page{ID: pageID, Name: title, Compact: &compact}
		for j, it := range items {
			if j >= homeDashHAWidgetsPerPage || placed >= maxWidgets {
				omitted++
				continue
			}
			placed++
			page.Widgets = append(page.Widgets, homedash.Widget{ID: "w" + strconv.Itoa(placed), Type: it.typ, Variant: it.variant,
				Bindings: map[string]string{"primary": string(it.e.ID)}, Options: map[string]any{}})
		}
		if len(page.Widgets) > 0 {
			pages = append(pages, page)
		}
	}
	return homedash.Dashboard{
		Name: name,
		Look: homedash.Look{Family: "glass", Palette: homedash.Palette{Mode: "theme"}, Backdrop: homedash.Backdrop{Kind: "theme"},
			Density: "comfortable", RadiusScale: 1},
		Grid:  homedash.Grid{RowUnit: homedash.RowUnit{Auto: true}, Fit: "scroll"},
		Pages: pages,
	}, omitted, len(pages) > 0
}

// homeDashCheckTemplateText refuses an over-long text, control characters or a URL (contract section 5).
func homeDashCheckTemplateText(path, text string, max int) []homedash.Issue {
	switch {
	case utf8.RuneCountInString(text) > max:
		return []homedash.Issue{{Path: path, Code: homedash.IssueTooLong, Message: "at most " + strconv.Itoa(max) + " characters"}}
	case homedash.ContainsControl(text):
		return []homedash.Issue{{Path: path, Code: homedash.IssueControlChars, Message: "control characters are not allowed"}}
	case homedash.ContainsURL(text):
		return []homedash.Issue{{Path: path, Code: homedash.IssueURLNotAllowed, Message: "URLs are not allowed"}}
	}
	return nil
}

// homeAssistantTemplate builds the home_assistant template from the HA catalog (fetched when stale).
func (svc *homeDashService) homeAssistantTemplate(ctx context.Context, name, unassigned string) (homedash.Dashboard, int, error) {
	if name == "" {
		name = homeDashHATemplateName
	}
	if unassigned == "" {
		unassigned = homeDashHAUnassignedTitle
	}
	issues := append(homeDashCheckTemplateText("/name", name, 80), homeDashCheckTemplateText("/unassigned_title", unassigned, 40)...)
	if len(issues) > 0 {
		return homedash.Dashboard{}, 0, &homedash.Error{Code: homedash.CodeInvalidDocument, Message: "the dashboard name or unassigned title is not valid", Issues: issues}
	}
	cctx, cancel := context.WithTimeout(ctx, homeDashCatalogWait)
	defer cancel()
	list, status, active := svc.hub.SourceCatalog(cctx, hass.SourceID, true)
	if !active || (len(list) == 0 && status.State == homedash.StatusUnavailable) {
		return homedash.Dashboard{}, 0, homedash.SourceUnavailable(hass.SourceID, "Home Assistant is not available")
	}
	doc, omitted, ok := homeDashTemplateHA(list, name, unassigned, svc.cat.Limits.Pages, svc.cat.Limits.Widgets)
	if !ok {
		return homedash.Dashboard{}, 0, &homedash.Error{Code: homeDashCodeTemplateEmpty, Message: "Home Assistant has no entity this template can control"}
	}
	return doc, omitted, nil
}

// templates lists what bootstrap advertises (decision D4): system always, home_assistant when the
// cached HA catalog qualifies. A stale catalog is refreshed in the background, one refresh at a
// time, so bootstrap never waits for Home Assistant.
func (svc *homeDashService) templates() []string {
	out := []string{homeDashTemplateSystem}
	list, _, active := svc.hub.SourceCatalog(context.Background(), hass.SourceID, false)
	if !active {
		return out
	}
	if svc.haProbe.CompareAndSwap(false, true) {
		go func() {
			defer svc.haProbe.Store(false)
			parent := svc.ctx
			if parent == nil {
				parent = context.Background()
			}
			ctx, cancel := context.WithTimeout(parent, homeDashCatalogWait)
			defer cancel()
			svc.hub.SourceCatalog(ctx, hass.SourceID, true)
		}()
	}
	for _, e := range list {
		if _, _, _, ok := homeDashHAWidget(e); ok {
			return append(out, homeDashTemplateHomeAssistant)
		}
	}
	return out
}
