package hass

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// historyDomains have a numeric or on/off recorder history.
var historyDomains = map[string]bool{"sensor": true, "number": true, "input_number": true, "switch": true, "input_boolean": true, "binary_sensor": true}

const maxHistorySamples = 20000

func historyValue(s string) (float64, bool) {
	switch s {
	case "on":
		return 1, true
	case "off":
		return 0, true
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil && finite(f)
}

// History reads Home Assistant's recorder for one entity (design-backend 8.1): numeric states as
// numbers, on/off as 1/0, other states skipped. Domains without such a history answer an empty
// series without a request. The hub downsamples to points. The endpoint is built only from the
// validated native id and times formatted here (RFC 3339 UTC); no lock is held during the request.
func (a *Adapter) History(ctx context.Context, native string, from, to time.Time, points int) ([]homedash.Sample, error) {
	out := []homedash.Sample{}
	domain, ok := domainOf(native)
	if !ok {
		return nil, errors.New("hass: invalid entity id")
	}
	if !historyDomains[domain] || !to.After(from) || to.Sub(from) > homedash.MaxHistoryRange || points < 1 {
		return out, nil
	}
	endpoint := "/api/history/period/" + from.UTC().Format(time.RFC3339) +
		"?filter_entity_id=" + url.QueryEscape(native) +
		"&end_time=" + url.QueryEscape(to.UTC().Format(time.RFC3339)) +
		"&minimal_response&no_attributes"
	var series [][]struct {
		State       string    `json:"state"`
		LastChanged time.Time `json:"last_changed"`
		LastUpdated time.Time `json:"last_updated"`
	}
	if err := tools.HARequestJSON(ctx, a.ha, http.MethodGet, endpoint, nil, &series); err != nil {
		a.log.Debug("homedash: Home Assistant history failed", "entity", native, "error", err)
		return nil, fmt.Errorf("hass history: %w", err)
	}
	for _, list := range series {
		for _, p := range list {
			v, ok := historyValue(p.State)
			at := p.LastChanged
			if at.IsZero() {
				at = p.LastUpdated
			}
			if !ok || at.IsZero() {
				continue
			}
			out = append(out, homedash.Sample{T: at.UTC(), V: v})
			if len(out) >= maxHistorySamples {
				break
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T.Before(out[j].T) })
	return out, nil
}
