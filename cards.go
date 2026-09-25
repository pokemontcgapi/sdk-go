package pokemontcgapi

import (
	"context"
	"fmt"
	"strings"
)

// CardsService wraps /v1/cards.
type CardsService struct{ client *Client }

// Search runs a catalogue search and returns the first page, iterable to the end.
func (s *CardsService) Search(ctx context.Context, p *CardListParams) (*Page[Card], error) {
	var body Collection[Card]
	if err := s.client.get(ctx, "/v1/cards", p.values(), &body); err != nil {
		return nil, err
	}
	return newPage(s.client, body), nil
}

// Get reads one card by id. Both the canonical id and the alternate legacy id resolve.
func (s *CardsService) Get(ctx context.Context, id string, p *CardGetParams) (*Card, error) {
	var out Card
	if err := s.client.get(ctx, pathf("/v1/cards/%s", id), p.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Batch reads up to 100 cards in one request. The answer carries Requested and
// Found, and Missing when something did not resolve: one entry per id, with
// SuggestedID where the id is a historical alias of a card now in another set.
func (s *CardsService) Batch(ctx context.Context, ids []string, p *CardGetParams) (*BatchResult[Card], error) {
	if len(ids) == 0 {
		return &BatchResult[Card]{Data: []Card{}}, nil
	}
	if len(ids) > 100 {
		return nil, fmt.Errorf("%w: Cards.Batch accepts at most 100 ids, received %d; chunk the list", ErrTooManyIDs, len(ids))
	}
	q := p.values()
	q.Set("ids", strings.Join(ids, ","))
	var out BatchResult[Card]
	if err := s.client.get(ctx, "/v1/cards/batch", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
