package ld

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError is a non-2xx response from the LaunchDarkly API, decoded from its
// standard {"code": "...", "message": "..."} error body.
type APIError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("launchdarkly: %s (HTTP %d, code %q)", e.Message, e.StatusCode, e.Code)
	}
	return fmt.Sprintf("launchdarkly: HTTP %d", e.StatusCode)
}

// IsNotFound reports whether err is an APIError with HTTP 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// IsUnauthorized reports whether err is an APIError with HTTP 401 or 403.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) &&
		(apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden)
}
