package pokemontcgapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ErrTooManyIDs is returned, wrapped with the count, when a batch method
// receives more ids than the API accepts in one request.
var ErrTooManyIDs = errors.New("pokemontcgapi: too many ids")

// APIError is the error envelope the API returns on every 4xx and 5xx. The
// typed wrappers in this file (NotFoundError, RateLimitedError, ...) embed it,
// so errors.As works both on the wrapper and on *APIError itself.
//
// QuotaExceededError is separate from RateLimitedError on purpose: both are
// 429, but a rate limit passes by waiting and an exhausted quota never does.
// The client retries the first and never the second.
type APIError struct {
	// Code is the stable code of the error taxonomy, e.g. CARD_NOT_FOUND.
	Code    string
	Status  int
	Message string
	// RequestID is set whenever the response came from the API. Quote it in a
	// support message: it is the only thing that can be looked up in the logs.
	RequestID string
	Details   map[string]any
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

// NextStep is the human handoff attached to commercial refusals
// (details.next_step): what the account owner has to do, and where.
type NextStep struct {
	Action            string // subscribe, upgrade, contact_sales, contact_support, verify_email
	Actor             string // always account_owner
	Plan              *NextStepPlan
	CheckoutURL       string
	CheckoutURLYearly string
	ManageURL         string
	ContactURL        string
	VerifyURL         string
	PlansURL          string
	Handoff           string
}

// NextStepPlan describes the plan a next step points at, when the API names one.
type NextStepPlan struct {
	Code            string
	Name            string
	MonthlyEUR      string
	YearlyEUR       string
	CreditsPerMonth *int
}

var nextStepActions = map[string]bool{
	"subscribe": true, "upgrade": true, "contact_sales": true, "contact_support": true, "verify_email": true,
}

func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// NextStep returns details.next_step when it is well formed, nil otherwise.
// Malformed blocks are ignored rather than half-read.
func (e *APIError) NextStep() *NextStep {
	raw, ok := e.Details["next_step"].(map[string]any)
	if !ok {
		return nil
	}
	action, _ := raw["action"].(string)
	handoff, okHandoff := raw["handoff"].(string)
	plansURL, okPlans := raw["plans_url"].(string)
	if raw["actor"] != "account_owner" || !okHandoff || !okPlans || !nextStepActions[action] {
		return nil
	}
	step := &NextStep{
		Action:            action,
		Actor:             "account_owner",
		CheckoutURL:       stringField(raw, "checkout_url"),
		CheckoutURLYearly: stringField(raw, "checkout_url_yearly"),
		ManageURL:         stringField(raw, "manage_url"),
		ContactURL:        stringField(raw, "contact_url"),
		VerifyURL:         stringField(raw, "verify_url"),
		PlansURL:          plansURL,
		Handoff:           handoff,
	}
	if plan, ok := raw["plan"].(map[string]any); ok {
		step.Plan = &NextStepPlan{
			Code:       stringField(plan, "code"),
			Name:       stringField(plan, "name"),
			MonthlyEUR: stringField(plan, "monthly_eur"),
			YearlyEUR:  stringField(plan, "yearly_eur"),
		}
		if n, ok := plan["credits_per_month"].(float64); ok {
			credits := int(n)
			step.Plan.CreditsPerMonth = &credits
		}
	}
	return step
}

// CheckoutURL is the subscribe checkout, when the next step carries one.
func (e *APIError) CheckoutURL() string {
	if step := e.NextStep(); step != nil {
		return step.CheckoutURL
	}
	return ""
}

// ActionURL is the URL for the next step's action: checkout for subscribe,
// the account page for upgrade, the verification link for verify_email, the
// contact address for sales and support. Empty when there is none.
func (e *APIError) ActionURL() string {
	step := e.NextStep()
	if step == nil {
		return ""
	}
	switch step.Action {
	case "subscribe":
		return step.CheckoutURL
	case "upgrade":
		return step.ManageURL
	case "verify_email":
		return step.VerifyURL
	default:
		return step.ContactURL
	}
}

// Handoff is the sentence to show the account owner verbatim.
func (e *APIError) Handoff() string {
	if step := e.NextStep(); step != nil {
		return step.Handoff
	}
	return ""
}

// AuthenticationError is a 401: the key is missing, malformed or revoked.
type AuthenticationError struct{ *APIError }

func (e *AuthenticationError) Unwrap() error { return e.APIError }

// PermissionDeniedError is a 403: the key is valid but may not do this.
type PermissionDeniedError struct{ *APIError }

func (e *PermissionDeniedError) Unwrap() error { return e.APIError }

// PlanRequiredError is 403 PLAN_REQUIRED: the route is not in the plan
// (movers and photo recognition start at Growth). It unwraps to
// *PermissionDeniedError, so code that catches that keeps working.
type PlanRequiredError struct{ *PermissionDeniedError }

func (e *PlanRequiredError) Unwrap() error { return e.PermissionDeniedError }

// TrialExpiredError is 403 TRIAL_EXPIRED: the trial is over and the route
// costs credits. Waiting and retrying do not help; a plan does.
type TrialExpiredError struct{ *PermissionDeniedError }

func (e *TrialExpiredError) Unwrap() error { return e.PermissionDeniedError }

// UpgradeRequiredError is 403 UPGRADE_REQUIRED: the window asked for is wider
// than the plan allows.
type UpgradeRequiredError struct{ *APIError }

func (e *UpgradeRequiredError) Unwrap() error { return e.APIError }

// PermittedWindow is the window the current plan grants, when the API says.
func (e *UpgradeRequiredError) PermittedWindow() any { return e.Details["permitted_window"] }

// NotFoundError is a 404, or any code ending in _NOT_FOUND. Not a network
// error: it is never retried.
type NotFoundError struct{ *APIError }

func (e *NotFoundError) Unwrap() error { return e.APIError }

// InvalidRequestError is a 400 or 422: the request is wrong. Field names the
// parameter when the API says which one.
type InvalidRequestError struct{ *APIError }

func (e *InvalidRequestError) Unwrap() error { return e.APIError }

// Field is details.field, when present.
func (e *InvalidRequestError) Field() string { return stringField(e.Details, "field") }

// RateLimitedError is a 429 with Retry-After: it passes by waiting.
type RateLimitedError struct {
	*APIError
	// RetryAfter is the wait the API asked for, valid when HasRetryAfter is true.
	RetryAfter    time.Duration
	HasRetryAfter bool
}

func (e *RateLimitedError) Unwrap() error { return e.APIError }

// QuotaExceededError is a 429 for an exhausted period quota: it does NOT pass
// by waiting, and the client never retries it.
type QuotaExceededError struct{ *APIError }

func (e *QuotaExceededError) Unwrap() error { return e.APIError }

// ServerError is any 5xx.
type ServerError struct{ *APIError }

func (e *ServerError) Unwrap() error { return e.APIError }

// ConnectionError means the request never got an answer: DNS, TLS, socket.
type ConnectionError struct {
	URL string
	Err error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("pokemontcgapi: request to %s failed: %v", e.URL, e.Err)
}

func (e *ConnectionError) Unwrap() error { return e.Err }

// TimeoutError means the request was abandoned by the client after Timeout.
// It unwraps to *ConnectionError.
type TimeoutError struct {
	*ConnectionError
	Timeout time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("pokemontcgapi: request to %s timed out after %s", e.URL, e.Timeout)
}

func (e *TimeoutError) Unwrap() error { return e.ConnectionError }

type errorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details"`
	RequestID string         `json:"request_id"`
}

// fallbackErrorBody is used when the body is not the API envelope (an HTML
// 502 from a proxy, say): the status still becomes an error instead of a
// decoding failure that hides it.
func fallbackErrorBody(status int) errorBody {
	message := http.StatusText(status)
	if message == "" {
		message = fmt.Sprintf("HTTP %d", status)
	}
	return errorBody{Code: fmt.Sprintf("HTTP_%d", status), Message: message}
}

// toAPIError picks the class from the body and the status: the code first,
// then the status. The status says the family, the code says the case, and
// the two cases that matter most (rate limit versus quota) share a status.
func toAPIError(status int, body errorBody, retryAfter *time.Duration) error {
	base := &APIError{Code: body.Code, Status: status, Message: body.Message, RequestID: body.RequestID, Details: body.Details}
	denied := func() *PermissionDeniedError { return &PermissionDeniedError{base} }
	switch {
	case body.Code == "QUOTA_EXCEEDED":
		return &QuotaExceededError{base}
	case body.Code == "UPGRADE_REQUIRED":
		return &UpgradeRequiredError{base}
	case body.Code == "PLAN_REQUIRED":
		return &PlanRequiredError{denied()}
	case body.Code == "TRIAL_EXPIRED":
		return &TrialExpiredError{denied()}
	case status == http.StatusTooManyRequests:
		e := &RateLimitedError{APIError: base}
		if retryAfter != nil {
			e.RetryAfter, e.HasRetryAfter = *retryAfter, true
		}
		return e
	case status == http.StatusUnauthorized:
		return &AuthenticationError{base}
	case status == http.StatusForbidden:
		return denied()
	case status == http.StatusNotFound || strings.HasSuffix(body.Code, "_NOT_FOUND"):
		return &NotFoundError{base}
	case status >= 500:
		return &ServerError{base}
	case status >= 400:
		return &InvalidRequestError{base}
	}
	return base
}
