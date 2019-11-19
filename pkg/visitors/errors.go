package visitors

import (
	"fmt"
)

// ErrorType -
type ErrorType int

const (
	// ErrNotSupported not supported sql syntax
	ErrNotSupported ErrorType = iota
	// ErrInvalidExpr invalid
	ErrInvalidExpr
	// ErrTypeCheck type check failed
	ErrTypeCheck
	// ErrCompilerError unexpected behaviour in code.
	ErrCompilerError
)

// Error -
type Error struct {
	Type   ErrorType
	Detail string
}

// NewError -
func NewError(t ErrorType, detail string) Error {
	return Error{
		Type:   t,
		Detail: detail,
	}
}

// NewErrorf -
func NewErrorf(t ErrorType, format string, arg ...interface{}) Error {
	return Error{
		Type:   t,
		Detail: fmt.Sprintf(format, arg...),
	}
}

func (e Error) Error() string {
	prefix := ""
	if e.Type == ErrNotSupported {
		prefix = "[NotSupported]"
	} else if e.Type == ErrInvalidExpr {
		prefix = "[InvalidExpr]"
	} else if e.Type == ErrCompilerError {
		prefix = "[CompilerError]"
	} else if e.Type == ErrTypeCheck {
		prefix = "[TypeCheck]"
	}
	return prefix + " " + e.Detail
}
