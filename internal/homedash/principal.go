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
