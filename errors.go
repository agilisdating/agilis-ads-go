package agilisads

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AgilisError is the base of every error the API returns. Each error the SDK
// returns for a failed request is one of the types below, and each of them
// unwraps to *AgilisError, so either can be used with errors.As:
//
//	var nf *agilisads.NotFoundError
//	if errors.As(err, &nf) { ... }
//
//	var ae *agilisads.AgilisError
//	if errors.As(err, &ae) { fmt.Println(ae.StatusCode, ae.ReturnCode) }
//
// The API key never appears in an error.
type AgilisError struct {
	// StatusCode is the HTTP status, or 0 when there was no response.
	StatusCode int

	// ReturnCode is the API's return_code, such as "E1", when there is one.
	ReturnCode string

	// Message is the API's message, or a description of the failure.
	Message string

	// Body is the raw response body.
	Body []byte

	// Data is the parsed response body when it is a JSON object.
	Data map[string]any

	// Header holds the response headers.
	Header http.Header
}

func (e *AgilisError) Error() string {
	var b strings.Builder
	b.WriteString("agilisads: ")
	b.WriteString(e.Message)
	switch {
	case e.StatusCode != 0 && e.ReturnCode != "":
		fmt.Fprintf(&b, " (HTTP %d, return_code %s)", e.StatusCode, e.ReturnCode)
	case e.StatusCode != 0:
		fmt.Fprintf(&b, " (HTTP %d)", e.StatusCode)
	case e.ReturnCode != "":
		fmt.Fprintf(&b, " (return_code %s)", e.ReturnCode)
	}
	return b.String()
}

// AuthenticationError is returned for HTTP 401: the key is missing, unknown,
// disabled or expired.
type AuthenticationError struct{ AgilisError }

// Unwrap returns the base error.
func (e *AuthenticationError) Unwrap() error { return &e.AgilisError }

// PermissionError is returned for HTTP 403, for example when the environment
// is not the key's own mode.
type PermissionError struct{ AgilisError }

// Unwrap returns the base error.
func (e *PermissionError) Unwrap() error { return &e.AgilisError }

// NotFoundError is returned for HTTP 404.
type NotFoundError struct{ AgilisError }

// Unwrap returns the base error.
func (e *NotFoundError) Unwrap() error { return &e.AgilisError }

// ConflictError is returned for HTTP 409, for example when an Idempotency-Key
// is reused while the first request is still being processed.
type ConflictError struct{ AgilisError }

// Unwrap returns the base error.
func (e *ConflictError) Unwrap() error { return &e.AgilisError }

// InvalidRequestError is returned for HTTP 400 and 422, and for a refusal that
// arrives as HTTP 200 with a return_code starting with "E" (for example E3,
// when the balance is too low). StatusCode is 200 in that case.
type InvalidRequestError struct{ AgilisError }

// Unwrap returns the base error.
func (e *InvalidRequestError) Unwrap() error { return &e.AgilisError }

// RateLimitError is returned for HTTP 429 once the retries are used up.
type RateLimitError struct {
	AgilisError

	// RetryAfter is the wait the API asked for, or 0 when it gave none.
	RetryAfter time.Duration
}

// Unwrap returns the base error.
func (e *RateLimitError) Unwrap() error { return &e.AgilisError }

// APIError is returned for HTTP 5xx and any other unexpected status, and when
// a successful response cannot be decoded.
type APIError struct{ AgilisError }

// Unwrap returns the base error.
func (e *APIError) Unwrap() error { return &e.AgilisError }

// ConnectionError is returned when no response was received: the connection
// failed, the attempt timed out or the context ended.
type ConnectionError struct {
	AgilisError

	// Err is the underlying error.
	Err error
}

// Unwrap returns the base error and the underlying error.
func (e *ConnectionError) Unwrap() []error { return []error{&e.AgilisError, e.Err} }

// SignatureVerificationError is returned by ConstructEvent when a webhook
// message cannot be verified.
type SignatureVerificationError struct {
	AgilisError

	// Reason is "header" (the signature header is missing or malformed),
	// "timestamp" (outside the tolerance) or "signature" (no v1 matches).
	Reason string
}

// Unwrap returns the base error.
func (e *SignatureVerificationError) Unwrap() error { return &e.AgilisError }

// errorFromResponse returns the error for a response, or nil when it is a
// success. key is redacted from everything the error holds.
func errorFromResponse(status int, header http.Header, body []byte, key string) error {
	data := parseObject(body)
	returnCode, _ := data["return_code"].(string)
	success := status >= 200 && status < 300
	if success && !strings.HasPrefix(returnCode, "E") {
		return nil
	}
	message, _ := data["message"].(string)
	if message == "" {
		if s, ok := data["error"].(string); ok {
			message = s
		}
	}
	if message == "" {
		message = http.StatusText(status)
	}
	if message == "" {
		message = "request failed"
	}
	base := AgilisError{
		StatusCode: status,
		ReturnCode: returnCode,
		Message:    redact(message, key),
		Body:       []byte(redact(string(body), key)),
		Header:     header,
	}
	base.Data = parseObject(base.Body)
	if success {
		return &InvalidRequestError{base}
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return &InvalidRequestError{base}
	case http.StatusUnauthorized:
		return &AuthenticationError{base}
	case http.StatusForbidden:
		return &PermissionError{base}
	case http.StatusNotFound:
		return &NotFoundError{base}
	case http.StatusConflict:
		return &ConflictError{base}
	case http.StatusTooManyRequests:
		ra, _ := retryAfter(header)
		return &RateLimitError{AgilisError: base, RetryAfter: ra}
	}
	return &APIError{base}
}

func newConnectionError(err error, key string) *ConnectionError {
	return &ConnectionError{
		AgilisError: AgilisError{Message: redact("connection error: "+err.Error(), key)},
		Err:         err,
	}
}

func parseObject(body []byte) map[string]any {
	var m map[string]any
	if len(body) == 0 || json.Unmarshal(body, &m) != nil {
		return nil
	}
	return m
}

func redact(s, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "[redacted]")
}
