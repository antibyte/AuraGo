package main

import "fmt"

// Preserve bilingual operation coverage as schemas grow, taking only the
// additional required rows from multi-call examples. The pack stays at 5000
// rows and retains at least 500 multi-call scenarios and all negative cases.
func scenarioOperationBudget(operations int) (direct, multi int, err error) {
	direct = max(2250, operations*2)
	multi = 3250 - direct
	if multi < 500 {
		return 0, 0, fmt.Errorf("%d operations exceed the bilingual scenario budget; review the training mix", operations)
	}
	return direct, multi, nil
}
