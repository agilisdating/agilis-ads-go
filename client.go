// Package agilisads is the official Go client for the Agilis Advertising REST
// API.
//
// Create a client with an API key (or leave it empty to read AGILIS_API_KEY
// from the environment) and call the operations grouped by resource:
//
//	client := agilisads.NewClient("pk_test_...")
//	categories, err := client.Categories.List(ctx)
//
// The key decides the mode: a pk_test_ key works in TEST mode and a pk_live_
// key in LIVE mode. Operations that take an environment fill it in from the
// key when it is left empty.
package agilisads

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// Version is the version of this module.
const Version = "0.1.0"

// DefaultBaseURL is the address of the Agilis Advertising API.
const DefaultBaseURL = "https://ads.agilis.dating"

// DefaultTimeout is how long a single attempt may take before it is abandoned.
const DefaultTimeout = 60 * time.Second

// DefaultMaxRetries is how many times a failed request is retried.
const DefaultMaxRetries = 2

// ErrNoAPIKey is returned by every operation when the client has no API key:
// none was passed to NewClient and AGILIS_API_KEY is not set.
var ErrNoAPIKey = errors.New("agilisads: no API key: pass one to NewClient or set AGILIS_API_KEY")

// Mode is the mode an API key works in.
type Mode string

// The modes an API key can have.
const (
	ModeTest    Mode = "test"
	ModeLive    Mode = "live"
	ModeUnknown Mode = ""
)

// Environment returns the environment value the API uses for this mode, or ""
// when the mode is unknown.
func (m Mode) Environment() Environment {
	switch m {
	case ModeTest:
		return EnvironmentTest
	case ModeLive:
		return EnvironmentLive
	}
	return ""
}

// Environment is the environment parameter of the API: TEST or LIVE.
type Environment string

// The environments of the API.
const (
	EnvironmentTest Environment = "TEST"
	EnvironmentLive Environment = "LIVE"
)

// ModeFromKey returns the mode of an API key from its prefix.
func ModeFromKey(apiKey string) Mode {
	switch {
	case strings.HasPrefix(apiKey, "pk_test_"):
		return ModeTest
	case strings.HasPrefix(apiKey, "pk_live_"):
		return ModeLive
	}
	return ModeUnknown
}

// HTTPDoer is the part of *http.Client the SDK uses. Any *http.Client works.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL sets the API address. The default is DefaultBaseURL.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

// WithTimeout sets how long a single attempt may take. The default is
// DefaultTimeout. Zero or less means no timeout beyond the context's own.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithMaxRetries sets how many times a failed request is retried. The default
// is DefaultMaxRetries. Zero turns retries off.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		if n < 0 {
			n = 0
		}
		c.maxRetries = n
	}
}

// WithHTTPClient sets the HTTP client used to send requests. The default is a
// new *http.Client.
func WithHTTPClient(hc HTTPDoer) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// Client is a client for the Agilis Advertising API. It is safe for
// concurrent use.
type Client struct {
	apiKey     string
	mode       Mode
	baseURL    string
	timeout    time.Duration
	maxRetries int
	httpClient HTTPDoer
	userAgent  string

	// Replaced in tests.
	sleep  func(ctx context.Context, d time.Duration) error
	jitter func() float64

	Categories       *CategoriesService
	Interests        *InterestsService
	Offers           *OffersService
	EventCategories  *EventCategoriesService
	EventCurrencies  *EventCurrenciesService
	EventConfig      *EventConfigService
	Events           *EventsService
	WebhookEndpoints *WebhookEndpointsService
	WebhookEvents    *WebhookEventsService
}

// NewClient returns a client that uses apiKey. When apiKey is empty, the key
// is read from the AGILIS_API_KEY environment variable.
func NewClient(apiKey string, opts ...Option) *Client {
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("AGILIS_API_KEY"))
	}
	c := &Client{
		apiKey:     apiKey,
		mode:       ModeFromKey(apiKey),
		baseURL:    DefaultBaseURL,
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
		httpClient: &http.Client{},
		userAgent:  fmt.Sprintf("agilis-ads-go/%s (%s; %s/%s)", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH),
		sleep:      sleepContext,
		jitter:     rand.Float64,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	c.Categories = &CategoriesService{client: c}
	c.Interests = &InterestsService{client: c}
	c.Offers = &OffersService{client: c}
	c.EventCategories = &EventCategoriesService{client: c}
	c.EventCurrencies = &EventCurrenciesService{client: c}
	c.EventConfig = &EventConfigService{client: c}
	c.Events = &EventsService{client: c}
	c.WebhookEndpoints = &WebhookEndpointsService{client: c}
	c.WebhookEvents = &WebhookEventsService{client: c}
	return c
}

// Mode returns the mode of the client's API key: ModeTest, ModeLive or
// ModeUnknown. Use it to refuse to run against LIVE.
func (c *Client) Mode() Mode { return c.mode }

// BaseURL returns the API address the client sends requests to.
func (c *Client) BaseURL() string { return c.baseURL }

// String describes the client without its API key.
func (c *Client) String() string {
	return fmt.Sprintf("agilisads.Client{mode: %q, baseURL: %q}", string(c.mode), c.baseURL)
}

// environment returns e, or the environment of the key's mode when e is empty.
func (c *Client) environment(e Environment) Environment {
	if e != "" {
		return e
	}
	return c.mode.Environment()
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// String returns a pointer to v, for optional parameters.
func String(v string) *string { return &v }

// Int returns a pointer to v, for optional parameters.
func Int(v int) *int { return &v }

// Int64 returns a pointer to v, for optional parameters.
func Int64(v int64) *int64 { return &v }

// Float64 returns a pointer to v, for optional parameters.
func Float64(v float64) *float64 { return &v }

// Bool returns a pointer to v, for optional parameters.
func Bool(v bool) *bool { return &v }
