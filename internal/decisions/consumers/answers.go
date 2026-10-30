package consumers

import (
	"math"

	"aurago/internal/decisions"
)

// Choice returns the selected option ID only for a well-formed, normalized and
// confident answer. Every caller keeps today's behaviour when ok is false.
func Choice(r decisions.Response, qid string, options []decisions.Option, minP, minMargin float64) (string, bool) {
	answer, found := r.Answers[qid]
	if !found || len(answer.Probabilities) != len(options) {
		return "", false
	}
	total, selectedP, second, known := 0.0, -1.0, 0.0, false
	for _, option := range options {
		p, exists := answer.Probabilities[option.ID]
		if !exists || math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
			return "", false
		}
		total += p
		if option.ID == answer.Decision {
			selectedP, known = p, true
		} else {
			second = max(second, p)
		}
	}
	if !known || math.Abs(total-1) > 1e-6 || selectedP < minP || selectedP-second < minMargin-1e-9 {
		return "", false
	}
	return answer.Decision, true
}

// YesNo returns the probability of yes.
func YesNo(r decisions.Response, qid string) (float64, bool) {
	answer, found := r.Answers[qid]
	if !found || answer.Noul == nil {
		return 0, false
	}
	yes, no := answer.Probabilities["yes"], answer.Probabilities["no"]
	p := *answer.Noul
	if math.IsNaN(p) || p < 0 || p > 1 || math.Abs(yes+no-1) > 1e-6 || math.Abs(yes-p) > 1e-6 {
		return 0, false
	}
	return p, true
}

// Score returns the probability-weighted value of a score question.
func Score(r decisions.Response, qid string) (float64, bool) {
	answer, found := r.Answers[qid]
	if !found || answer.Score == nil || math.IsNaN(*answer.Score) || math.IsInf(*answer.Score, 0) {
		return 0, false
	}
	return *answer.Score, true
}
