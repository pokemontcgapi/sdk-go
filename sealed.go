package pokemontcgapi

import "context"

// SealedService wraps /v1/sealed: booster boxes, ETBs, tins, blisters, collections.
type SealedService struct{ client *Client }

// List returns sealed products.
func (s *SealedService) List(ctx context.Context, p *SealedListParams) (*Page[SealedProduct], error) {
	var body Collection[SealedProduct]
	if err := s.client.get(ctx, "/v1/sealed", p.values(), &body); err != nil {
		return nil, err
	}
	return newPage(s.client, body), nil
}

// Get reads one sealed product.
func (s *SealedService) Get(ctx context.Context, id string, p *SetGetParams) (*SealedProduct, error) {
	var out SealedProduct
	if err := s.client.get(ctx, pathf("/v1/sealed/%s", id), p.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Prices returns the current quotes of a product. 2 credits. The composite
// index does not cover sealed products: Data.Index is nil.
func (s *SealedService) Prices(ctx context.Context, id string, p *PriceFilterParams) (*PricesResponse[SealedPrices], error) {
	var out PricesResponse[SealedPrices]
	if err := s.client.get(ctx, pathf("/v1/sealed/%s/prices", id), p.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
