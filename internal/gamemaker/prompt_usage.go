package gamemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// PromptUsageCard is the result-card view of one job's prompt_usage events.
// Tokens and CostUSD stay nil when any call omitted that measurement.
// Cache tokens and legacy token_usage are not included.
type PromptUsageCard struct {
	Calls   int      `json:"calls"`
	Tokens  *int     `json:"tokens,omitempty"`
	CostUSD *float64 `json:"cost_usd,omitempty"`
}

func (s *Service) attachPromptUsage(job *Job) {
	if job == nil || strings.TrimSpace(job.ID) == "" {
		return
	}
	card, err := s.promptUsageCard(context.Background(), job.ID)
	if err != nil || card == nil {
		return
	}
	job.Usage = card
}

func (s *Service) promptUsageCard(ctx context.Context, jobID string) (*PromptUsageCard, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM gm_events WHERE job_id=? AND event_type='prompt_usage' ORDER BY id`, jobID)
	if err != nil {
		return nil, fmt.Errorf("read game maker prompt usage: %w", err)
	}
	defer rows.Close()
	var payloads []map[string]any
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("read game maker prompt usage: %w", err)
		}
		payloads = append(payloads, decodeEventPayload(raw))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read game maker prompt usage: %w", err)
	}
	return summarizePromptUsage(payloads), nil
}

// summarizePromptUsage sums measured input and output tokens only when every
// call reported both, and a price only when every call has a finite cost.
// A call with no events returns nil so the result card omits the line.
func summarizePromptUsage(payloads []map[string]any) *PromptUsageCard {
	if len(payloads) == 0 {
		return nil
	}
	card := &PromptUsageCard{Calls: len(payloads)}
	var tokens int64
	var cost float64
	tokensKnown := true
	costKnown := true
	for _, payload := range payloads {
		inN, inOK := promptUsageTokenInt(payload["input_tokens"])
		outN, outOK := promptUsageTokenInt(payload["output_tokens"])
		if !inOK || !outOK || tokens > 1_000_000_000-int64(inN)-int64(outN) {
			tokensKnown = false
		} else {
			tokens += int64(inN) + int64(outN)
		}
		price, ok := promptUsageCost(payload["estimated_cost_usd"])
		if !ok {
			costKnown = false
		} else {
			cost += price
		}
	}
	if tokensKnown {
		n := int(tokens)
		card.Tokens = &n
	}
	if costKnown {
		card.CostUSD = &cost
	}
	return card
}

func promptUsageTokenInt(value any) (int, bool) {
	const maxValue = 10_000_000
	var n int64
	switch value := value.(type) {
	case int:
		n = int64(value)
	case int32:
		n = int64(value)
	case int64:
		n = value
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > maxValue || value != math.Trunc(value) {
			return 0, false
		}
		n = int64(value)
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return 0, false
		}
		n = parsed
	default:
		return 0, false
	}
	if n < 0 || n > maxValue {
		return 0, false
	}
	return int(n), true
}

func promptUsageCost(value any) (float64, bool) {
	switch value := value.(type) {
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return 0, false
		}
		return value, true
	case int:
		if value < 0 {
			return 0, false
		}
		return float64(value), true
	case int64:
		if value < 0 {
			return 0, false
		}
		return float64(value), true
	case json.Number:
		parsed, err := value.Float64()
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
