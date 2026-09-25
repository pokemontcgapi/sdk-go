package pokemontcgapi

import "context"

// SetsService wraps /v1/sets.
type SetsService struct{ client *Client }

// List returns the sets. Region is the filter worth knowing: JP returns the
// Japanese releases, which are the largest part of the catalogue and not
// translations of the Western ones.
func (s *SetsService) List(ctx context.Context, p *SetListParams) (*Page[CardSet], error) {
	var body Collection[CardSet]
	if err := s.client.get(ctx, "/v1/sets", p.values(), &body); err != nil {
		return nil, err
	}
	return newPage(s.client, body), nil
}

// Get reads one set by code, slug or alternate id.
func (s *SetsService) Get(ctx context.Context, code string, p *SetGetParams) (*CardSet, error) {
	var out CardSet
	if err := s.client.get(ctx, pathf("/v1/sets/%s", code), p.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Cards returns the cards of one set, in collection order. The Set field of
// the params is ignored: the set is the one in the path.
func (s *SetsService) Cards(ctx context.Context, code string, p *CardListParams) (*Page[Card], error) {
	q := p.values()
	q.Del("set")
	var body Collection[Card]
	if err := s.client.get(ctx, pathf("/v1/sets/%s/cards", code), q, &body); err != nil {
		return nil, err
	}
	return newPage(s.client, body), nil
}
