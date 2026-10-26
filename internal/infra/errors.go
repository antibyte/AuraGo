package infra

import "errors"

var (
	ErrNotFound       = errors.New("infra item not found")
	ErrConflict       = errors.New("infra version conflict")
	ErrValidation     = errors.New("invalid infra data")
	ErrReadOnly       = errors.New("provider edge is read-only")
	ErrSourceNotFound = errors.New("infra source not found")
	ErrSourceLinked   = errors.New("infra source is already linked")
	ErrSourceRemoved  = errors.New("infra source was removed")
	ErrHasChildren    = errors.New("infra node has children")
	ErrEdgeExists     = errors.New("infra edge already exists")
)
