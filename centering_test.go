package pokemontcgapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime"
	"mime/multipart"
	"os"
	"reflect"
	"strings"
	"testing"
)

func confirmFixture(t *testing.T) CenteringConfirmInput {
	t.Helper()
	raw, err := os.ReadFile("testdata/fixtures/centering-confirm-input.json")
	if err != nil {
		t.Fatal(err)
	}
	var input CenteringConfirmInput
	if err = json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	return input
}

func TestCenteringMultipart(t *testing.T) {
	srv := fixtureServer(t, "centering-measure")
	c := newClient(srv)
	corners := confirmFixture(t).Geometry.Corners
	result, err := c.Centering.Measure(context.Background(), strings.NewReader("synthetic"), CenteringMeasureOptions{Face: "front", CardSize: "standard", Corners: &corners})
	if err != nil {
		t.Fatal(err)
	}
	if result.Data.Status != "needs-review" || result.Data.Ceilings != nil || result.Data.Geometry == nil || result.Data.Geometry.GeometryVersion != 1 {
		t.Fatalf("response: %+v", result)
	}
	req := srv.requests[0]
	if req.Method != "POST" || req.URL != "/v1/centering/measure" {
		t.Fatalf("request: %+v", req)
	}
	media, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || media != "multipart/form-data" || params["boundary"] == "" {
		t.Fatal("multipart boundary missing")
	}
	form, err := multipart.NewReader(bytes.NewReader(req.Body), params["boundary"]).ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer form.RemoveAll()
	if form.File["image"][0].Filename != "card" || form.File["image"][0].Size != 9 || form.Value["face"][0] != "front" || form.Value["card_size"][0] != "standard" {
		t.Fatalf("form: %+v", form)
	}
	var sent CenteringCorners
	if err = json.Unmarshal([]byte(form.Value["corners"][0]), &sent); err != nil || sent != corners {
		t.Fatal("corners changed")
	}
	if _, err = c.Centering.Measure(context.Background(), strings.NewReader("x"), CenteringMeasureOptions{Face: "front"}); err != nil {
		t.Fatal(err)
	}
	_, params, _ = mime.ParseMediaType(srv.requests[1].Header.Get("Content-Type"))
	form, err = multipart.NewReader(bytes.NewReader(srv.requests[1].Body), params["boundary"]).ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer form.RemoveAll()
	if len(form.Value) != 1 {
		t.Fatalf("unexpected optional fields: %v", form.Value)
	}
}

func TestCenteringConfirm(t *testing.T) {
	srv := fixtureServer(t, "centering-confirm")
	c := newClient(srv)
	input := confirmFixture(t)
	result, err := c.Centering.Confirm(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	req := srv.requests[0]
	var sent CenteringConfirmInput
	if err = json.Unmarshal(req.Body, &sent); err != nil || !reflect.DeepEqual(sent, input) {
		t.Fatal("geometry changed")
	}
	if req.Method != "POST" || req.URL != "/v1/centering/confirm" || req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("request: %+v", req)
	}
	if result.Data.Basis != "client-confirmed-lines" || result.Data.Ceilings["PSA"].MaximumGrade == nil || !strings.Contains(result.Data.Disclaimer, "ceiling from centering alone, not a prediction of the final grade") {
		t.Fatalf("response: %+v", result)
	}
	// All response fields round-trip without losing geometry, nullable or fractional fields.
	want := loadFixture(t, "centering-confirm").Body
	got, _ := json.Marshal(result)
	var w, g any
	_ = json.Unmarshal(want, &w)
	_ = json.Unmarshal(got, &g)
	if !reflect.DeepEqual(w, g) {
		t.Fatal("response field lost or changed")
	}
}

func TestCenteringGrade(t *testing.T) {
	srv := fixtureServer(t, "centering-grade")
	c := newClient(srv)
	result, err := c.Centering.Grade(context.Background(), CenteringGradeInput{FrontLR: 55, FrontTB: 52, House: "PSA"})
	if err != nil {
		t.Fatal(err)
	}
	req := srv.requests[0]
	var sent map[string]any
	_ = json.Unmarshal(req.Body, &sent)
	if req.URL != "/v1/centering/grade" || req.Method != "POST" || req.Header.Get("Content-Type") != "application/json" || sent["front_lr"] != float64(55) || sent["house"] != "PSA" {
		t.Fatalf("request: %+v", req)
	}
	if !result.Data.Incomplete || result.Data.Back != nil || result.Data.Combined["PSA"].MaximumGrade == nil {
		t.Fatalf("response: %+v", result)
	}
}

func TestCenteringStandards(t *testing.T) {
	srv := fixtureServer(t, "centering-standards")
	c := newClient(srv)
	result, err := c.Centering.Standards(context.Background(), &CenteringStandardsOptions{House: "PSA", Face: "front"})
	if err != nil {
		t.Fatal(err)
	}
	req := srv.requests[0]
	if req.Method != "GET" || req.URL != "/v1/centering/standards?face=front&house=PSA" {
		t.Fatalf("request: %+v", req)
	}
	if len(result.Data.Standards) != 1 || result.Data.Standards[0].Bands[0].Value != 10 {
		t.Fatalf("response: %+v", result)
	}
}

func TestCenteringPostNoRetry(t *testing.T) {
	srv := fixtureServer(t, "500-html")
	c := newClient(srv)
	c.maxRetries = 3
	_, a := c.Centering.Measure(context.Background(), strings.NewReader("x"), CenteringMeasureOptions{Face: "front"})
	_, b := c.Centering.Confirm(context.Background(), confirmFixture(t))
	_, d := c.Centering.Grade(context.Background(), CenteringGradeInput{FrontLR: 55})
	for _, err := range []error{a, b, d} {
		var target *ServerError
		if !errors.As(err, &target) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if len(srv.requests) != 3 {
		t.Fatalf("retried POST: %d", len(srv.requests))
	}
}
