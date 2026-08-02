// Package errors 为 API 提供统一的错误类型。
package errors

import (
	"fmt"
	"net/http"
)

// Code 是 API 错误码。
type Code string

const (
	CodeNotFound       Code = "NOT_FOUND"
	CodeInvalidID      Code = "INVALID_ID"
	CodeInvalidRequest Code = "INVALID_REQUEST"
	CodeInternal       Code = "INTERNAL_ERROR"
	CodeRateLimited    Code = "RATE_LIMITED"
)

// APIError 是结构化的 API 错误。
type APIError struct {
	Code       Code   `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *APIError) Error() string {
	return e.Message
}

// 预定义的常见错误
var (
	ErrNotFound       = &APIError{Code: CodeNotFound, Message: "Resource not found", HTTPStatus: http.StatusNotFound}
	ErrInvalidID      = &APIError{Code: CodeInvalidID, Message: "Invalid ID format", HTTPStatus: http.StatusBadRequest}
	ErrInternal       = &APIError{Code: CodeInternal, Message: "Internal server error", HTTPStatus: http.StatusInternalServerError}
	ErrInvalidRequest = &APIError{Code: CodeInvalidRequest, Message: "Invalid request", HTTPStatus: http.StatusBadRequest}
	ErrRateLimited    = &APIError{Code: CodeRateLimited, Message: "Rate limit exceeded", HTTPStatus: http.StatusTooManyRequests}
)

// NotFound 构造指定资源的「未找到」错误。
func NotFound(resource string) *APIError {
	return &APIError{
		Code:       CodeNotFound,
		Message:    fmt.Sprintf("%s not found", resource),
		HTTPStatus: http.StatusNotFound,
	}
}

// InvalidID 构造指定参数的「ID 非法」错误。
func InvalidID(paramName string) *APIError {
	return &APIError{
		Code:       CodeInvalidID,
		Message:    fmt.Sprintf("Invalid %s: must be a positive integer", paramName),
		HTTPStatus: http.StatusBadRequest,
	}
}

// InvalidRequest 构造带自定义提示的「请求非法」错误。
func InvalidRequest(message string) *APIError {
	return &APIError{
		Code:       CodeInvalidRequest,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}

// Internal 构造服务端内部错误，message 为空时使用默认提示。
func Internal(message string) *APIError {
	if message == "" {
		message = "Internal server error"
	}
	return &APIError{
		Code:       CodeInternal,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
	}
}
