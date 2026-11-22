package agilisads

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Response holds the HTTP details of a response. Every response type embeds
// it, so the status, the headers (for example X-RateLimit-Remaining) and the
// raw JSON, including any fields this version of the SDK does not know, stay
// reachable.
type Response struct {
	// StatusCode is the HTTP status.
	StatusCode int `json:"-"`

	// Header holds the response headers.
	Header http.Header `json:"-"`

	// RawJSON is the response body as received.
	RawJSON json.RawMessage `json:"-"`
}

func (r *Response) setResponse(status int, header http.Header, body []byte) {
	r.StatusCode = status
	r.Header = header
	r.RawJSON = append(json.RawMessage(nil), body...)
}

// Field returns the raw JSON of a top-level field of the response, or nil
// when it is absent. Use it for fields this version of the SDK does not type.
func (r *Response) Field(name string) json.RawMessage {
	var m map[string]json.RawMessage
	if json.Unmarshal(r.RawJSON, &m) != nil {
		return nil
	}
	return m[name]
}

// RateLimitRemaining returns the X-RateLimit-Remaining header, or -1 when it
// is absent.
func (r *Response) RateLimitRemaining() int {
	if r.Header == nil {
		return -1
	}
	n, err := strconv.Atoi(r.Header.Get("X-RateLimit-Remaining"))
	if err != nil {
		return -1
	}
	return n
}

type responseSetter interface {
	setResponse(status int, header http.Header, body []byte)
}

type request struct {
	method         string
	path           string
	query          url.Values
	body           []byte
	contentType    string
	idempotencyKey string
	methodOverride string
}

func (r *request) setJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("agilisads: encoding the request: %w", err)
	}
	r.body = b
	r.contentType = "application/json"
	return nil
}

func (r *request) setForm(f *form) error {
	body, contentType, err := f.finish()
	if err != nil {
		return err
	}
	r.body = body
	r.contentType = contentType
	return nil
}

// do sends the request, retrying as the SDK specification says, and decodes
// a successful response into out.
func (c *Client) do(ctx context.Context, r *request, out responseSetter) error {
	if c.apiKey == "" {
		return ErrNoAPIKey
	}
	if ctx == nil {
		ctx = context.Background()
	}
	u := c.baseURL + r.path
	if len(r.query) > 0 {
		u += "?" + r.query.Encode()
	}
	for attempt := 0; ; attempt++ {
		status, header, body, err := c.send(ctx, r, u)
		if err != nil {
			if ctx.Err() != nil {
				return newConnectionError(ctx.Err(), c.apiKey)
			}
			if attempt < c.maxRetries {
				if serr := c.sleep(ctx, c.backoff(attempt)); serr != nil {
					return newConnectionError(serr, c.apiKey)
				}
				continue
			}
			return newConnectionError(err, c.apiKey)
		}
		if apiErr := errorFromResponse(status, header, body, c.apiKey); apiErr != nil {
			if attempt < c.maxRetries && r.retryable(status) {
				wait, ok := retryAfter(header)
				if !ok {
					wait = c.backoff(attempt)
				}
				if serr := c.sleep(ctx, wait); serr != nil {
					return newConnectionError(serr, c.apiKey)
				}
				continue
			}
			return apiErr
		}
		if out == nil {
			return nil
		}
		out.setResponse(status, header, body)
		if len(bytes.TrimSpace(body)) == 0 {
			return nil
		}
		if err := json.Unmarshal(body, out); err != nil {
			return &APIError{AgilisError{
				StatusCode: status,
				Message:    "could not decode the response: " + err.Error(),
				Body:       []byte(redact(string(body), c.apiKey)),
				Data:       parseObject(body),
				Header:     header,
			}}
		}
		return nil
	}
}

// send makes one attempt. An error means no response was received.
func (c *Client) send(ctx context.Context, r *request, u string) (int, http.Header, []byte, error) {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	var body io.Reader
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}
	req, err := http.NewRequestWithContext(ctx, r.method, u, body)
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if r.contentType != "" {
		req.Header.Set("Content-Type", r.contentType)
	}
	if r.idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", r.idempotencyKey)
	}
	if r.methodOverride != "" {
		req.Header.Set("X-HTTP-Method-Override", r.methodOverride)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, err
	}
	return resp.StatusCode, resp.Header, b, nil
}

// retryable says whether a response with this status may be retried. 429 is
// always retried. 500, 502, 503 and 504 are retried for GET and DELETE, and
// for a POST that carries an Idempotency-Key; never for another POST or a
// PATCH, because the change may have happened.
func (r *request) retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests:
		return true
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		switch r.method {
		case http.MethodGet, http.MethodDelete:
			return true
		case http.MethodPost:
			return r.idempotencyKey != "" && r.methodOverride == ""
		}
	}
	return false
}

// backoff is 0.5 seconds × 2^attempt with ±25% jitter, at most 8 seconds.
func (c *Client) backoff(attempt int) time.Duration {
	if attempt > 10 {
		attempt = 10
	}
	base := float64(500*time.Millisecond) * float64(int64(1)<<uint(attempt))
	d := time.Duration(base * (0.75 + 0.5*c.jitter()))
	if d > 8*time.Second {
		d = 8 * time.Second
	}
	return d
}

// retryAfter reads the Retry-After header: a number of seconds or an HTTP date.
func retryAfter(h http.Header) (time.Duration, bool) {
	if h == nil {
		return 0, false
	}
	v := h.Get("Retry-After")
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.ParseFloat(v, 64); err == nil {
		if secs < 0 {
			secs = 0
		}
		return time.Duration(secs * float64(time.Second)), true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}

// newIdempotencyKey returns a random UUID version 4.
func newIdempotencyKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("agilisads: generating an Idempotency-Key: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

var errNilParams = errors.New("agilisads: params are required")

func pathInt(id int64) string { return strconv.FormatInt(id, 10) }

func pathString(id string) string { return url.PathEscape(id) }

func dryRunQuery(dryRun bool) url.Values {
	if !dryRun {
		return nil
	}
	return url.Values{"dry_run": {"true"}}
}
