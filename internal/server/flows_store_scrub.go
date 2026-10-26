package server

import (
	"encoding/json"

	"aurago/internal/flows"
)

// flowStoreScrubber is what the flow store runs before it writes run data (flows.Store.SetScrubber,
// S7): the same value walk the API runs on every read (scrubFlowValue, scrubFlowText), so a
// registered secret is redacted in flows.db as well.
func flowStoreScrubber() flows.Scrubber {
	return flows.Scrubber{
		JSON: func(data []byte) ([]byte, error) {
			var plain any
			if err := json.Unmarshal(data, &plain); err != nil {
				return nil, err
			}
			return json.Marshal(scrubFlowValue(plain))
		},
		Text: scrubFlowText,
	}
}
