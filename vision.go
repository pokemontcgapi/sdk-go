package pokemontcgapi

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"strconv"
	"strings"
)

// VisionService recognises a card from a photograph.
//
// It costs 25 base credits a call; index adds 1 and prices 4 for top_k <= 10. It is the only route that
// does not return a row but the outcome of a comparison against the whole
// image index. Worth knowing before putting it in a loop.
type VisionService struct{ client *Client }

// Identify sends a photo and returns the ranked candidates.
//
// Read Data.Decision before Data.ID. ID is set only on a match; on ambiguous
// it is nil on purpose, because two printings of the same illustration cannot
// be told apart from the image alone and picking one means being wrong half
// the time, on exactly the cards that are worth the most. If your flow knows
// the set (whoever inventories a just-opened pack does), pass it in
// IdentifyOptions.Set: that is what resolves the tie.
func (s *VisionService) Identify(ctx context.Context, image io.Reader, opts *IdentifyOptions) (*VisionResponse, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("image", "card")
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, image); err != nil {
		return nil, err
	}
	if opts != nil {
		if opts.TopK != 0 {
			if err := w.WriteField("top_k", strconv.Itoa(opts.TopK)); err != nil {
				return nil, err
			}
		}
		if opts.Set != "" {
			if err := w.WriteField("set", opts.Set); err != nil {
				return nil, err
			}
		}
		if opts.Region != "" {
			if err := w.WriteField("region", opts.Region); err != nil {
				return nil, err
			}
		}
		if len(opts.Include) > 0 {
			if err := w.WriteField("include", strings.Join(opts.Include, ",")); err != nil {
				return nil, err
			}
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	var out VisionResponse
	if err := s.client.post(ctx, "/v1/vision/identify", buf.Bytes(), w.FormDataContentType(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
