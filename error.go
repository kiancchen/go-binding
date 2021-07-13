package binding

import (
	"fmt"
)

// Common Error
var (
	FieldNotFound = &Error{
		Format: "go-binding error: field=%s, cause=%s",
		Cause:  "field required but not found",
	}
	FieldConversionError = &Error{
		Format: "go-binding error: field=%s, cause=%s",
		Cause:  "field type can't be converted from string"}
)

type Error struct {
	field string

	Format string
	Cause  string
}

func (e *Error) setField(fieldName string) *Error {
	err := *e
	err.field = fieldName
	return &err
}

func (e *Error) Error() string {
	return fmt.Sprintf(e.Format, e.field, e.Cause)
}
