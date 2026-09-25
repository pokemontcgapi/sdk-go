package pokemontcgapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ResponseInfo is what the response says in its headers and the body does not
// carry: what the call cost, what is left of the quota, and what the plan
// withheld. PlanWithheld is why it exists: on Cards.Get with Include prices,
// the rows the plan does not cover are missing from the body in silence, and
// only the X-Plan-Withheld header says so.
type ResponseInfo struct {
	URL       string
	Status    int
	RequestID string
	// ErrorCode equals the error's Code on API errors; empty on success.
	ErrorCode string
	// CreditsCost is the credits charged by this call. 0 on free routes, 304s and client errors.
	CreditsCost        *int
	QuotaLimit         *int
	QuotaRemaining     *int
	QuotaReset         string
	RateLimitRemaining *int
	PlanWithheld       []string
	// TrialExpiresAt is set on trial keys only.
	TrialExpiresAt string
}

func headerInt(h http.Header, name string) *int {
	raw := h.Get(name)
	if raw == "" {
		return nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return nil
	}
	return &n
}

func toResponseInfo(rawURL string, resp *http.Response) *ResponseInfo {
	h := resp.Header
	info := &ResponseInfo{
		URL:                rawURL,
		Status:             resp.StatusCode,
		RequestID:          h.Get("X-Request-Id"),
		ErrorCode:          h.Get("X-Error-Code"),
		CreditsCost:        headerInt(h, "X-Credits-Cost"),
		QuotaLimit:         headerInt(h, "X-Quota-Limit"),
		QuotaRemaining:     headerInt(h, "X-Quota-Remaining"),
		QuotaReset:         h.Get("X-Quota-Reset"),
		RateLimitRemaining: headerInt(h, "RateLimit-Remaining"),
		PlanWithheld:       []string{},
		TrialExpiresAt:     h.Get("X-Trial-Expires-At"),
	}
	if withheld := h.Get("X-Plan-Withheld"); withheld != "" {
		for _, v := range strings.Split(withheld, ",") {
			info.PlanWithheld = append(info.PlanWithheld, strings.TrimSpace(v))
		}
	}
	return info
}

const (
	defaultBaseURL    = "https://api.pokemontcgapi.com"
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 2
	maxRetryAfter     = 60 * time.Second
)

// sleep waits for d or until ctx is done. A variable so tests can replace it.
var sleep = func(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// isRetryable: network failures, 5xx and 429 are the only set worth retrying.
func isRetryable(err error) bool {
	var quota *QuotaExceededError
	if errors.As(err, &quota) {
		return false
	}
	var limited *RateLimitedError
	var server *ServerError
	var conn *ConnectionError
	if errors.As(err, &limited) || errors.As(err, &server) || errors.As(err, &conn) {
		return true
	}
	var api *APIError
	return errors.As(err, &api) && api.Status == http.StatusRequestTimeout
}

// backoff is exponential with full jitter. The jitter is not a detail:
// without it, a thousand clients hit by the same 429 retry in the same
// millisecond and rebuild the queue they were trying to drain.
func backoff(attempt int, retryAfter *time.Duration) time.Duration {
	if retryAfter != nil {
		return min(*retryAfter, maxRetryAfter)
	}
	ceiling := min(500*time.Millisecond<<attempt, 8*time.Second)
	return time.Duration(rand.Float64() * float64(ceiling))
}

type etagEntry struct {
	etag string
	body []byte
}

func (c *Client) url(path string, q url.Values) string {
	encoded := q.Encode()
	if encoded == "" {
		return c.baseURL + path
	}
	return c.baseURL + path + "?" + encoded
}

// get performs a GET with retries and decodes the body into out.
func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	return c.request(ctx, c.url(path, q), out)
}

// follow requests a URL the API already built (links.next). Public through
// Page.Next: the cursor carries the signature of the sort order, and
// rebuilding the URL by hand is exactly what the API rejects with INVALID_CURSOR.
func (c *Client) follow(ctx context.Context, absoluteURL string, out any) error {
	return c.request(ctx, absoluteURL, out)
}

// post sends a body once. It never goes through the ETag cache and is never
// retried: an automatic retry is safe only on idempotent requests, and a POST
// that may have reached the server is not one.
func (c *Client) post(ctx context.Context, path string, body []byte, contentType string, out any) error {
	return c.attempt(ctx, http.MethodPost, c.url(path, nil), body, contentType, out)
}

func (c *Client) request(ctx context.Context, rawURL string, out any) error {
	for attempt := 0; ; attempt++ {
		err := c.attempt(ctx, http.MethodGet, rawURL, nil, "", out)
		if err == nil {
			return nil
		}
		if attempt >= c.maxRetries || !isRetryable(err) || ctx.Err() != nil {
			return err
		}
		var retryAfter *time.Duration
		var limited *RateLimitedError
		if errors.As(err, &limited) && limited.HasRetryAfter {
			retryAfter = &limited.RetryAfter
		}
		if err := sleep(ctx, backoff(attempt, retryAfter)); err != nil {
			return err
		}
	}
}

func (c *Client) attempt(ctx context.Context, method, rawURL string, body []byte, contentType string, out any) error {
	attemptCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(attemptCtx, method, rawURL, reader)
	if err != nil {
		return &ConnectionError{URL: rawURL, Err: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// No conditional cache on writes: the ETag is the fingerprint of a body,
	// and on a response that depends on what you just uploaded it means nothing.
	var cached *etagEntry
	if method == http.MethodGet && c.etags != nil {
		c.etagMu.Lock()
		if entry, ok := c.etags[rawURL]; ok {
			cached = &entry
		}
		c.etagMu.Unlock()
		if cached != nil {
			req.Header.Set("If-None-Match", cached.etag)
		}
	}

	resp, err := c.httpc.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			// The caller cancelled: hand their error back untouched.
			return ctx.Err()
		}
		if errors.Is(attemptCtx.Err(), context.DeadlineExceeded) {
			return &TimeoutError{ConnectionError: &ConnectionError{URL: rawURL, Err: err}, Timeout: c.timeout}
		}
		return &ConnectionError{URL: rawURL, Err: err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ConnectionError{URL: rawURL, Err: err}
	}

	info := toResponseInfo(rawURL, resp)
	c.last.Store(info)
	if c.onResponse != nil {
		c.onResponse(*info)
	}

	// 304: the body is empty by definition, the answer is the cached one.
	if resp.StatusCode == http.StatusNotModified && cached != nil {
		return decode(cached.body, out)
	}

	if resp.StatusCode >= 400 || resp.StatusCode == http.StatusNotModified {
		var retryAfter *time.Duration
		if header := resp.Header.Get("Retry-After"); header != "" {
			if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil {
				d := time.Duration(seconds) * time.Second
				retryAfter = &d
			}
		}
		return toAPIError(resp.StatusCode, readErrorBody(resp.StatusCode, raw), retryAfter)
	}

	if err := decode(raw, out); err != nil {
		return err
	}
	if method == http.MethodGet && c.etags != nil {
		if etag := resp.Header.Get("ETag"); etag != "" {
			c.etagMu.Lock()
			c.etags[rawURL] = etagEntry{etag: etag, body: raw}
			c.etagMu.Unlock()
		}
	}
	return nil
}

func decode(raw []byte, out any) error {
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// readErrorBody accepts the API envelope, and builds an error from the status
// alone when the body is something else (an HTML page from a proxy).
func readErrorBody(status int, raw []byte) errorBody {
	var parsed struct {
		Error *errorBody `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil && parsed.Error != nil && parsed.Error.Code != "" {
		return *parsed.Error
	}
	return fallbackErrorBody(status)
}
