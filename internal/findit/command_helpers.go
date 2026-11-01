package findit

import (
	"database/sql"
	"errors"
	"strings"

	"aurago/internal/uid"
)

func mapWriteError(err error, operation string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "unique constraint") || strings.Contains(text, "foreign key constraint") || strings.Contains(text, "check constraint") {
		return domainError("conflict", "FindIt data conflicts with an existing record")
	}
	var domain *DomainError
	if errors.As(err, &domain) {
		return err
	}
	return wrapError("unavailable", "FindIt could not "+operation, err)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func oneOf(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func validCategory(category string) bool {
	if fieldKeyPattern.MatchString(category) {
		return true
	}
	for _, item := range Catalog().Categories {
		if item.ID == category {
			return true
		}
	}
	return false
}

func newID() string { return uid.New() }
