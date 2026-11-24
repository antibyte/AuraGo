package homedash

import "fmt"

// Migrate brings doc to SchemaVersion in memory. A newer document stays unchanged and returns
// schema_too_new (the caller shows it read-only).
func Migrate(doc *Dashboard) error {
	switch {
	case doc.SchemaVersion > SchemaVersion:
		return &Error{Code: CodeSchemaTooNew, Message: fmt.Sprintf("dashboard schema %d is newer than supported %d", doc.SchemaVersion, SchemaVersion)}
	case doc.SchemaVersion <= 0:
		doc.SchemaVersion = SchemaVersion
	}
	return nil
}
