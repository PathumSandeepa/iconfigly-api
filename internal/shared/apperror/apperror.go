package apperror

import (
	"errors"
	"fmt"
)

type Code string

const (
	CodeNotFound           Code = "NOT_FOUND"
	CodeConflict           Code = "CONFLICT"
	CodeValidationFailed   Code = "VALIDATION_FAILED"
	CodeUnauthorized       Code = "UNAUTHORIZED"
	CodeForbidden          Code = "FORBIDDEN"
	CodeSeparationOfDuties Code = "SEPARATION_OF_DUTIES"
	CodeRateLimited        Code = "RATE_LIMITED"
	CodeInternal           Code = "INTERNAL"
)

type Error struct {
	Code   Code
	Title  string
	Detail string
	Fields map[string]string
	Cause  error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Detail, e.Cause)
	}

	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func NotFound(resource string) *Error {
	return &Error{
		Code:   CodeNotFound,
		Title:  "Resource not found",
		Detail: fmt.Sprintf("%s was not found", resource),
	}
}

func Conflict(detail string) *Error {
	return &Error{
		Code:   CodeConflict,
		Title:  "Conflict",
		Detail: detail,
	}
}

func Validation(detail string, fields map[string]string) *Error {
	return &Error{
		Code:   CodeValidationFailed,
		Title:  "Validation failed",
		Detail: detail,
		Fields: fields,
	}
}

func Unauthorized(detail string) *Error {
	return &Error{
		Code:   CodeUnauthorized,
		Title:  "Authentication required",
		Detail: detail,
	}
}

func Forbidden(detail string) *Error {
	return &Error{
		Code:   CodeForbidden,
		Title:  "Access denied",
		Detail: detail,
	}
}

func Internal(cause error) *Error {
	return &Error{
		Code:   CodeInternal,
		Title:  "Internal server error",
		Detail: "an unexpected error occurred",
		Cause:  cause,
	}
}

func From(err error) (*Error, bool) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr, true
	}

	return nil, false
}

func HasCode(err error, code Code) bool {
	appErr, ok := From(err)

	return ok && appErr.Code == code
}
