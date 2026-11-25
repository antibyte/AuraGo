package server

import "aurago/internal/homedash"

// homeDashStarterSystem builds the "system" starter dashboard (decision D3): hero variants only,
// family glass, theme palette, square 4x4 gauges over a 2-row band. Look.RadiusScale and
// Grid.RowUnit are set explicitly (Go-built documents, design section 12.3). The caller runs the
// save pipeline.
func homeDashStarterSystem(name string) homedash.Dashboard {
	if name == "" {
		name = "System"
	}
	gauge := func(id, variant, entity string, warn float64) homedash.Widget {
		return homedash.Widget{ID: id, Type: "gauge", Variant: variant, Bindings: map[string]string{"primary": entity},
			Options: map[string]any{"min": 0.0, "max": 100.0, "warn": warn, "alert": 95.0, "decimals": 0.0}}
	}
	spark := func(id, entity string) homedash.Widget {
		return homedash.Widget{ID: id, Type: "sensor", Variant: "sparkline", Bindings: map[string]string{"primary": entity},
			Options: map[string]any{"decimals": 1.0, "hours": 1.0}}
	}
	compact := true
	return homedash.Dashboard{
		Name: name,
		Look: homedash.Look{Family: "glass", Palette: homedash.Palette{Mode: "theme"}, Backdrop: homedash.Backdrop{Kind: "theme"},
			Density: "comfortable", RadiusScale: 1},
		Grid: homedash.Grid{RowUnit: homedash.RowUnit{Auto: true}, Fit: "scroll"},
		Pages: []homedash.Page{{
			ID: "main", Name: "Main", Compact: &compact,
			Widgets: []homedash.Widget{
				gauge("cpu", "arc", "system:cpu", 80),
				gauge("memory", "needle", "system:memory", 85),
				gauge("disk", "arc", "system:disk", 85),
				spark("net-rx", "system:net_rx"),
				spark("net-tx", "system:net_tx"),
				{ID: "uptime", Type: "entity", Variant: "tile", Bindings: map[string]string{"primary": "system:uptime"}, Options: map[string]any{}},
			},
			Layouts: homedash.Layouts{LG: []homedash.LayoutItem{
				{ID: "cpu", X: 0, Y: 0, W: 4, H: 4}, {ID: "memory", X: 4, Y: 0, W: 4, H: 4}, {ID: "disk", X: 8, Y: 0, W: 4, H: 4},
				{ID: "net-rx", X: 0, Y: 4, W: 5, H: 2}, {ID: "net-tx", X: 5, Y: 4, W: 5, H: 2}, {ID: "uptime", X: 10, Y: 4, W: 2, H: 2},
			}},
		}},
	}
}
