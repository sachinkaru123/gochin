package orm

import "errors"

var (
	// ErrRecordNotFound is returned when a query expected to match a row finds none.
	ErrRecordNotFound = errors.New("orm: record not found")

	// ErrMissingPrimaryKey is returned when an operation requires a primary key
	// value that is still at its zero value.
	ErrMissingPrimaryKey = errors.New("orm: missing primary key")
)
