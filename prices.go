package pokemontcgapi

import (
	"context"
	"fmt"
	"strings"
)

// PricesService wraps the dedicated price routes.
//
// Every row says where it comes from (Source), what it rests on (Basis: sold,
// asking, guide, derived) and which day it is for (AsOf). The rows the plan
// does not cover are missing from the body: LastResponse().PlanWithheld says which.
type PricesService struct{ client *Client }

// Card returns the index and current quotes of one card. 2 credits.
func (s *PricesService) Card(ctx context.Context, id string, p *PriceFilterParams) (*PricesResponse[CardPrices], error) {
	var out PricesResponse[CardPrices]
	if err := s.client.get(ctx, pathf("/v1/cards/%s/prices", id), p.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Current returns up to 50 cards in one call, 4 credits per 25.
func (s *PricesService) Current(ctx context.Context, ids []string, p *PriceFilterParams) (*BatchResult[CardPrices], error) {
	if len(ids) == 0 {
		return &BatchResult[CardPrices]{Data: []CardPrices{}}, nil
	}
	if len(ids) > 50 {
		return nil, fmt.Errorf("%w: Prices.Current accepts at most 50 ids, received %d; chunk the list", ErrTooManyIDs, len(ids))
	}
	q := p.values()
	q.Set("ids", strings.Join(ids, ","))
	var out BatchResult[CardPrices]
	if err := s.client.get(ctx, "/v1/prices/current", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// History returns the daily history. 5 credits. The window depends on the plan
// (7 days on the trial, 30 on Developer, everything from Growth): asking for a
// wider one gives *UpgradeRequiredError with PermittedWindow.
func (s *PricesService) History(ctx context.Context, id string, p *HistoryParams) (*HistoryResponse, error) {
	var out HistoryResponse
	if err := s.client.get(ctx, pathf("/v1/cards/%s/prices/history", id), p.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Stats returns low, high, median and change of the index over a window. 2 credits.
func (s *PricesService) Stats(ctx context.Context, id string, p *StatsParams) (*StatsResponse, error) {
	var out StatsResponse
	if err := s.client.get(ctx, pathf("/v1/cards/%s/prices/stats", id), p.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Movers returns the cards that moved the most. 3 credits, from the Growth plan
// (*PlanRequiredError below it).
func (s *PricesService) Movers(ctx context.Context, p *MoversParams) (*MoversResponse, error) {
	var out MoversResponse
	if err := s.client.get(ctx, "/v1/prices/movers", p.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Sources lists the price sources with the declared delay of each. Free.
func (s *PricesService) Sources(ctx context.Context) ([]PriceSourceInfo, error) {
	var body struct {
		Data []PriceSourceInfo `json:"data"`
	}
	if err := s.client.get(ctx, "/v1/prices/sources", nil, &body); err != nil {
		return nil, err
	}
	return body.Data, nil
}
