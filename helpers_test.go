package agilisads

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const testKey = "pk_test_0123456789abcdefSECRET"

// recorded is one request the fake server received.
type recorded struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   []byte
}

// reply is one response the fake server sends.
type reply struct {
	Status int
	Header map[string]string
	Body   string
}

// fakeAPI is an httptest server that records requests and answers with the
// queued replies (the last one repeats).
type fakeAPI struct {
	t        *testing.T
	server   *httptest.Server
	mu       sync.Mutex
	requests []recorded
	replies  []reply
}

func newFakeAPI(t *testing.T, replies ...reply) *fakeAPI {
	t.Helper()
	f := &fakeAPI{t: t, replies: replies}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, recorded{
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.RawQuery,
		Header: r.Header.Clone(),
		Body:   body,
	})
	n := len(f.requests)
	var rep reply
	switch {
	case len(f.replies) == 0:
		rep = reply{Status: 200, Body: `{"return_code":"S1"}`}
	case n <= len(f.replies):
		rep = f.replies[n-1]
	default:
		rep = f.replies[len(f.replies)-1]
	}
	f.mu.Unlock()
	for k, v := range rep.Header {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", "application/json")
	if rep.Status == 0 {
		rep.Status = 200
	}
	w.WriteHeader(rep.Status)
	_, _ = io.WriteString(w, rep.Body)
}

func (f *fakeAPI) all() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.requests...)
}

func (f *fakeAPI) last() recorded {
	f.t.Helper()
	all := f.all()
	if len(all) == 0 {
		f.t.Fatal("no request was made")
	}
	return all[len(all)-1]
}

// sleeps records the waits the client asked for instead of sleeping.
type sleeps struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (s *sleeps) sleep(ctx context.Context, d time.Duration) error {
	s.mu.Lock()
	s.waits = append(s.waits, d)
	s.mu.Unlock()
	return ctx.Err()
}

func (s *sleeps) all() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Duration(nil), s.waits...)
}

// newTestClient returns a client for the fake server that does not sleep and
// uses a fixed jitter of 0.5 (so the backoff is exactly 0.5 s × 2^attempt).
func newTestClient(t *testing.T, f *fakeAPI, opts ...Option) (*Client, *sleeps) {
	t.Helper()
	all := append([]Option{WithBaseURL(f.server.URL)}, opts...)
	c := NewClient(testKey, all...)
	s := &sleeps{}
	c.sleep = s.sleep
	c.jitter = func() float64 { return 0.5 }
	return c, s
}

type part struct {
	FileName    string
	ContentType string
	Data        []byte
}

// parseMultipart returns the values and files of a multipart body, in order.
func parseMultipart(t *testing.T, rec recorded) (map[string][]string, map[string]part) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(rec.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		t.Fatalf("Content-Type = %q, want multipart/form-data", rec.Header.Get("Content-Type"))
	}
	values := map[string][]string{}
	files := map[string]part{}
	mr := multipart.NewReader(bytes.NewReader(rec.Body), params["boundary"])
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading multipart: %v", err)
		}
		data, _ := io.ReadAll(p)
		if p.FileName() != "" {
			files[p.FormName()] = part{FileName: p.FileName(), ContentType: p.Header.Get("Content-Type"), Data: data}
			continue
		}
		values[p.FormName()] = append(values[p.FormName()], string(data))
	}
	return values, files
}

func decodeJSONBody(t *testing.T, rec recorded) map[string]any {
	t.Helper()
	if ct := rec.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body, &m); err != nil {
		t.Fatalf("body is not JSON: %v: %s", err, rec.Body)
	}
	return m
}

func wantEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func wantValues(t *testing.T, values map[string][]string, name string, want ...string) {
	t.Helper()
	got := values[name]
	if strings.Join(got, "|") != strings.Join(want, "|") || len(got) != len(want) {
		t.Errorf("form %s = %q, want %q", name, got, want)
	}
}

func jsonUnmarshalString(s string, v any) error { return json.Unmarshal([]byte(s), v) }

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
