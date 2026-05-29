package service

import "errors"

// ErrAccessDenied is a sentinel error for permission denials.
var ErrAccessDenied = errors.New("access denied")

// AccessDeniedError carries a detailed message about a permission denial.
type AccessDeniedError struct {
	Msg string
}

func (e *AccessDeniedError) Error() string { return e.Msg }

// Is reports whether this error matches the ErrAccessDenied sentinel.
func (e *AccessDeniedError) Is(target error) bool {
	return target == ErrAccessDenied
}

// NewAccessDeniedError returns a new AccessDeniedError wrapping the given message.
func NewAccessDeniedError(msg string) error {
	return &AccessDeniedError{Msg: msg}
}
