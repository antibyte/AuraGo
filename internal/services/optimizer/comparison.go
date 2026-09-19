package optimizer

import "context"

// Compare matched source revisions, canonical actions and operations only. Each
// exposure contributes one observation per action/operation; both need evidence.
func (w *OptimizerWorker) comparableRates(ctx context.Context, manual, candidate, baseline string) (float64, float64, bool) {
	rows, err := w.db.db.QueryContext(ctx, `SELECT t.prompt_version,t.action_identity,t.operation,e.source_revision,t.success
	 FROM tool_traces t JOIN prompt_exposures e ON e.id=t.exposure_id
	 WHERE t.tool_name=? AND t.action_identity<>'' AND t.prompt_version IN (?,?) AND t.timestamp>datetime('now','-7 days')
	 ORDER BY t.id DESC LIMIT 500`, manual, candidate, baseline)
	if err != nil {
		return 0, 0, false
	}
	defer rows.Close()
	type sample struct{ n, successes int }
	groups := map[string]map[string]*sample{}
	for rows.Next() {
		var version, action, operation, revision string
		var success bool
		if rows.Scan(&version, &action, &operation, &revision, &success) != nil {
			return 0, 0, false
		}
		key := action + "\x00" + operation + "\x00" + revision
		if groups[key] == nil {
			groups[key] = map[string]*sample{}
		}
		if groups[key][version] == nil {
			groups[key][version] = &sample{}
		}
		s := groups[key][version]
		if s.n >= 50 {
			continue
		}
		s.n++
		if success {
			s.successes++
		}
	}
	if rows.Err() != nil {
		return 0, 0, false
	}
	var newRate, baseRate, weight float64
	for _, g := range groups {
		a, b := g[candidate], g[baseline]
		if a == nil {
			continue
		}
		if b == nil || a.n < w.evaluationLimit || b.n < w.evaluationLimit {
			return 0, 0, false
		}
		n := float64(min(a.n, b.n))
		weight += n
		newRate += n * float64(a.successes) / float64(a.n)
		baseRate += n * float64(b.successes) / float64(b.n)
	}
	if weight == 0 {
		return 0, 0, false
	}
	return newRate / weight, baseRate / weight, true
}
