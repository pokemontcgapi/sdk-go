package pokemontcgapi

import (
	"context"
	"iter"
)

// Page is one page of a collection that knows how to fetch the next one.
//
// The cursor in links.next carries a signature of the sort order, so it must
// never be rebuilt by hand: Next requests exactly the URL the API returned,
// which is the failure mode this type exists to avoid.
type Page[T any] struct {
	Data []T
	Meta CollectionMeta

	nextURL string
	client  *Client
}

func newPage[T any](c *Client, body Collection[T]) *Page[T] {
	p := &Page[T]{Data: body.Data, Meta: body.Meta, client: c}
	if body.Links != nil {
		p.nextURL = body.Links.Next
	}
	return p
}

// HasMore reports whether the API returned a links.next URL.
func (p *Page[T]) HasMore() bool { return p.nextURL != "" }

// Next fetches the following page, or returns (nil, nil) when there is none.
func (p *Page[T]) Next(ctx context.Context) (*Page[T], error) {
	if p.nextURL == "" {
		return nil, nil
	}
	var body Collection[T]
	if err := p.client.follow(ctx, p.nextURL, &body); err != nil {
		return nil, err
	}
	return newPage(p.client, body), nil
}

// All walks every item of every page, starting from this one.
//
//	for card, err := range page.All(ctx) {
//	    if err != nil { return err }
//	    ...
//	}
func (p *Page[T]) All(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		page := p
		for page != nil {
			for _, item := range page.Data {
				if !yield(item, nil) {
					return
				}
			}
			next, err := page.Next(ctx)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			page = next
		}
	}
}

// Collect materialises up to max items. max is mandatory because the catalogue
// has tens of thousands of cards, and an unbounded collect is the fastest way
// to fill a process's memory by mistake.
func (p *Page[T]) Collect(ctx context.Context, max int) ([]T, error) {
	out := make([]T, 0, min(max, len(p.Data)))
	for item, err := range p.All(ctx) {
		if err != nil {
			return out, err
		}
		out = append(out, item)
		if len(out) >= max {
			break
		}
	}
	return out, nil
}
