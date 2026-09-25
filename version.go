// Package pokemontcgapi is a thin client for the Pokémon TCG API at
// https://pokemontcgapi.com: cards, sets, artists, series, sealed products,
// prices with stated provenance, the change feed, the reference vocabularies
// and photo recognition.
//
// It has no dependencies outside the standard library. Every method takes a
// context.Context; list methods return a *Page that follows the API's own
// links.next URL, so pagination never has to be rebuilt by hand.
package pokemontcgapi

// Version is the SDK version. It travels in the User-Agent header of every
// request and must match the git tag of a release.
const Version = "0.1.0"

const defaultUserAgent = "pokemontcgapi-go/" + Version
