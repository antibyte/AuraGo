package findit

import (
	"context"
	"database/sql"
)

// Catalog returns static lookup metadata with built-in templates read from the
// persisted database definitions. A fresh or compatible catalog refresh must
// not hide older definitions retained for items that already use them.
func (s *Store) Catalog(ctx context.Context) (CatalogData, error) {
	if s == nil || s.db == nil {
		return CatalogData{}, domainError("unavailable", "FindIt is unavailable")
	}
	return readSnapshot(ctx, s.db, func(tx *sql.Tx) (CatalogData, error) {
		data := Catalog()
		templates := make([]Template, 0, len(data.Templates))
		for _, definition := range data.Templates {
			template, err := getTemplateRecord(ctx, tx, definition.ID)
			if err != nil {
				return CatalogData{}, err
			}
			if template.BuiltIn {
				templates = append(templates, template)
			}
		}
		data.Templates = templates
		return data, nil
	})
}
