package pokemontcgapi

import "context"

// SeriesService wraps /v1/series.
type SeriesService struct{ client *Client }

// List returns the series (Scarlet & Violet, Sword & Shield, ...) with their set counts.
// Only OrderBy, Limit and Cursor apply on this route.
func (s *SeriesService) List(ctx context.Context, p *ListParams) (*Page[Series], error) {
	q := p.values()
	q.Del("q")
	q.Del("select")
	var body Collection[Series]
	if err := s.client.get(ctx, "/v1/series", q, &body); err != nil {
		return nil, err
	}
	return newPage(s.client, body), nil
}
