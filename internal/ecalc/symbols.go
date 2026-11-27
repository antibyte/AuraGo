package ecalc

// SymbolIDs are the schematic symbols of ECalc's own library, in the order of the SYMBOLS object of
// ui/js/desktop/apps/ecalc/ecalc-symbols.js. A cell of a table column with dim "symbol" names one of them; the page draws
// it in the IEC and in the ANSI style. A ui test keeps this list equal to the keys of the SYMBOLS object.
var SymbolIDs = []string{
	"resistor", "capacitor", "polarized", "inductor", "diode", "led", "zener", "crystal", "fuse", "switch", "pot", "source",
	"acsource", "isource", "battery", "meter", "ground", "antenna", "npn", "pnp", "nmos", "pmos", "opamp", "transformer", "ic",
}

// IsSymbolID reports whether id is one of SymbolIDs.
func IsSymbolID(id string) bool {
	for _, s := range SymbolIDs {
		if s == id {
			return true
		}
	}
	return false
}
