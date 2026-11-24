package homedash

// PrincipalKind says who sends a request.
type PrincipalKind string

const (
	PrincipalSession PrincipalKind = "session"
	PrincipalKiosk   PrincipalKind = "kiosk"
	PrincipalAgent   PrincipalKind = "agent"
)

// Principal is who sends a request. Key is the rate-limit key (session hash, kiosk device id).
// A kiosk principal with a nil Allowed may read and command nothing.
type Principal struct {
	Kind        PrincipalKind
	Key         string
	DeviceID    string
	DashboardID string
	ViewOnly    bool
	Allowed     *AllowedSet
}

// AllowedSet maps an entity to the commands a kiosk may send; an empty map means read only.
type AllowedSet struct {
	Commands map[EntityID]map[Capability]bool
}

// Has reports whether the entity is in the set at all (read access).
func (a *AllowedSet) Has(id EntityID) bool {
	if a == nil {
		return false
	}
	_, ok := a.Commands[id]
	return ok
}

// Allows reports whether the entity may receive the command.
func (a *AllowedSet) Allows(id EntityID, c Capability) bool { return a != nil && a.Commands[id][c] }

// BuildAllowedSet gives a kiosk display the entities bound on doc (the approved revision) and the
// commands its widgets can issue (catalog Commands); bindings without commands are read only. The
// set derives only from the catalog: a widget of an unknown type or variant, and a binding the type
// does not declare, contribute nothing, not even read access. Call it with the approved document only.
func BuildAllowedSet(doc Dashboard, cat WidgetCatalog) *AllowedSet {
	set := &AllowedSet{Commands: map[EntityID]map[Capability]bool{}}
	for _, p := range doc.Pages {
		for _, w := range p.Widgets {
			wt, typed := cat.Type(w.Type)
			if !typed {
				continue
			}
			if _, known := wt.Variant(w.Variant); !known {
				continue
			}
			for name, raw := range w.Bindings {
				if _, declared := wt.Binding(name); !declared {
					continue
				}
				if _, _, err := ParseEntityID(raw); err != nil {
					continue
				}
				id := EntityID(raw)
				m := set.Commands[id]
				if m == nil {
					m = map[Capability]bool{}
					set.Commands[id] = m
				}
				for _, c := range wt.Commands[name] {
					m[c] = true
				}
			}
		}
	}
	return set
}
