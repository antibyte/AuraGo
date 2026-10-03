package server

import (
	"encoding/json"
)

type gameMakerPreviousRequestContext struct {
	Encoding string   `json:"encoding"`
	Texts    []string `json:"texts"`
	Sequence []int    `json:"sequence"`
}

// gameMakerPreviousRequests stores repeated historical prompt text once while
// retaining one sequence entry for every original request. Encoding explains
// how the model reconstructs the prompt list. The complete envelope, including
// that explanation, must be smaller than the legacy array before using it.
func gameMakerPreviousRequests(requests []string) any {
	if len(requests) < 2 {
		return requests
	}

	textIndexes := make(map[string]int, len(requests))
	compact := gameMakerPreviousRequestContext{
		Encoding: "sequence values are zero-based text indexes; original order and repeats retained",
		Texts:    make([]string, 0, len(requests)),
		Sequence: make([]int, len(requests)),
	}
	for i, request := range requests {
		index, ok := textIndexes[request]
		if !ok {
			index = len(compact.Texts)
			textIndexes[request] = index
			compact.Texts = append(compact.Texts, request)
		}
		compact.Sequence[i] = index
	}

	originalJSON, err := json.Marshal(requests)
	if err != nil {
		return requests
	}
	compactJSON, err := json.Marshal(compact)
	if err != nil || len(compactJSON) >= len(originalJSON) {
		return requests
	}
	return compact
}
