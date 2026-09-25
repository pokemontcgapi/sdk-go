package pokemontcgapi

import (
	"context"
	"sync"
)

// ReferenceService reads the vocabularies, to populate a UI's filters without
// guessing strings. One network request per client no matter how many of the
// methods are called: the answer is the same and is kept for the life of the
// client. A failed request is not kept, so the next call tries again.
type ReferenceService struct {
	client *Client
	mu     sync.Mutex
	data   map[string][]string
}

// All returns every vocabulary at once: besides the four below, locales,
// print_regions, conditions, printings, grading_companies, price_variants,
// price_bases, change_kinds and the others /v1/reference lists.
func (s *ReferenceService) All(ctx context.Context) (map[string][]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data != nil {
		return s.data, nil
	}
	var body struct {
		Data map[string][]string `json:"data"`
	}
	if err := s.client.get(ctx, "/v1/reference", nil, &body); err != nil {
		return nil, err
	}
	s.data = body.Data
	return s.data, nil
}

func (s *ReferenceService) list(ctx context.Context, key string) ([]string, error) {
	data, err := s.All(ctx)
	if err != nil {
		return nil, err
	}
	values := data[key]
	if values == nil {
		return []string{}, nil
	}
	return values, nil
}

// Types returns the energy types.
func (s *ReferenceService) Types(ctx context.Context) ([]string, error) { return s.list(ctx, "types") }

// Subtypes returns the card subtypes.
func (s *ReferenceService) Subtypes(ctx context.Context) ([]string, error) {
	return s.list(ctx, "subtypes")
}

// Supertypes returns the card supertypes.
func (s *ReferenceService) Supertypes(ctx context.Context) ([]string, error) {
	return s.list(ctx, "supertypes")
}

// Rarities returns the rarities.
func (s *ReferenceService) Rarities(ctx context.Context) ([]string, error) {
	return s.list(ctx, "rarities")
}
