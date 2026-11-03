package ecalc

import "sort"

// Generated tables (generate: in tables/*.yaml) take their columns and rows from the engine instead of the YAML. The
// generators themselves follow with the generated tables; until then no generator is known.

// genResult is the output of a generator: the columns as the YAML would list them, the default key column and the rows
// in SI units.
type genResult struct {
	cols []rawColumn
	key  string
	rows [][]Cell
}

var tableGenerators = map[string]func() genResult{}

func generatorIDs() []string {
	ids := make([]string, 0, len(tableGenerators))
	for id := range tableGenerators {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
