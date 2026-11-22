package agilisads

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestModeFromKey(t *testing.T) {
	cases := map[string]Mode{
		"pk_test_abc": ModeTest,
		"pk_live_abc": ModeLive,
		"sk_other":    ModeUnknown,
		"":            ModeUnknown,
	}
	for key, want := range cases {
		wantEqual(t, "ModeFromKey("+key+")", ModeFromKey(key), want)
	}
	wantEqual(t, "live client", NewClient("pk_live_x").Mode(), ModeLive)
	wantEqual(t, "test environment", ModeTest.Environment(), EnvironmentTest)
	wantEqual(t, "live environment", ModeLive.Environment(), EnvironmentLive)
	wantEqual(t, "unknown environment", ModeUnknown.Environment(), Environment(""))
}

func TestLiveKeyFillsLiveEnvironment(t *testing.T) {
	f := newFakeAPI(t, reply{Body: `{"data":[],"pagination":{"has_more":false}}`})
	c := NewClient("pk_live_abc", WithBaseURL(f.server.URL))
	if _, err := c.Offers.List(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "query", f.last().Query, "environment=LIVE")
}

func TestKeyFromEnvironment(t *testing.T) {
	t.Setenv("AGILIS_API_KEY", "pk_test_fromenv")
	c := NewClient("")
	wantEqual(t, "mode", c.Mode(), ModeTest)
	wantEqual(t, "key", c.apiKey, "pk_test_fromenv")
}

func TestNoAPIKey(t *testing.T) {
	t.Setenv("AGILIS_API_KEY", "")
	c := NewClient("")
	if _, err := c.Categories.List(context.Background()); !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("err = %v, want ErrNoAPIKey", err)
	}
}

func TestOptions(t *testing.T) {
	hc := &http.Client{}
	c := NewClient(testKey, WithBaseURL("https://example.test/"), WithTimeout(5*time.Second), WithMaxRetries(-1), WithHTTPClient(hc))
	wantEqual(t, "base URL", c.BaseURL(), "https://example.test")
	wantEqual(t, "timeout", c.timeout, 5*time.Second)
	wantEqual(t, "retries", c.maxRetries, 0)
	if c.httpClient != hc {
		t.Error("the HTTP client was not used")
	}
	d := NewClient(testKey)
	wantEqual(t, "default base URL", d.BaseURL(), DefaultBaseURL)
	wantEqual(t, "default timeout", d.timeout, DefaultTimeout)
	wantEqual(t, "default retries", d.maxRetries, DefaultMaxRetries)
	if strings.Contains(fmt.Sprint(d), testKey) {
		t.Error("the client's String includes the key")
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		body   string
		check  func(err error) bool
	}{
		{400, `{"return_code":"E1","message":"radius out of range"}`, func(err error) bool { var e *InvalidRequestError; return errors.As(err, &e) }},
		{422, `{"message":"Idempotency-Key reused"}`, func(err error) bool { var e *InvalidRequestError; return errors.As(err, &e) }},
		{401, `{"message":"Unauthorised"}`, func(err error) bool { var e *AuthenticationError; return errors.As(err, &e) }},
		{403, `{"return_code":"E1","message":"wrong mode"}`, func(err error) bool { var e *PermissionError; return errors.As(err, &e) }},
		{404, `{"return_code":"E1","message":"not found"}`, func(err error) bool { var e *NotFoundError; return errors.As(err, &e) }},
		{409, `{"message":"in progress"}`, func(err error) bool { var e *ConflictError; return errors.As(err, &e) }},
		{429, `{"message":"slow down"}`, func(err error) bool {
			var e *RateLimitError
			return errors.As(err, &e) && e.RetryAfter == 7*time.Second
		}},
		{500, `{"message":"boom"}`, func(err error) bool { var e *APIError; return errors.As(err, &e) }},
		{418, `not json`, func(err error) bool { var e *APIError; return errors.As(err, &e) && e.Message == "I'm a teapot" }},
		{200, `{"return_code":"E3","message":"balance too low"}`, func(err error) bool {
			var e *InvalidRequestError
			return errors.As(err, &e) && e.StatusCode == 200 && e.ReturnCode == "E3"
		}},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			f := newFakeAPI(t, reply{Status: tc.status, Body: tc.body, Header: map[string]string{"Retry-After": "7", "X-RateLimit-Remaining": "3"}})
			c, _ := newTestClient(t, f, WithMaxRetries(0))
			_, err := c.Offers.Pause(context.Background(), 1)
			if err == nil {
				t.Fatal("no error")
			}
			if !tc.check(err) {
				t.Fatalf("err = %T %v", err, err)
			}
			var base *AgilisError
			if !errors.As(err, &base) {
				t.Fatal("does not unwrap to *AgilisError")
			}
			wantEqual(t, "status", base.StatusCode, tc.status)
			wantEqual(t, "header", base.Header.Get("X-RateLimit-Remaining"), "3")
			wantEqual(t, "body", string(base.Body), tc.body)
		})
	}
}

func TestSuccessCodes(t *testing.T) {
	f := newFakeAPI(t,
		reply{Body: `{"return_code":"S1","message":"Offer paused"}`, Header: map[string]string{"X-RateLimit-Remaining": "41"}},
		reply{Body: `{"return_code":"S2","message":"Offer resumed"}`},
	)
	c, _ := newTestClient(t, f)
	p, err := c.Offers.Pause(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "status", p.StatusCode, 200)
	wantEqual(t, "remaining", p.RateLimitRemaining(), 41)
	r, err := c.Offers.Resume(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "return_code", r.ReturnCode, "S2")
	wantEqual(t, "no header", r.RateLimitRemaining(), -1)
}

func TestKeyNeverInErrors(t *testing.T) {
	body := `{"return_code":"E1","message":"key ` + testKey + ` is not valid"}`
	f := newFakeAPI(t, reply{Status: 401, Body: body})
	c, _ := newTestClient(t, f, WithMaxRetries(0))
	_, err := c.Categories.List(context.Background())
	if err == nil {
		t.Fatal("no error")
	}
	var base *AgilisError
	errors.As(err, &base)
	for name, text := range map[string]string{
		"Error()": err.Error(),
		"%+v":     fmt.Sprintf("%+v", err),
		"Message": base.Message,
		"Body":    string(base.Body),
		"Data":    fmt.Sprint(base.Data),
	} {
		if strings.Contains(text, testKey) {
			t.Errorf("%s contains the key: %s", name, text)
		}
	}
	if !strings.Contains(err.Error(), "[redacted]") {
		t.Errorf("Error() = %q", err.Error())
	}

	// A connection error whose text names the key.
	c2 := NewClient(testKey, WithHTTPClient(doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("proxy refused " + testKey)
	})), WithMaxRetries(0))
	_, err = c2.Categories.List(context.Background())
	var ce *ConnectionError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %T", err)
	}
	if strings.Contains(err.Error(), testKey) {
		t.Errorf("connection error contains the key: %s", err)
	}
	if strings.Contains(c2.String(), testKey) {
		t.Error("String() contains the key")
	}
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

func TestIdempotencyKeyKeptAcrossRetries(t *testing.T) {
	f := newFakeAPI(t,
		reply{Status: 503, Body: `{"message":"unavailable"}`},
		reply{Status: 429, Body: `{"message":"slow down"}`, Header: map[string]string{"Retry-After": "2"}},
		reply{Body: `{"return_code":"S1","offer_id":46}`},
	)
	c, s := newTestClient(t, f)
	res, err := c.Offers.Create(context.Background(), &CreateOfferParams{Title: "x", Image: FileFromReader("a.png", strings.NewReader("img"))})
	if err != nil {
		t.Fatal(err)
	}
	all := f.all()
	if len(all) != 3 {
		t.Fatalf("%d requests, want 3", len(all))
	}
	key := all[0].Header.Get("Idempotency-Key")
	for i, r := range all {
		wantEqual(t, fmt.Sprintf("key %d", i), r.Header.Get("Idempotency-Key"), key)
		_, files := parseMultipart(t, r)
		wantEqual(t, fmt.Sprintf("image %d", i), string(files["image"].Data), "img")
	}
	wantEqual(t, "result key", res.IdempotencyKey, key)
	waits := s.all()
	if len(waits) != 2 || waits[0] != 500*time.Millisecond || waits[1] != 2*time.Second {
		t.Errorf("waits = %v, want [500ms 2s]", waits)
	}
}

func TestWhatIsRetried(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name     string
		status   int
		call     func(c *Client) error
		requests int
	}{
		{"GET 500", 500, func(c *Client) error { _, err := c.Categories.List(ctx); return err }, 3},
		{"GET 502", 502, func(c *Client) error { _, err := c.Categories.List(ctx); return err }, 3},
		{"GET 504", 504, func(c *Client) error { _, err := c.Categories.List(ctx); return err }, 3},
		{"GET 501 is not", 501, func(c *Client) error { _, err := c.Categories.List(ctx); return err }, 1},
		{"GET 400 is not", 400, func(c *Client) error { _, err := c.Categories.List(ctx); return err }, 1},
		{"DELETE 503", 503, func(c *Client) error { _, err := c.Offers.Delete(ctx, 1, nil); return err }, 3},
		{"POST with key 500", 500, func(c *Client) error {
			_, err := c.Events.Publish(ctx, &PublishEventParams{Title: "x"})
			return err
		}, 3},
		{"POST without key 500 is not", 500, func(c *Client) error { _, err := c.Offers.Pause(ctx, 1); return err }, 1},
		{"POST without key 429", 429, func(c *Client) error { _, err := c.Offers.Pause(ctx, 1); return err }, 3},
		{"PATCH 503 is not", 503, func(c *Client) error {
			_, err := c.WebhookEndpoints.Update(ctx, "we_1", &UpdateWebhookEndpointParams{Status: String("enabled")})
			return err
		}, 1},
		{"PATCH 429", 429, func(c *Client) error {
			_, err := c.WebhookEndpoints.Update(ctx, "we_1", &UpdateWebhookEndpointParams{Status: String("enabled")})
			return err
		}, 3},
		{"edit 503 is not", 503, func(c *Client) error {
			_, err := c.Events.Edit(ctx, 1, &EditEventParams{Image: FileFromBytes("a.jpg", []byte("x"))})
			return err
		}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAPI(t, reply{Status: tc.status, Body: `{"message":"failed"}`})
			c, s := newTestClient(t, f)
			if err := tc.call(c); err == nil {
				t.Fatal("no error")
			}
			wantEqual(t, "requests", len(f.all()), tc.requests)
			wantEqual(t, "waits", len(s.all()), tc.requests-1)
		})
	}
}

func TestRetriesThenSucceeds(t *testing.T) {
	f := newFakeAPI(t, reply{Status: 502, Body: `<html>bad gateway</html>`}, reply{Body: `{"categories":[]}`})
	c, s := newTestClient(t, f)
	if _, err := c.Categories.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "requests", len(f.all()), 2)
	wantEqual(t, "waits", len(s.all()), 1)
}

func TestMaxRetriesOption(t *testing.T) {
	f := newFakeAPI(t, reply{Status: 503, Body: `{}`})
	c, _ := newTestClient(t, f, WithMaxRetries(4))
	_, err := c.Categories.List(context.Background())
	var e *APIError
	if !errors.As(err, &e) || e.StatusCode != 503 {
		t.Fatalf("err = %v", err)
	}
	wantEqual(t, "requests", len(f.all()), 5)
}

func TestBackoff(t *testing.T) {
	c := NewClient(testKey)
	c.jitter = func() float64 { return 0.5 }
	wantEqual(t, "attempt 0", c.backoff(0), 500*time.Millisecond)
	wantEqual(t, "attempt 1", c.backoff(1), time.Second)
	wantEqual(t, "attempt 2", c.backoff(2), 2*time.Second)
	wantEqual(t, "attempt 4", c.backoff(4), 8*time.Second)
	wantEqual(t, "attempt 9", c.backoff(9), 8*time.Second)
	c.jitter = func() float64 { return 0 }
	wantEqual(t, "-25%", c.backoff(1), 750*time.Millisecond)
	c.jitter = func() float64 { return 1 }
	wantEqual(t, "+25%", c.backoff(1), 1250*time.Millisecond)
	wantEqual(t, "cap", c.backoff(4), 8*time.Second)
	c = NewClient(testKey)
	for i := 0; i < 100; i++ {
		d := c.backoff(0)
		if d < 375*time.Millisecond || d > 625*time.Millisecond {
			t.Fatalf("backoff(0) = %v", d)
		}
	}
}

func TestRetryAfterHeader(t *testing.T) {
	h := http.Header{}
	if _, ok := retryAfter(h); ok {
		t.Error("an absent header was read")
	}
	h.Set("Retry-After", "12")
	d, ok := retryAfter(h)
	if !ok || d != 12*time.Second {
		t.Errorf("seconds: %v %v", d, ok)
	}
	h.Set("Retry-After", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
	d, ok = retryAfter(h)
	if !ok || d < 59*time.Minute || d > time.Hour {
		t.Errorf("date: %v %v", d, ok)
	}
	h.Set("Retry-After", "soon")
	if _, ok := retryAfter(h); ok {
		t.Error("a bad header was read")
	}
}

func TestConnectionErrorsAreRetried(t *testing.T) {
	calls := 0
	doer := doerFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("connection reset")
	})
	c := NewClient(testKey, WithHTTPClient(doer))
	s := &sleeps{}
	c.sleep = s.sleep
	c.jitter = func() float64 { return 0.5 }
	_, err := c.Offers.Pause(context.Background(), 1)
	var ce *ConnectionError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %T %v", err, err)
	}
	var base *AgilisError
	if !errors.As(err, &base) || base.StatusCode != 0 {
		t.Error("does not unwrap to *AgilisError with status 0")
	}
	wantEqual(t, "calls", calls, 3)
	waits := s.all()
	if len(waits) != 2 || waits[0] != 500*time.Millisecond || waits[1] != time.Second {
		t.Errorf("waits = %v", waits)
	}
}

func TestCancelledContextIsNotRetried(t *testing.T) {
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	doer := doerFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		cancel()
		return nil, r.Context().Err()
	})
	c := NewClient(testKey, WithHTTPClient(doer))
	_, err := c.Categories.List(ctx)
	var ce *ConnectionError
	if !errors.As(err, &ce) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %T %v", err, err)
	}
	wantEqual(t, "calls", calls, 1)
}

func TestTimeoutPerAttempt(t *testing.T) {
	calls := 0
	doer := doerFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	c := NewClient(testKey, WithHTTPClient(doer), WithTimeout(10*time.Millisecond), WithMaxRetries(1))
	c.sleep = (&sleeps{}).sleep
	_, err := c.Categories.List(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	wantEqual(t, "calls", calls, 2)
}

func TestUndecodableSuccess(t *testing.T) {
	f := newFakeAPI(t, reply{Body: `{"categories":"not a list"}`})
	c, _ := newTestClient(t, f)
	_, err := c.Categories.List(context.Background())
	var e *APIError
	if !errors.As(err, &e) || e.StatusCode != 200 {
		t.Fatalf("err = %T %v", err, err)
	}
}

func TestPagination(t *testing.T) {
	f := newFakeAPI(t,
		reply{Body: `{"data":[{"offer_id":1},{"offer_id":2}],"pagination":{"limit":2,"count":2,"has_more":true,"next_cursor":"c2"}}`},
		reply{Body: `{"data":[{"offer_id":3},{"offer_id":4}],"pagination":{"limit":2,"count":2,"has_more":true,"next_cursor":"c3"}}`},
		reply{Body: `{"data":[{"offer_id":5}],"pagination":{"limit":2,"count":1,"has_more":false,"next_cursor":null}}`},
	)
	c, _ := newTestClient(t, f)
	it := c.Offers.ListAutoPaging(context.Background(), &ListOffersParams{Limit: 2, OfferStatus: "active"})
	var ids []string
	for it.Next() {
		ids = append(ids, pathInt(it.Current().OfferID))
	}
	if err := it.Err(); err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "ids", strings.Join(ids, ","), "1,2,3,4,5")
	wantEqual(t, "pages", it.Pages(), 3)
	all := f.all()
	wantEqual(t, "requests", len(all), 3)
	wantEqual(t, "first query", all[0].Query, "environment=TEST&limit=2&offer_status=active")
	wantEqual(t, "second query", all[1].Query, "cursor=c2&environment=TEST&limit=2&offer_status=active")
	wantEqual(t, "third query", all[2].Query, "cursor=c3&environment=TEST&limit=2&offer_status=active")
	if it.Next() {
		t.Error("Next after the end")
	}
}

func TestPaginationEventsAndWebhookEvents(t *testing.T) {
	f := newFakeAPI(t,
		reply{Body: `{"data":[{"event_id":1}],"pagination":{"has_more":true,"next_cursor":"e2"}}`},
		reply{Body: `{"data":[],"pagination":{"has_more":false,"next_cursor":null}}`},
		reply{Body: `{"return_code":"S1","data":[{"id":"whevt_1"}],"pagination":{"has_more":true,"next_cursor":"w2"}}`},
		reply{Body: `{"return_code":"S1","data":[{"id":"whevt_2"}],"pagination":{"has_more":false,"next_cursor":null}}`},
	)
	c, _ := newTestClient(t, f)
	ev := c.Events.ListAutoPaging(context.Background(), nil)
	n := 0
	for ev.Next() {
		n++
	}
	if ev.Err() != nil || n != 1 {
		t.Fatalf("events: n = %d, err = %v", n, ev.Err())
	}
	wh := c.WebhookEvents.ListAutoPaging(context.Background(), &ListWebhookEventsParams{Limit: 1})
	var ids []string
	for wh.Next() {
		ids = append(ids, wh.Current().ID)
	}
	if wh.Err() != nil {
		t.Fatal(wh.Err())
	}
	wantEqual(t, "webhook ids", strings.Join(ids, ","), "whevt_1,whevt_2")
	wantEqual(t, "webhook cursor", f.all()[3].Query, "cursor=w2&limit=1")
}

func TestPaginationError(t *testing.T) {
	f := newFakeAPI(t,
		reply{Body: `{"data":[{"offer_id":1}],"pagination":{"has_more":true,"next_cursor":"c2"}}`},
		reply{Status: 400, Body: `{"message":"Invalid cursor"}`},
	)
	c, _ := newTestClient(t, f)
	it := c.Offers.ListAutoPaging(context.Background(), nil)
	n := 0
	for it.Next() {
		n++
	}
	var e *InvalidRequestError
	if !errors.As(it.Err(), &e) || n != 1 {
		t.Fatalf("n = %d, err = %v", n, it.Err())
	}
}

func TestFileContentTypes(t *testing.T) {
	cases := map[string]string{
		"a.jpg":  "image/jpeg",
		"a.JPEG": "image/jpeg",
		"a.png":  "image/png",
		"a.gif":  "image/gif",
		"a":      "application/octet-stream",
	}
	for name, want := range cases {
		wantEqual(t, name, FileFromBytes(name, nil).ContentType(), want)
	}
	wantEqual(t, "override", FileFromBytes("a.bin", nil).WithContentType("image/png").ContentType(), "image/png")
	wantEqual(t, "path name", FileFromPath("testdata/sample-offer.png").Name(), "sample-offer.png")
	if _, err := FileFromPath("testdata/missing.png").bytes(); err == nil {
		t.Error("a missing file was read")
	}
}
