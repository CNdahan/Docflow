package errs

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError 是服务层向 API 层抛的业务错误。handler 把它翻成 HTTP 响应。
type AppError struct {
	HTTPStatus int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Cause      error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Cause }

func New(status int, code, msg string) *AppError {
	return &AppError{HTTPStatus: status, Code: code, Message: msg}
}

func Wrap(status int, code, msg string, cause error) *AppError {
	return &AppError{HTTPStatus: status, Code: code, Message: msg, Cause: cause}
}

// 预定义错误,便于全局引用
var (
	ErrUnauthorized = New(http.StatusUnauthorized, "UNAUTHORIZED", "未登录或登录已过期")
	ErrForbidden    = New(http.StatusForbidden, "FORBIDDEN", "无权限执行此操作")
	ErrNotFound     = New(http.StatusNotFound, "NOT_FOUND", "资源不存在")
	ErrBadRequest   = New(http.StatusBadRequest, "BAD_REQUEST", "请求参数错误")
	ErrConflict     = New(http.StatusConflict, "CONFLICT", "资源冲突")
	ErrInternal     = New(http.StatusInternalServerError, "INTERNAL", "服务内部错误")
)

func AsAppError(err error) (*AppError, bool) {
	var e *AppError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
