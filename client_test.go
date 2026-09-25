package pokemontcgapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// One test per item of packages/CONFORMANCE.md, named after it.

type fixture struct {
	Status   int               `json:"status"`
	Headers  map[string]string `json:"headers"`
	Body     json.RawMessage   `json:"body"`
	BodyText *string           `json:"body_text"`
}

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "fixtures", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// serve writes a fixture, rewriting the API host in the body to the test
// server so that links.next can be followed verbatim.
func serve(w http.ResponseWriter, f fixture, host string) {
	for k, v := range f.Headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(f.Status)
	if f.BodyText != nil {
		_, _ = io.WriteString(w, *f.BodyText)
		return
	}
	body := bytes.ReplaceAll(f.Body, []byte(defaultBaseURL), []byte(host))
	_, _ = w.Write(body)
}

type recorded struct {
	Method string
	URL    string // path + query
	Header http.Header
	Body   []byte
}

type testServer struct {
	*httptest.Server
	requests []recorded
	handler  func(w http.ResponseWriter, r *http.Request, srv *testServer)
}

func newServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, srv *testServer)) *testServer {
	t.Helper()
	ts := &testServer{handler: handler}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ts.requests = append(ts.requests, recorded{Method: r.Method, URL: r.URL.RequestURI(), Header: r.Header.Clone(), Body: body})
		ts.handler(w, r, ts)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// fixtureServer answers every request with one fixture.
func fixtureServer(t *testing.T, name string) *testServer {
	t.Helper()
	f := loadFixture(t, name)
	return newServer(t, func(w http.ResponseWriter, r *http.Request, srv *testServer) { serve(w, f, srv.URL) })
}

func newClient(srv *testServer, opts ...Option) *Client {
	base := []Option{WithAPIKey("test"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithMaxRetries(0)}
	return New(append(base, opts...)...)
}

func noSleep(t *testing.T) *[]time.Duration {
	t.Helper()
	var slept []time.Duration
	previous := sleep
	sleep = func(ctx context.Context, d time.Duration) error { slept = append(slept, d); return nil }
	t.Cleanup(func() { sleep = previous })
	return &slept
}

// emptyServer answers every request with an empty collection: enough for
// tests that only look at the request.
func emptyServer(t *testing.T) *testServer {
	t.Helper()
	return newServer(t, func(w http.ResponseWriter, r *http.Request, srv *testServer) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"meta":{"limit":0,"count":0,"has_more":false},"requested":0,"found":0}`))
	})
}

// pagedServer serves the three sets pages by cursor, and INVALID_CURSOR for
// any cursor it did not hand out: exactly what the API does to a rebuilt one.
func pagedServer(t *testing.T) *testServer {
	t.Helper()
	pages := map[string]fixture{
		"":                 loadFixture(t, "sets-page-1"),
		"eyJwIjp71fQ.sig1": loadFixture(t, "sets-page-2"),
		"eyJwIjp72fQ.sig2": loadFixture(t, "sets-page-3"),
	}
	return newServer(t, func(w http.ResponseWriter, r *http.Request, srv *testServer) {
		f, ok := pages[r.URL.Query().Get("cursor")]
		if !ok {
			http.Error(w, `{"error":{"code":"INVALID_CURSOR","message":"rebuilt"}}`, http.StatusBadRequest)
			return
		}
		serve(w, f, srv.URL)
	})
}

func TestParamsEncoding(t *testing.T) {
	srv := emptyServer(t)
	c := newClient(srv)
	ctx := context.Background()

	_, err := c.Cards.Search(ctx, &CardListParams{Q: "name:charizard", Select: []string{"id", "name"}, Include: []string{"index"}, Set: []string{"evs", "fst"}, Limit: 0, Lang: ""})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Sets.Cards(ctx, "sv 8/x", &CardListParams{Set: []string{"ignored"}, Include: []string{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Prices.Movers(ctx, &MoversParams{MinValue: 2.5, Direction: "losers"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Changes(ctx, &ChangesParams{Since: 42}); err != nil {
		t.Fatal(err)
	}

	got := []string{srv.requests[0].URL, srv.requests[1].URL, srv.requests[2].URL, srv.requests[3].URL}
	want := []string{
		"/v1/cards?include=index&q=name%3Acharizard&select=id%2Cname&set=evs%2Cfst",
		"/v1/sets/sv%208%2Fx/cards",
		"/v1/prices/movers?direction=losers&min_value=2.5",
		"/v1/changes?since=42",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("request %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestEnvKeyFallback(t *testing.T) {
	srv := fixtureServer(t, "status")
	t.Setenv("PTCG_API_KEY", "from-env")
	c := New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if _, err := c.Status(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := srv.requests[0].Header.Get("X-Api-Key"); got != "from-env" {
		t.Errorf("x-api-key: %q", got)
	}
	t.Setenv("PTCG_API_KEY", "")
	c = New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if _, err := c.Status(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, present := srv.requests[1].Header["X-Api-Key"]; present {
		t.Error("x-api-key sent without a key")
	}
}

func TestHeadersSent(t *testing.T) {
	srv := fixtureServer(t, "status")
	c := newClient(srv)
	if _, err := c.Status(context.Background()); err != nil {
		t.Fatal(err)
	}
	h := srv.requests[0].Header
	if h.Get("Accept") != "application/json" {
		t.Errorf("accept: %q", h.Get("Accept"))
	}
	if h.Get("X-Api-Key") != "test" {
		t.Errorf("x-api-key: %q", h.Get("X-Api-Key"))
	}
	if h.Get("User-Agent") != "pokemontcgapi-go/"+Version {
		t.Errorf("user-agent: %q", h.Get("User-Agent"))
	}
}

func TestResponseInfo(t *testing.T) {
	srv := fixtureServer(t, "card-bs-4-with-prices")
	c := newClient(srv)
	card, err := c.Cards.Get(context.Background(), "bs-4", &CardGetParams{Include: []string{"prices"}})
	if err != nil {
		t.Fatal(err)
	}
	if card.Name != "Charizard" || card.IndexEUR == nil || *card.IndexEUR != 561.84 || len(card.Prices) != 6 {
		t.Errorf("card decoded wrong: %+v", card)
	}
	info := c.LastResponse()
	if info == nil {
		t.Fatal("no last response")
	}
	if info.Status != 200 || info.RequestID != fmt.Sprintf("%032x", 20) || info.ErrorCode != "" {
		t.Errorf("info: %+v", info)
	}
	if *info.CreditsCost != 3 || *info.QuotaLimit != 800 || *info.QuotaRemaining != 780 || *info.RateLimitRemaining != 4 {
		t.Errorf("numbers: %+v", info)
	}
	if info.QuotaReset != "2026-10-01T00:00:00Z" || info.TrialExpiresAt != "2026-10-18T09:00:00Z" {
		t.Errorf("strings: %+v", info)
	}
	if strings.Join(info.PlanWithheld, "|") != "graded|non_english_locales" {
		t.Errorf("withheld: %v", info.PlanWithheld)
	}
	if !strings.HasSuffix(info.URL, "/v1/cards/bs-4?include=prices") {
		t.Errorf("url: %s", info.URL)
	}

	srv2 := newServer(t, func(w http.ResponseWriter, r *http.Request, srv *testServer) {
		w.Header().Set("X-Credits-Cost", "many")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	c2 := newClient(srv2)
	if _, err := c2.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	if info := c2.LastResponse(); info.CreditsCost != nil || len(info.PlanWithheld) != 0 {
		t.Errorf("non-numeric header must be nil, absent withheld empty: %+v", info)
	}
}

func TestOnResponseCalledOnErrors(t *testing.T) {
	srv := fixtureServer(t, "404-card-not-found-did-you-mean")
	var seen []ResponseInfo
	c := newClient(srv, WithOnResponse(func(info ResponseInfo) { seen = append(seen, info) }))
	_, err := c.Cards.Get(context.Background(), "bs-4x", nil)
	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("want NotFoundError, got %T %v", err, err)
	}
	if len(seen) != 1 || seen[0].Status != 404 || seen[0].ErrorCode != "CARD_NOT_FOUND" {
		t.Errorf("on_response: %+v", seen)
	}
	if notFound.Details["did_you_mean"] != "bs-4" || notFound.RequestID == "" {
		t.Errorf("details: %+v", notFound.APIError)
	}
}

func TestErrorMappingTable(t *testing.T) {
	ctx := context.Background()
	type check func(t *testing.T, err error)
	as := func(target any) check {
		return func(t *testing.T, err error) {
			t.Helper()
			if !errors.As(err, target) {
				t.Errorf("want %T, got %T: %v", target, err, err)
			}
		}
	}
	cases := []struct {
		fixture string
		checks  []check
	}{
		{"429-quota-exceeded-next-step", []check{as(new(*QuotaExceededError)), func(t *testing.T, err error) {
			var limited *RateLimitedError
			if errors.As(err, &limited) {
				t.Error("quota must not be a rate limit")
			}
		}}},
		{"429-rate-limited", []check{as(new(*RateLimitedError)), func(t *testing.T, err error) {
			var limited *RateLimitedError
			errors.As(err, &limited)
			if !limited.HasRetryAfter || limited.RetryAfter != time.Second {
				t.Errorf("retry-after: %+v", limited)
			}
		}}},
		{"403-plan-required-next-step", []check{as(new(*PlanRequiredError)), as(new(*PermissionDeniedError)), as(new(*APIError))}},
		{"403-trial-expired", []check{as(new(*TrialExpiredError)), as(new(*PermissionDeniedError))}},
		{"401-missing-key", []check{as(new(*AuthenticationError))}},
		{"401-invalid-key", []check{as(new(*AuthenticationError))}},
		{"404-card-not-found-did-you-mean", []check{as(new(*NotFoundError))}},
		{"400-invalid-parameter", []check{as(new(*InvalidRequestError)), func(t *testing.T, err error) {
			var invalid *InvalidRequestError
			errors.As(err, &invalid)
			if invalid.Field() != "page" {
				t.Errorf("field: %q", invalid.Field())
			}
		}}},
		{"500-html", []check{as(new(*ServerError))}},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			srv := fixtureServer(t, tc.fixture)
			_, err := newClient(srv).Prices.Movers(ctx, nil)
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, chk := range tc.checks {
				chk(t, err)
			}
			var api *APIError
			if !errors.As(err, &api) || !strings.HasPrefix(err.Error(), api.Code+": ") {
				t.Errorf("message: %q", err.Error())
			}
		})
	}
	// Code before status: a 400 whose code ends in _NOT_FOUND is a NotFoundError;
	// UPGRADE_REQUIRED and a 403 without a code map by status.
	body := errorBody{Code: "SET_NOT_FOUND", Message: "no"}
	var notFound *NotFoundError
	if !errors.As(toAPIError(400, body, nil), &notFound) {
		t.Error("_NOT_FOUND code must win over the status")
	}
	var upgrade *UpgradeRequiredError
	err := toAPIError(403, errorBody{Code: "UPGRADE_REQUIRED", Message: "no", Details: map[string]any{"permitted_window": "30d"}}, nil)
	if !errors.As(err, &upgrade) || upgrade.PermittedWindow() != "30d" {
		t.Errorf("upgrade: %v", err)
	}
	var denied *PermissionDeniedError
	if !errors.As(toAPIError(403, errorBody{Code: "FORBIDDEN", Message: "no"}, nil), &denied) {
		t.Error("403 must be PermissionDeniedError")
	}
	var invalid *InvalidRequestError
	if !errors.As(toAPIError(422, errorBody{Code: "VALIDATION", Message: "no"}, nil), &invalid) {
		t.Error("422 must be InvalidRequestError")
	}
	if base := toAPIError(302, errorBody{Code: "ODD", Message: "no"}, nil); errors.As(base, &invalid) {
		t.Error("a non-4xx/5xx status must be the base error")
	}
}

func TestErrorBodyFallback(t *testing.T) {
	srv := fixtureServer(t, "500-html")
	_, err := newClient(srv).Status(context.Background())
	var server *ServerError
	if !errors.As(err, &server) {
		t.Fatalf("got %T %v", err, err)
	}
	if server.Code != "HTTP_502" || server.Message != "Bad Gateway" || server.Status != 502 {
		t.Errorf("fallback: %+v", server.APIError)
	}
}

func TestNextStepAccessors(t *testing.T) {
	plans := "https://example.test/pricing"
	mk := func(step any) *APIError {
		var api *APIError
		errors.As(toAPIError(403, errorBody{Code: "PLAN_REQUIRED", Message: "Denied", Details: map[string]any{"next_step": step}}, nil), &api)
		return api
	}
	full := map[string]any{
		"action": "subscribe", "actor": "account_owner", "plans_url": plans,
		"checkout_url": "https://example.test/subscribe?plan=custom&interval=monthly",
		"handoff":      "The account owner needs to open https://example.test/subscribe.",
		"plan":         map[string]any{"code": "developer", "name": "Developer", "credits_per_month": 50000.0},
	}
	e := mk(full)
	step := e.NextStep()
	if step == nil || step.Handoff != full["handoff"] || step.CheckoutURL != full["checkout_url"] || step.PlansURL != plans {
		t.Fatalf("next step: %+v", step)
	}
	if step.Plan == nil || step.Plan.Code != "developer" || *step.Plan.CreditsPerMonth != 50000 {
		t.Errorf("plan: %+v", step.Plan)
	}
	if e.Handoff() != step.Handoff || e.CheckoutURL() != step.CheckoutURL || e.ActionURL() != step.CheckoutURL {
		t.Errorf("accessors: %q %q %q", e.Handoff(), e.CheckoutURL(), e.ActionURL())
	}

	for _, malformed := range []any{nil, "wrong", map[string]any{}, map[string]any{"action": "pay", "actor": "agent"},
		map[string]any{"action": "subscribe", "actor": "account_owner", "plans_url": plans, "handoff": 5}} {
		e := mk(malformed)
		if e.NextStep() != nil || e.CheckoutURL() != "" || e.ActionURL() != "" || e.Handoff() != "" {
			t.Errorf("malformed %v must yield nothing", malformed)
		}
	}

	for _, action := range []string{"verify_email", "contact_support", "contact_sales"} {
		e := mk(map[string]any{"action": action, "actor": "account_owner", "plans_url": plans, "handoff": "Contact the owner."})
		if e.NextStep() == nil || e.NextStep().Action != action || e.CheckoutURL() != "" || e.ActionURL() != "" {
			t.Errorf("%s must expose no URL", action)
		}
	}

	table := []struct{ action, field, url string }{
		{"subscribe", "checkout_url", "https://example.test/subscribe?plan=developer&interval=monthly"},
		{"upgrade", "manage_url", "https://example.test/account"},
		{"verify_email", "verify_url", "https://example.test/account"},
		{"contact_sales", "contact_url", "mailto:sales@example.test"},
		{"contact_support", "contact_url", "mailto:support@example.test"},
	}
	for _, tc := range table {
		step := map[string]any{"action": tc.action, "actor": "account_owner", "plans_url": plans, "handoff": "Contact the owner.", tc.field: tc.url}
		e := mk(step)
		if e.ActionURL() != tc.url {
			t.Errorf("%s: action url %q", tc.action, e.ActionURL())
		}
		wantCheckout := ""
		if tc.action == "subscribe" {
			wantCheckout = tc.url
		}
		if e.CheckoutURL() != wantCheckout {
			t.Errorf("%s: checkout %q", tc.action, e.CheckoutURL())
		}
		step[tc.field] = 123
		if mk(step).ActionURL() != "" {
			t.Errorf("%s: non-string url must be empty", tc.action)
		}
	}
}

func TestRetryPolicy(t *testing.T) {
	ctx := context.Background()
	sequence := func(t *testing.T, names ...string) *testServer {
		t.Helper()
		fixtures := make([]fixture, len(names))
		for i, n := range names {
			fixtures[i] = loadFixture(t, n)
		}
		var i atomic.Int32
		return newServer(t, func(w http.ResponseWriter, r *http.Request, srv *testServer) {
			n := int(i.Add(1)) - 1
			if n >= len(fixtures) {
				n = len(fixtures) - 1
			}
			serve(w, fixtures[n], srv.URL)
		})
	}

	t.Run("429 then 200 is retried once", func(t *testing.T) {
		slept := noSleep(t)
		srv := sequence(t, "429-rate-limited", "status")
		if _, err := newClient(srv, WithMaxRetries(2)).Status(ctx); err != nil {
			t.Fatal(err)
		}
		if len(srv.requests) != 2 || len(*slept) != 1 || (*slept)[0] != time.Second {
			t.Errorf("requests %d slept %v", len(srv.requests), *slept)
		}
	})
	t.Run("5xx then 200", func(t *testing.T) {
		slept := noSleep(t)
		srv := sequence(t, "500-html", "status")
		if _, err := newClient(srv, WithMaxRetries(2)).Status(ctx); err != nil {
			t.Fatal(err)
		}
		if len(srv.requests) != 2 || len(*slept) != 1 || (*slept)[0] < 0 || (*slept)[0] >= 500*time.Millisecond {
			t.Errorf("requests %d slept %v", len(srv.requests), *slept)
		}
	})
	t.Run("quota exceeded is never retried", func(t *testing.T) {
		noSleep(t)
		srv := sequence(t, "429-quota-exceeded-next-step", "status")
		var quota *QuotaExceededError
		if _, err := newClient(srv, WithMaxRetries(2)).Status(ctx); !errors.As(err, &quota) {
			t.Fatalf("got %v", err)
		}
		if len(srv.requests) != 1 {
			t.Errorf("requests %d", len(srv.requests))
		}
	})
	t.Run("4xx is never retried", func(t *testing.T) {
		noSleep(t)
		srv := sequence(t, "404-card-not-found-did-you-mean", "status")
		if _, err := newClient(srv, WithMaxRetries(2)).Status(ctx); err == nil || len(srv.requests) != 1 {
			t.Errorf("err %v requests %d", err, len(srv.requests))
		}
	})
	t.Run("max retries bounds the attempts", func(t *testing.T) {
		slept := noSleep(t)
		srv := sequence(t, "500-html")
		var server *ServerError
		if _, err := newClient(srv, WithMaxRetries(2)).Status(ctx); !errors.As(err, &server) {
			t.Fatalf("got %v", err)
		}
		if len(srv.requests) != 3 || len(*slept) != 2 || (*slept)[1] >= time.Second {
			t.Errorf("requests %d slept %v", len(srv.requests), *slept)
		}
	})
	t.Run("Retry-After is honoured and capped at 60 s", func(t *testing.T) {
		slept := noSleep(t)
		f := loadFixture(t, "429-rate-limited")
		f.Headers["retry-after"] = "120"
		srv := newServer(t, func(w http.ResponseWriter, r *http.Request, srv *testServer) { serve(w, f, srv.URL) })
		_, _ = newClient(srv, WithMaxRetries(1)).Status(ctx)
		if len(*slept) != 1 || (*slept)[0] != 60*time.Second {
			t.Errorf("slept %v", *slept)
		}
	})
	t.Run("408 is retried", func(t *testing.T) {
		noSleep(t)
		f := fixture{Status: 408, Headers: map[string]string{"content-type": "application/json"}, Body: json.RawMessage(`{"error":{"code":"REQUEST_TIMEOUT","message":"slow"}}`)}
		ok := loadFixture(t, "status")
		var i atomic.Int32
		srv := newServer(t, func(w http.ResponseWriter, r *http.Request, srv *testServer) {
			if i.Add(1) == 1 {
				serve(w, f, srv.URL)
				return
			}
			serve(w, ok, srv.URL)
		})
		if _, err := newClient(srv, WithMaxRetries(1)).Status(ctx); err != nil || len(srv.requests) != 2 {
			t.Errorf("err %v requests %d", err, len(srv.requests))
		}
	})
	t.Run("POST is never retried", func(t *testing.T) {
		noSleep(t)
		srv := sequence(t, "500-html", "vision-ambiguous")
		_, err := newClient(srv, WithMaxRetries(2)).Vision.Identify(ctx, strings.NewReader("jpeg"), nil)
		if err == nil || len(srv.requests) != 1 {
			t.Errorf("err %v requests %d", err, len(srv.requests))
		}
	})
	t.Run("connection failure is retried", func(t *testing.T) {
		slept := noSleep(t)
		srv := fixtureServer(t, "status")
		bad := New(WithAPIKey("t"), WithBaseURL("http://127.0.0.1:1"), WithMaxRetries(1), WithHTTPClient(srv.Client()))
		var conn *ConnectionError
		if _, err := bad.Status(ctx); !errors.As(err, &conn) || len(*slept) != 1 {
			t.Errorf("err %v slept %v", err, *slept)
		}
	})
	t.Run("sleep respects the caller's context", func(t *testing.T) {
		srv := sequence(t, "500-html")
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := newClient(srv, WithMaxRetries(2)).Status(cctx)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("got %v", err)
		}
	})
}

func TestPaginationFollowsLinksNext(t *testing.T) {
	srv := pagedServer(t)
	c := newClient(srv)
	ctx := context.Background()

	first, err := c.Sets.List(ctx, &SetListParams{Region: "JP", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !first.HasMore() || len(first.Data) != 2 || first.Data[0].Code != "m6" || *first.Meta.TotalCount != 5 {
		t.Fatalf("first page: %+v", first)
	}
	second, err := first.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	third, err := second.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if third.HasMore() {
		t.Error("third page must be the last")
	}
	if fourth, err := third.Next(ctx); fourth != nil || err != nil {
		t.Errorf("after the last page: %v %v", fourth, err)
	}

	// The second and third requests are the links.next of the previous page, byte for byte.
	for i, want := range []string{first.nextURL, second.nextURL} {
		u, _ := url.Parse(want)
		if got := srv.requests[i+1].URL; got != u.RequestURI() {
			t.Errorf("request %d: got %q want %q", i+1, got, u.RequestURI())
		}
	}

	var codes []string
	for set, err := range first.All(ctx) {
		if err != nil {
			t.Fatal(err)
		}
		codes = append(codes, set.Code)
	}
	if strings.Join(codes, ",") != "m6,m5,m4,m3,m2" {
		t.Errorf("walk: %v", codes)
	}
	if len(srv.requests) != 5 {
		t.Errorf("requests: %d", len(srv.requests))
	}
}

func TestToListMax(t *testing.T) {
	srv := pagedServer(t)
	first, err := newClient(srv).Sets.List(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := first.Collect(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[2].Code != "m4" || len(srv.requests) != 2 {
		t.Errorf("collect: %d items, %d requests", len(got), len(srv.requests))
	}
	all, err := first.Collect(context.Background(), 100)
	if err != nil || len(all) != 5 {
		t.Errorf("collect all: %d %v", len(all), err)
	}
}

func TestEtag304(t *testing.T) {
	page := loadFixture(t, "sets-page-1")
	notModified := loadFixture(t, "304")
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request, srv *testServer) {
		if r.Header.Get("If-None-Match") == page.Headers["etag"] {
			serve(w, notModified, srv.URL)
			return
		}
		serve(w, page, srv.URL)
	})
	ctx := context.Background()

	c := newClient(srv, WithETagCache())
	first, err := c.Sets.List(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Sets.List(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(srv.requests) != 2 || srv.requests[0].Header.Get("If-None-Match") != "" || srv.requests[1].Header.Get("If-None-Match") != `W/"sets-1-gzip"` {
		t.Errorf("conditional headers: %v", srv.requests)
	}
	if c.LastResponse().Status != 304 || len(second.Data) != 2 || second.Data[0].Code != first.Data[0].Code || !second.HasMore() {
		t.Errorf("304 must replay the cached body: %+v", second)
	}

	// POST never uses the cache, and without the cache nothing conditional is sent.
	if _, err := c.Vision.Identify(ctx, strings.NewReader("x"), nil); err == nil || srv.requests[2].Header.Get("If-None-Match") != "" {
		t.Errorf("post: %v %v", err, srv.requests[2].Header)
	}
	plain := newClient(srv)
	_, _ = plain.Sets.List(ctx, nil)
	_, _ = plain.Sets.List(ctx, nil)
	if srv.requests[4].Header.Get("If-None-Match") != "" {
		t.Error("cache off must not send If-None-Match")
	}
}

func TestBatchLimits(t *testing.T) {
	srv := fixtureServer(t, "batch-missing")
	c := newClient(srv)
	ctx := context.Background()

	empty, err := c.Cards.Batch(ctx, nil, nil)
	if err != nil || empty.Requested != 0 || empty.Found != 0 || len(empty.Data) != 0 || len(srv.requests) != 0 {
		t.Errorf("empty: %+v %v (%d requests)", empty, err, len(srv.requests))
	}
	ids := make([]string, 101)
	for i := range ids {
		ids[i] = "x"
	}
	if _, err := c.Cards.Batch(ctx, ids, nil); !errors.Is(err, ErrTooManyIDs) || len(srv.requests) != 0 {
		t.Errorf("101 ids: %v", err)
	}
	if _, err := c.Cards.Batch(ctx, ids[:100], nil); err != nil {
		t.Fatal(err)
	}
	if got := srv.requests[0].URL; got != "/v1/cards/batch?ids="+url.QueryEscape(strings.Repeat("x,", 99)+"x") {
		t.Errorf("100 ids: %s", got)
	}

	result, err := c.Cards.Batch(ctx, []string{"sv8-116", "sv8-100", "inventato-xyz"}, &CardGetParams{Include: []string{"index"}})
	if err != nil {
		t.Fatal(err)
	}
	if srv.requests[1].URL != "/v1/cards/batch?ids=sv8-116%2Csv8-100%2Cinventato-xyz&include=index" {
		t.Errorf("url: %s", srv.requests[1].URL)
	}
	if result.Requested != 3 || result.Found != 1 || len(result.Missing) != 2 || result.Missing[0].SuggestedID != "ssp-116" || result.Missing[1].SuggestedID != "" || strings.Join(result.Withheld, "") != "index" {
		t.Errorf("batch: %+v", result)
	}
}

func TestPricesCurrentLimits(t *testing.T) {
	withheld := loadFixture(t, "prices-card-withheld")
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request, srv *testServer) {
		if r.URL.Path == "/v1/prices/current" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[],"requested":2,"found":0,"missing":[{"id":"base1-4"},{"id":"bs-4"}]}`))
			return
		}
		serve(w, withheld, srv.URL)
	})
	c := newClient(srv)
	ctx := context.Background()
	empty, err := c.Prices.Current(ctx, []string{}, nil)
	if err != nil || empty.Requested != 0 || len(srv.requests) != 0 {
		t.Errorf("empty: %+v %v", empty, err)
	}
	ids := make([]string, 51)
	for i := range ids {
		ids[i] = "x"
	}
	if _, err := c.Prices.Current(ctx, ids, nil); !errors.Is(err, ErrTooManyIDs) || len(srv.requests) != 0 {
		t.Errorf("51 ids: %v", err)
	}
	if _, err := c.Prices.Current(ctx, []string{"base1-4", "bs-4"}, &PriceFilterParams{Source: "CARDMARKET"}); err != nil {
		t.Fatal(err)
	}
	if srv.requests[0].URL != "/v1/prices/current?ids=base1-4%2Cbs-4&source=CARDMARKET" {
		t.Errorf("url: %s", srv.requests[0].URL)
	}
	prices, err := c.Prices.Card(ctx, "bs-4", nil)
	if err != nil {
		t.Fatal(err)
	}
	if prices.Data.Index == nil || prices.Data.Index.EUR != 556.23 || len(prices.Data.Quotes) != 4 || strings.Join(prices.Meta.Withheld, "|") != "graded|non_english_locales" {
		t.Errorf("prices: %+v", prices)
	}
}

func TestReferenceSingleRequest(t *testing.T) {
	srv := fixtureServer(t, "reference")
	c := newClient(srv)
	ctx := context.Background()
	all, err := c.Reference.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	types, _ := c.Reference.Types(ctx)
	subtypes, _ := c.Reference.Subtypes(ctx)
	supertypes, _ := c.Reference.Supertypes(ctx)
	rarities, _ := c.Reference.Rarities(ctx)
	if _, err := c.Reference.All(ctx); err != nil {
		t.Fatal(err)
	}
	if len(srv.requests) != 1 || srv.requests[0].URL != "/v1/reference" {
		t.Errorf("requests: %v", srv.requests)
	}
	if len(all["locales"]) != 9 || len(types) != 11 || len(subtypes) == 0 || len(supertypes) == 0 || len(rarities) == 0 {
		t.Errorf("vocabularies: %d %d %d %d %d", len(all["locales"]), len(types), len(subtypes), len(supertypes), len(rarities))
	}
	if missing, err := c.Reference.list(ctx, "nope"); err != nil || missing == nil || len(missing) != 0 {
		t.Errorf("unknown key: %v %v", missing, err)
	}

	// A failed request is not cached.
	var i atomic.Int32
	bad := loadFixture(t, "500-html")
	good := loadFixture(t, "reference")
	srv2 := newServer(t, func(w http.ResponseWriter, r *http.Request, srv *testServer) {
		if i.Add(1) == 1 {
			serve(w, bad, srv.URL)
			return
		}
		serve(w, good, srv.URL)
	})
	c2 := newClient(srv2)
	if _, err := c2.Reference.Types(ctx); err == nil {
		t.Fatal("first call must fail")
	}
	if types, err := c2.Reference.Types(ctx); err != nil || len(types) != 11 {
		t.Errorf("second call: %v %v", types, err)
	}
}

func TestVisionMultipart(t *testing.T) {
	srv := fixtureServer(t, "vision-ambiguous")
	c := newClient(srv)
	resp, err := c.Vision.Identify(context.Background(), bytes.NewReader([]byte("\xff\xd8jpeg")), &IdentifyOptions{TopK: 5, Set: "sv3"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Data.Decision != VisionAmbiguous || resp.Data.ID != nil || len(resp.Data.Candidates) != 2 || resp.Meta.OCR == nil || resp.Meta.OCR.Reason != "multiple" {
		t.Errorf("response: %+v", resp)
	}
	req := srv.requests[0]
	if req.Method != http.MethodPost || req.URL != "/v1/vision/identify" {
		t.Errorf("request: %s %s", req.Method, req.URL)
	}
	mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		t.Fatalf("content-type: %s", req.Header.Get("Content-Type"))
	}
	reader := multipart.NewReader(bytes.NewReader(req.Body), params["boundary"])
	form, err := reader.ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	image := form.File["image"]
	if len(image) != 1 || image[0].Filename != "card" || image[0].Size != 6 {
		t.Errorf("image part: %+v", image)
	}
	if form.Value["top_k"][0] != "5" || form.Value["set"][0] != "sv3" || len(form.Value["region"]) != 0 {
		t.Errorf("fields: %v", form.Value)
	}

	// No options: only the image part.
	if _, err := c.Vision.Identify(context.Background(), strings.NewReader("x"), nil); err != nil {
		t.Fatal(err)
	}
	_, params, _ = mime.ParseMediaType(srv.requests[1].Header.Get("Content-Type"))
	form, _ = multipart.NewReader(bytes.NewReader(srv.requests[1].Body), params["boundary"]).ReadForm(1 << 20)
	if len(form.Value) != 0 {
		t.Errorf("unexpected fields: %v", form.Value)
	}
}

func TestRouteTable(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var source strings.Builder
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
			raw, _ := os.ReadFile(e.Name())
			source.Write(raw)
			source.WriteByte('\n')
		}
	}
	routes := []string{
		"/v1/cards", "/v1/cards/{id}", "/v1/cards/batch", "/v1/sets", "/v1/sets/{code}", "/v1/sets/{code}/cards",
		"/v1/artists", "/v1/artists/{slug}", "/v1/series", "/v1/sealed", "/v1/sealed/{id}", "/v1/sealed/{id}/prices",
		"/v1/reference", "/v1/status", "/v1/health", "/v1/cards/{id}/prices", "/v1/prices/current",
		"/v1/cards/{id}/prices/history", "/v1/cards/{id}/prices/stats", "/v1/prices/movers", "/v1/prices/sources",
		"/v1/changes", "/v1/vision/identify",
	}
	placeholder := regexp.MustCompile(`\{[a-z_]+\}`)
	for _, route := range routes {
		var pattern strings.Builder
		last := 0
		for _, m := range placeholder.FindAllStringIndex(route, -1) {
			pattern.WriteString(regexp.QuoteMeta(route[last:m[0]]))
			pattern.WriteString(`[^/'"` + "`" + `]+`)
			last = m[1]
		}
		pattern.WriteString(regexp.QuoteMeta(route[last:]))
		if !regexp.MustCompile(pattern.String()).MatchString(source.String()) {
			t.Errorf("no method for %s", route)
		}
	}
	// pathf escapes each segment.
	if got := pathf("/v1/cards/%s/prices", "a b/c"); got != "/v1/cards/a%20b%2Fc/prices" {
		t.Errorf("pathf: %s", got)
	}
}

func TestTimeoutMapsToTimeoutError(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request, srv *testServer) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	c := newClient(srv, WithTimeout(50*time.Millisecond))
	_, err := c.Status(context.Background())
	var timeout *TimeoutError
	var conn *ConnectionError
	if !errors.As(err, &timeout) || !errors.As(err, &conn) || timeout.Timeout != 50*time.Millisecond {
		t.Fatalf("got %T %v", err, err)
	}
	if !strings.Contains(err.Error(), "timed out after 50ms") {
		t.Errorf("message: %s", err)
	}
}

func TestReadmeSections(t *testing.T) {
	raw, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	var headings []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			headings = append(headings, line)
		}
	}
	want := []string{
		"## Get a key", "## Install", "## Use",
		"### Pagination that you never have to think about", "### One call for a hundred cards",
		"### Japanese, and the other seven locales", "### Conditional requests are free",
		"### A photo instead of an id", "### Errors you can branch on",
		"## What this API does not have", "## Prices", "## Credits and quota", "## The change feed",
		"## Also available", "## Build from source", "## Licence",
	}
	if strings.Join(headings, "\n") != strings.Join(want, "\n") {
		t.Errorf("headings:\n%s", strings.Join(headings, "\n"))
	}
	if !strings.Contains(string(raw), "README synced from sdk-typescript README as of commit") {
		t.Error("README must state the TypeScript README commit it was synced from")
	}
}
