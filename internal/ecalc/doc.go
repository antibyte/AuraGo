// Package ecalc is the engine behind the ECalc desktop app: a declarative
// electronics catalog (embedded YAML), a small safe expression language, units
// and number parsing/formatting, E-series lookup and a rule propagation solver.
//
// The package has no server dependency. The HTTP route, the agent tool and the
// tests all use it directly. The expression language, the unit parser and
// formatter, the E-series lookup and the solver are mirrored in JavaScript under
// ui/js/desktop/apps/ecalc; both implementations run the shared vectors in
// testdata/vectors.
package ecalc
