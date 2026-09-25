package pokemontcgapi

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// query builds the query string the API expects: lists are comma-joined
// (ids=a,b), not repeated keys, and zero values are left out entirely.
type query struct{ url.Values }

func newQuery() query { return query{url.Values{}} }

func (q query) str(key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

func (q query) int(key string, value int) {
	if value != 0 {
		q.Set(key, strconv.Itoa(value))
	}
}

func (q query) int64(key string, value int64) {
	if value != 0 {
		q.Set(key, strconv.FormatInt(value, 10))
	}
}

func (q query) float(key string, value float64) {
	if value != 0 {
		q.Set(key, strconv.FormatFloat(value, 'f', -1, 64))
	}
}

func (q query) list(key string, values []string) {
	if len(values) > 0 {
		q.Set(key, strings.Join(values, ","))
	}
}

// pathf fills a path template such as "/v1/cards/%s/prices" with URL-escaped
// segments. The template is kept literal so that the route table can be
// audited by reading the source.
func pathf(template string, segments ...string) string {
	escaped := make([]any, len(segments))
	for i, s := range segments {
		escaped[i] = url.PathEscape(s)
	}
	return fmt.Sprintf(template, escaped...)
}
