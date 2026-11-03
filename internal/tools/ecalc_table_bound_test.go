package tools

import (
	"slices"
	"testing"

	"aurago/internal/ecalc"
)

// The outer bound of ExecuteECalc (ecalc_budget.go) refuses a result that is over the budget with output_budget_exceeded.
// A page of table rows can always be shortened, so it must never get that answer: the table fit lists fewer rows, then
// leaves out the descriptive parts and, for a table that is too wide for the smallest budget, the listed row itself.
// The test asks for the largest page (50 rows) of every table of the catalog, with and without a lookup of the first
// key, at the default budget and at the smallest budgets the agent can hand over.
func TestECalcLargeTablePagesStayWithinTheBound(t *testing.T) {
	cat, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Tables) == 0 {
		t.Fatal("the catalog has no table")
	}
	for _, tbl := range cat.Tables {
		desc, err := cat.DescribeTable(tbl.ID)
		if err != nil {
			t.Fatal(err)
		}
		keyIdx := slices.IndexFunc(desc.Columns, func(c ecalc.TableColumnInfo) bool { return c.Key })
		// The first key as the page lists it (a number with its unit or a text), read with a budget nothing exceeds.
		first := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: tbl.ID, Limit: 1, OutputBudget: 1 << 20}))
		var lookup *ecalc.TableLookup
		if rows, _ := first["rows"].([]any); len(rows) > 0 {
			if key, ok := rows[0].([]any)[keyIdx].(string); ok {
				lookup = &ecalc.TableLookup{Value: key}
			}
		}
		for _, outputBudget := range []int{0, 6000, 2100, 1} {
			budget := ecalcBudget(outputBudget)
			requests := []ECalcRequest{{Operation: "table", ID: tbl.ID, Limit: 50, OutputBudget: outputBudget}}
			if lookup != nil {
				requests = append(requests, ECalcRequest{Operation: "table", ID: tbl.ID, Limit: 50, Lookup: lookup, OutputBudget: outputBudget})
			}
			for _, req := range requests {
				exec := ExecuteECalc(nil, req)
				out := ecalcDecode(t, exec)
				if out["status"] != "ok" || out["code"] != nil || len(exec.Output) > budget {
					t.Errorf("%s with budget %d (lookup %v): %d bytes, status %v, code %v: %.300s", tbl.ID, budget, req.Lookup != nil, len(exec.Output), out["status"], out["code"], exec.Output)
					continue
				}
				// A shortened page says where to continue (the page starts at row 0, so the next one starts after its rows).
				rows, _ := out["rows"].([]any)
				if out["returned"] != float64(len(rows)) || (out["truncated"] == true && len(rows) > 0 && out["next_offset"] != float64(len(rows))) {
					t.Errorf("%s with budget %d: returned %v of %d rows, truncated %v, next_offset %v", tbl.ID, budget, out["returned"], len(rows), out["truncated"], out["next_offset"])
				}
			}
		}
	}

	// The biggest generated table has 192 rows. At the smallest budget a page is a few rows, and the next page starts right
	// after them.
	small := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "eseries_e192", Limit: 50, OutputBudget: 2100}))
	rows, _ := small["rows"].([]any)
	if len(rows) < 5 || len(rows) >= 50 || small["total"] != float64(192) || small["truncated"] != true || small["next_offset"] != float64(len(rows)) {
		t.Fatalf("a page of E192 at the smallest budget: %d rows, %v", len(rows), small)
	}
	next := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "eseries_e192", Offset: len(rows), Limit: 3, OutputBudget: 2100}))
	if next["offset"] != float64(len(rows)) || next["returned"] != float64(3) {
		t.Errorf("the next page = %v", next)
	}
}
