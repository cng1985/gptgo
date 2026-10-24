package errors

import (
	stderrors "errors"
	"fmt"
)

type Category string

const (
	CategoryInvalid      Category = "invalid"
	CategoryUnauthorized Category = "unauthorized"
	CategoryForbidden    Category = "forbidden"
	CategoryNotFound     Category = "not_found"
	CategoryConflict     Category = "conflict"
	CategoryTooLarge     Category = "too_large"
	CategoryRateLimit    Category = "rate_limit"
	CategoryInternal     Category = "internal"
	CategoryUnavailable  Category = "unavailable"
)

type Error struct {
	Cat     Category
	AppCode int
	Msg     string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Cause)
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Cause }

func New(cat Category, code int, msg string, cause error) *Error {
	return &Error{Cat: cat, AppCode: code, Msg: msg, Cause: cause}
}

func Invalid(msg string) *Error      { return New(CategoryInvalid, 400, msg, nil) }
func Unauthorized(msg string) *Error { return New(CategoryUnauthorized, 401, msg, nil) }
func Forbidden(msg string) *Error    { return New(CategoryForbidden, 403, msg, nil) }
func NotFound(msg string) *Error     { return New(CategoryNotFound, 404, msg, nil) }
func Conflict(msg string) *Error     { return New(CategoryConflict, 409, msg, nil) }
func TooLarge(msg string) *Error     { return New(CategoryTooLarge, 413, msg, nil) }
func Unavailable(msg string) *Error  { return New(CategoryUnavailable, 503, msg, nil) }
func Internal(cause error) *Error    { return New(CategoryInternal, 500, "服务内部错误", cause) }

func As(err error) (*Error, bool) {
	var target *Error
	if stderrors.As(err, &target) {
		return target, true
	}
	return nil, false
}
