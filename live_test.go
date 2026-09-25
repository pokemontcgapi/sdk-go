//go:build live

package pokemontcgapi_test

// Live smoke test against the real API. Run with:
//
//	PTCG_API_KEY=... go test -tags live -run TestLive -v ./...
//
// It spends about four credits: one card search, up to three set pages, and
// the free routes. Everything else asserts on error classes, not on data.

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pokemontcgapi/sdk-go"
)

func TestLive(t *testing.T) {
	if os.Getenv("PTCG_API_KEY") == "" {
		t.Skip("PTCG_API_KEY not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := pokemontcgapi.New(pokemontcgapi.WithETagCache())

	status, err := client.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Catalog.Cards == 0 {
		t.Error("status reports no cards")
	}
	if cost := client.LastResponse().CreditsCost; cost != nil && *cost != 0 {
		t.Errorf("status must be free, cost %d", *cost)
	}

	page, err := client.Cards.Search(ctx, &pokemontcgapi.CardListParams{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	info := client.LastResponse()
	if len(page.Data) != 1 || info.RequestID == "" || info.CreditsCost == nil || *info.CreditsCost != 1 {
		t.Errorf("search: %d rows, info %+v", len(page.Data), info)
	}

	_, err = client.Cards.Get(ctx, "bs-4x", nil)
	var notFound *pokemontcgapi.NotFoundError
	if !errors.As(err, &notFound) {
		t.Errorf("near-miss id: want NotFoundError, got %v", err)
	} else if s, _ := notFound.Details["did_you_mean"].(string); s != "" {
		t.Logf("did_you_mean: %s", s)
	}

	sets, err := client.Sets.List(ctx, &pokemontcgapi.SetListParams{Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2 && sets.HasMore(); i++ {
		next, err := sets.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(client.LastResponse().URL, "https://api.pokemontcgapi.com/v1/sets?") {
			t.Errorf("page %d did not follow links.next: %s", i+2, client.LastResponse().URL)
		}
		sets = next
	}

	if _, err := client.Cards.Search(ctx, &pokemontcgapi.CardListParams{Limit: 1}); err != nil {
		t.Fatal(err)
	}
	if again := client.LastResponse(); again.Status != 304 {
		t.Errorf("second identical search should be a 304, got %d", again.Status)
	}

	_, err = client.Prices.Movers(ctx, &pokemontcgapi.MoversParams{Window: "7d"})
	var plan *pokemontcgapi.PlanRequiredError
	if err != nil && !errors.As(err, &plan) {
		t.Errorf("movers: want success or PlanRequiredError, got %v", err)
	}

	burst := pokemontcgapi.New(pokemontcgapi.WithMaxRetries(0))
	for i := 0; i < 40; i++ {
		_, err := burst.Reference.All(ctx)
		burst = pokemontcgapi.New(pokemontcgapi.WithMaxRetries(0))
		var limited *pokemontcgapi.RateLimitedError
		if errors.As(err, &limited) {
			if !limited.HasRetryAfter {
				t.Error("429 without Retry-After")
			}
			return
		}
	}
	t.Log("no 429 in a burst of 40 free calls; rate limits differ per plan")
}
