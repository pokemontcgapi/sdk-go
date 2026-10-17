package pokemontcgapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/url"
)

// CenteringService measures card centering. Growth and higher; verified Free
// trial: 5 measure and 5 confirm calls, independently. Developer is excluded.
type CenteringService struct{ client *Client }

// Measure costs 25 credits. A needs-review result without ceilings is expected.
// Show the geometry and uncertainty to a person before offering confirmation.
func (s *CenteringService) Measure(ctx context.Context, image io.Reader, opts CenteringMeasureOptions) (*CenteringMeasureResponse, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("image", "card")
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(part, image); err != nil {
		return nil, err
	}
	if err = w.WriteField("face", opts.Face); err != nil {
		return nil, err
	}
	if opts.CardSize != "" {
		if err = w.WriteField("card_size", opts.CardSize); err != nil {
			return nil, err
		}
	}
	if opts.Corners != nil {
		corners, err := json.Marshal(opts.Corners)
		if err != nil {
			return nil, err
		}
		if err = w.WriteField("corners", string(corners)); err != nil {
			return nil, err
		}
	}
	if err = w.Close(); err != nil {
		return nil, err
	}
	var out CenteringMeasureResponse
	if err = s.client.post(ctx, "/v1/centering/measure", buf.Bytes(), w.FormDataContentType(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Confirm costs 5 credits. Only call after a person checked the lines in your app.
func (s *CenteringService) Confirm(ctx context.Context, input CenteringConfirmInput) (*CenteringConfirmResponse, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var out CenteringConfirmResponse
	if err = s.client.post(ctx, "/v1/centering/confirm", body, "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Grade costs 1 credit. Ceiling from centering alone, not a prediction of the final grade.
func (s *CenteringService) Grade(ctx context.Context, input CenteringGradeInput) (*CenteringGradeResponse, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var out CenteringGradeResponse
	if err = s.client.post(ctx, "/v1/centering/grade", body, "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Standards costs 1 credit and returns thresholds with source status and provenance.
func (s *CenteringService) Standards(ctx context.Context, opts *CenteringStandardsOptions) (*CenteringStandardsResponse, error) {
	q := url.Values{}
	if opts != nil {
		if opts.House != "" {
			q.Set("house", opts.House)
		}
		if opts.Face != "" {
			q.Set("face", opts.Face)
		}
	}
	var out CenteringStandardsResponse
	if err := s.client.get(ctx, "/v1/centering/standards", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
