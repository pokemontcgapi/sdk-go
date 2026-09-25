package pokemontcgapi

import "context"

// ArtistsService wraps /v1/artists.
type ArtistsService struct{ client *Client }

// List returns the illustrators. Select is ignored on this route.
func (s *ArtistsService) List(ctx context.Context, p *ListParams) (*Page[Artist], error) {
	q := p.values()
	q.Del("select")
	var body Collection[Artist]
	if err := s.client.get(ctx, "/v1/artists", q, &body); err != nil {
		return nil, err
	}
	return newPage(s.client, body), nil
}

// Get reads one illustrator by slug.
func (s *ArtistsService) Get(ctx context.Context, slug string) (*Artist, error) {
	var out Artist
	if err := s.client.get(ctx, pathf("/v1/artists/%s", slug), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
