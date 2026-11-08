package consumers

import (
	"fmt"
	"time"

	"aurago/internal/decisions"
)

const (
	HelperTimeout          = 900 * time.Millisecond
	HelperQualificationTTL = 15 * time.Minute
	HelperRetryDelay       = 5 * time.Minute
	HelperSamples          = 5
)

var HelperProfiles = []ID{Router, HelperRAG, HelperTurn}

type HelperStatus struct {
	ID             ID      `json:"id"`
	Enabled        bool    `json:"enabled"`
	Available      bool    `json:"available"`
	Code           string  `json:"code"`
	Samples        int     `json:"samples"`
	ProbeCalls     int     `json:"probe_calls"`
	LastResponseMS float64 `json:"last_response_ms"`
	MaxResponseMS  float64 `json:"max_response_ms"`
	Evaluations    int     `json:"offloaded_evaluations"`
	AvoidedCalls   int     `json:"helper_calls_avoided"`
}

// HelperRequestFits bounds the complete request shape qualified by the worker.
func HelperRequestFits(id ID, r decisions.Request) bool {
	state, ok := r.State.(string)
	if !ok || len(r.Images) != 0 || len(r.Questions) == 0 {
		return false
	}
	limits := map[ID][2]int{Router: {24000, 1}, HelperRAG: {20000, 7}, HelperTurn: {12000, 4}}
	limit, ok := limits[id]
	return ok && len(state) <= limit[0] && len(r.Questions) <= limit[1]
}

// ValidateHelperResponse rejects partial or malformed answers before latency is accepted.
func ValidateHelperResponse(r decisions.Request, response decisions.Response) error {
	if len(response.Answers) != len(r.Questions) {
		return fmt.Errorf("incomplete helper decision response")
	}
	for _, q := range r.Questions {
		switch q.Type {
		case "noul":
			if _, ok := YesNo(response, q.ID); !ok {
				return fmt.Errorf("invalid yes/no answer")
			}
		case "score":
			if score, ok := Score(response, q.ID); !ok || score < 0 || score > 10 {
				return fmt.Errorf("invalid relevance score")
			}
		case "choice":
			if _, ok := Choice(response, q.ID, q.Options, 0, 0); !ok {
				return fmt.Errorf("invalid choice answer")
			}
		default:
			return fmt.Errorf("unsupported helper decision question")
		}
	}
	return nil
}
