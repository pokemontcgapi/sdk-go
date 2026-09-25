# pokemontcgapi Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/pokemontcgapi/sdk-go.svg)](https://pkg.go.dev/github.com/pokemontcgapi/sdk-go) [![license](https://img.shields.io/github/license/pokemontcgapi/sdk-go)](./LICENSE) [![CI](https://github.com/pokemontcgapi/sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/pokemontcgapi/sdk-go/actions/workflows/ci.yml)

Go client for the Pokémon TCG API at [pokemontcgapi.com](https://pokemontcgapi.com): cards,
sets, illustrators, the reference vocabularies and photo recognition, across three print lines,
international, Japanese and Simplified Chinese, with card names in eight locales, images, and prices
that state their source, basis, grade and sample size. The current counts are live at
[/v1/status](https://api.pokemontcgapi.com/v1/status).

**Every data route has a method**: cards, sets, series, artists, sealed products, the dedicated
price routes (current, batch, history, stats, movers, sources), the `/v1/changes` feed, the reference
vocabularies and photo recognition. Account and billing routes (`/v1/me`, keys, checkout) are not
wrapped: they belong to the dashboard.

**Zero dependencies.** Standard library only (`net/http`, `encoding/json`, `mime/multipart`), Go 1.23
or newer. Every method takes a `context.Context`.

Unofficial. Not produced, endorsed, supported by or affiliated with Nintendo, Creatures Inc.,
GAME FREAK inc. or The Pokémon Company International. Pokémon and all related marks are trademarks of
their respective owners.

## Get a key

Generate the Idempotency-Key once per signup and keep it with the request body:

```bash
IDEM=$(uuidgen)
```

```bash
curl -s -X POST "https://api.pokemontcgapi.com/v1/accounts/free" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $IDEM" \
  -d '{"email":"you@example.com"}'
```

Lost the response? Repeat the exact same request (same Idempotency-Key, same body byte for byte, same network: same public IPv4 or the same IPv6 /64) within 24 hours and the response comes back, if stored, secret included; it is the original response, so a key rotated or revoked since then is not revived. A new Idempotency-Key for the same email returns 409 ACCOUNT_EXISTS; the same key with a different body returns 409 IDEMPOTENCY_CONFLICT.

We store only a hash of the key; the signup response is kept for 24 hours so the same request can be replayed. Save `data.key.secret` now.

If replay is unavailable, [sign in](https://pokemontcgapi.com/account) and rotate the key, or use /v1/accounts/recover with an already verified email to get a new secret.

The key comes back in `data.key.secret`. Confirming the address we email raises the trial from
80 to 800 credits, and the trial ends 30 days after signup. Paid plans start at 29 EUR a month:
[pricing](https://pokemontcgapi.com/pricing).

## Install

```bash
go get github.com/pokemontcgapi/sdk-go
```

## Use

```go
import "github.com/pokemontcgapi/sdk-go"

client := pokemontcgapi.New() // reads PTCG_API_KEY; or pokemontcgapi.WithAPIKey("...")

card, err := client.Cards.Get(ctx, "base1-4", &pokemontcgapi.CardGetParams{Include: []string{"prices"}})
if err != nil {
    return err
}
fmt.Println(card.ID, card.Name, *card.IndexEUR)
// bs-4 Charizard 523.76   ← the index on 16 September 2026; it moves, yours will differ
```

`base1-4` and `bs-4` both resolve: the id is the printed coordinate — set code, dash, collector
number — and the alternate legacy id resolves on the same route, so a catalogue you already have does
not start with a matching problem.

### Pagination that you never have to think about

Every list method returns a `*Page`, whose `All(ctx)` iterator follows `links.next` for you. For an
initial import of all cards, use the flat card list so pages fill across set boundaries:

```go
page, err := client.Cards.Search(ctx, &pokemontcgapi.CardListParams{Limit: 250, OrderBy: "id"})
if err != nil {
    return err
}
for card, err := range page.All(ctx) {
    if err != nil {
        return err
    }
    fmt.Println(card.ID, card.Name, card.SetCode)
}
```

For Japanese cards, add `Q: "set.region:JP"`; for Simplified Chinese cards, use
`Q: "set.region:CN"`. Add `Include: []string{"translations"}` when you need localized names;
this keeps the plain catalogue cost. `Include` of `index` and of `prices`
have different credit costs. `Lang` selects a name translation, not a print region.

Use `client.Sets.List(ctx, &pokemontcgapi.SetListParams{Region: "JP", Limit: 250})` to browse set metadata and
`client.Sets.Cards(ctx, "obf", &pokemontcgapi.CardListParams{Limit: 250})` when you need one particular set. For all cards,
the flat list uses fewer requests than a card loop for every set. The
[quickstart](https://pokemontcgapi.com/docs/quickstart#page-the-whole-catalogue) includes dated
measurements, and the [migration guide](https://pokemontcgapi.com/docs/migrate-from-pokemontcg-io)
explains capturing the change feed watermark before an import and keeping the replica current.

The cursor carries a signature of the sort order, so it must never be reconstructed by hand — the SDK
follows the URL the API returned, which is the failure mode this avoids. `Collect(ctx, max)` requires
an explicit ceiling, because the catalogue is large enough that an unbounded materialisation is a
mistake rather than a choice. `Next(ctx)` returns `nil, nil` after the last page.

### One call for a hundred cards

```go
result, err := client.Cards.Batch(ctx, []string{"sv8-116", "sv8-100", "inventato-xyz"}, &pokemontcgapi.CardGetParams{
    Include: []string{"index"}, // index_eur on list and batch rows is opt-in: 1 credit per 50 cards
    Select:  []string{"id", "name", "index_eur"},
})
// result.Data, result.Requested, result.Found, result.Missing
```

`Missing` is nil when every id resolves. Otherwise each unresolved id appears once
as `MissingCard{ID, SuggestedID}`. A suggestion is included only for an existing historical candidate in a
different canonical set. In this example only `sv8-100` is returned; `sv8-116` suggests `ssp-116`,
and `inventato-xyz` has no suggestion. A canonical set prefix binds the lookup to that set;
`base1-4` still resolves to `bs-4` because `base1` is only a historical alias.

`Data` contains distinct cards. Repeated ids count towards `Requested` and credits, but do not
repeat rows in `Data` or `Missing`. Two valid aliases for one card can make `Found` smaller than
`Requested` with no missing ids. Missing entries ignore case and retain the first spelling and
request order after whitespace trimming. `Withheld` remains an optional top-level field. More than
100 ids returns an error wrapping `ErrTooManyIDs` before any request is made.

### Japanese, and the other seven locales

```go
page, err := client.Sets.Cards(ctx, "sv8", &pokemontcgapi.CardListParams{Lang: "ja", Limit: 1})
fmt.Println(page.Data[0].Name) // タマタマ
```

`Lang` replaces the `Name` field itself and falls back to English where a translation is missing.
Locales, with the rows each one actually has on 16 September 2026: `en` 57,421, `fr` 42,858,
`de` 42,604, `ja` 27,230, `it` 21,644, `es` 21,003, `pt` 13,822, `zh` 3,492. A thin locale answers
mostly in English, because the fallback is per card and not per request.

### Conditional requests are free

```go
client := pokemontcgapi.New(pokemontcgapi.WithETagCache())
```

Every collection carries an ETag. We compute it strong, from the body; the edge rewrites it weak with
an encoding suffix when it compresses, so what you receive looks like `W/"…-gzip"` and you send back
exactly that. With the cache on, the client stores it and replays a `304` without a body, and a `304`
consumes no quota. A mirror that re-syncs often pays only for what changed.

### A photo instead of an id

**Included from the Growth plan up.** On a trial or a Developer key the call answers `403
PLAN_REQUIRED` with `details.min_plan`, before reading the image and without spending credits.

```go
resp, err := client.Vision.Identify(ctx, file, &pokemontcgapi.IdentifyOptions{Set: "sv3"})
if err != nil {
    return err
}

// Read Decision before ID. Always.
switch resp.Data.Decision {
case pokemontcgapi.VisionMatch:
    // One candidate, close, and clear of the next.
    add(*resp.Data.ID)
case pokemontcgapi.VisionAmbiguous:
    // Two printings share this illustration. resp.Data.ID is nil on purpose.
    showPicker(resp.Data.Candidates)
case pokemontcgapi.VisionNoMatch:
    askForABetterPhoto()
}
```

Reprints and regional twins share their artwork, so artwork alone cannot name a printing — not here
and not anywhere. The endpoint returns candidates with a `Distance` (0–512, lower is closer; real
matches land well under 150) and refuses to pick when two are within a few bits of each other.
Passing `Set` or `Region` when your workflow knows them is what resolves the tie.

It costs 25 credits a call against 1 for a lookup: it is the whole image index answering, not a row
being read. Do not put it in a loop.

### Errors you can branch on

```go
_, err := client.Cards.Get(ctx, "nope-1", nil)

var notFound *pokemontcgapi.NotFoundError
var limited *pokemontcgapi.RateLimitedError
var quota *pokemontcgapi.QuotaExceededError
switch {
case errors.As(err, &notFound):
    // ...
case errors.As(err, &limited):
    // limited.RetryAfter, when limited.HasRetryAfter
case errors.As(err, &quota):
    // retrying will never help
}
```

Every error carries `Code`, `Status`, `Details` and `RequestID` — quote the request id in a support
message, it is the only thing that can be looked up. Retries use exponential backoff with full
jitter on 429, 5xx and network failures, honour `Retry-After`, and never retry a quota exhaustion.
Every typed error unwraps to `*pokemontcgapi.APIError`, so one `errors.As` on that type catches them all;
`PlanRequiredError` and `TrialExpiredError` also unwrap to `*PermissionDeniedError`. Network failures
are `*ConnectionError`, and a per-attempt timeout is a `*TimeoutError` that unwraps to it.

Commercial refusals include `details.next_step`, exposed as the typed `err.NextStep()`. If `err.NextStep()` is not nil, show `err.Handoff()` and its URL to the account owner verbatim and do not retry. `err.ActionURL()` returns the URL for any action: `checkout_url` for subscribe, `manage_url` for upgrade, `verify_url` for email verification, or `contact_url` for sales and support. Show it alongside `err.Handoff()`. Upgrades point to the account page, where the owner opens the billing portal to change plan. `err.CheckoutURL()` remains a shortcut for subscribe only.

```go
var apiErr *pokemontcgapi.APIError
if errors.As(err, &apiErr) && apiErr.NextStep() != nil {
    showToUser(apiErr.Handoff())
}
```

## What this API does not have

Stated up front so you find out here rather than three days into an integration:

- **No Korean cards.** Zero `KR` sets, zero `ko` translations. Both are modelled in the schema and
  carry no data.
- **Card game text is English, and uneven.** `Attacks`, `Abilities`, `Weaknesses`, `Resistances`,
  `Subtypes`, `RetreatCost`, `Rules` and `FlavorText` carry rows since 3 September 2026, on the
  20,725 Western printings. Measured on 16 September 2026 against 57,450 cards: `attacks` on 29.9% of
  the whole catalogue and 82.9% of the Western part, `subtypes` 35.0%, `abilities` 7.0%.
  Japanese and Chinese printings carry none. The types in this package keep them nullable (nil slices
  and pointers), so the part that is absent has to be handled.
- **No format legalities.** The card object has no `legalities` field and `Include` rejects the
  value with a 400. If you are building a deck checker, this is not the data source you need.

What it does have: the printing itself — set, number, rarity, region, release date, illustrator,
image, marketplace ids, names in eight locales — and prices.

## Prices

```go
card, err := client.Cards.Get(ctx, "base1-4", &pokemontcgapi.CardGetParams{Include: []string{"prices"}})
for _, price := range card.Prices {
    fmt.Println(price.Source, price.Basis, price.Amount, price.Currency, price.AsOf, price.SampleN)
}
```

The dedicated price routes have their own methods, and they are the ones to use when prices are the
point of the call:

```go
prices, err := client.Prices.Card(ctx, "base1-4", nil)                                        // index + quotes, 2 credits
many, err := client.Prices.Current(ctx, []string{"base1-4", "sv3-125"}, nil)                    // up to 50 ids, 4 credits per 25
history, err := client.Prices.History(ctx, "base1-4", &pokemontcgapi.HistoryParams{Bucket: "week"}) // 5 credits
stats, err := client.Prices.Stats(ctx, "base1-4", &pokemontcgapi.StatsParams{Window: "30d"})       // 2 credits
movers, err := client.Prices.Movers(ctx, &pokemontcgapi.MoversParams{Window: "7d", Direction: "gainers"}) // Growth and up
box, err := client.Sealed.Prices(ctx, "evolving-skies-booster-box", nil)
```

`History` is bounded by your plan (7 days on the trial, 30 on Developer, everything from Growth): a
wider window returns `*UpgradeRequiredError`, whose `PermittedWindow()` says what you may ask for.
`Movers` below Growth returns `*PlanRequiredError`, and a trial past its 30 days returns
`*TrialExpiredError` on every route that costs credits. Both unwrap to `*PermissionDeniedError`.

There is no printing filter on `Include: []string{"prices"}`: first edition, holofoil and graded rows come back together, so read
`Printing`, `Condition` and `Grading` per row. `Basis` separates `GUIDE` (published upstream) from
`DERIVED` (computed by us). `PTCG_INDEX` is a composite index in EUR carrying `SampleN`, and the same
number sits on the card row as `IndexEUR` wherever we have enough observations to compute one: 51,636
cards of 57,450 on 16 September 2026, so treat it as nullable. On a list or batch it comes with `Include: []string{"index"}`
(1 credit per 50 rows), so a list still has a comparable number without a second request per card.

What your plan withholds is named rather than hidden, but it is named in three different places, so
read the one that matches the call you made:

| call | where the exclusions are |
|---|---|
| `client.Prices.Card(ctx, id, nil)` | `Meta.Withheld` |
| `client.Cards.Get(ctx, id, &CardGetParams{Include: []string{"prices"}})` | the `X-Plan-Withheld` header: `client.LastResponse().PlanWithheld` |
| `client.Cards.Batch(ctx, ids, …)` | a top-level `Withheld` field |

The values are `graded` and `non_english_locales`: a trial key gets both, Developer keeps `graded`,
and from Growth up nothing is withheld, in which case the field is absent rather than an empty array.
Read it before concluding that a card has no graded observations: it may be your plan, not the
catalogue. Prices also carry their own `Locale`, and a card read with `Include: []string{"prices"}` returns
every locale your plan allows, so the currency does not tell you the language.

## Credits and quota

Every response says what it cost. The SDK keeps the headers of the last one, and hands each one to
`WithOnResponse` if you want a running total:

```go
var spent int
client := pokemontcgapi.New(pokemontcgapi.WithOnResponse(func(r pokemontcgapi.ResponseInfo) {
    if r.CreditsCost != nil {
        spent += *r.CreditsCost
    }
}))

_, err := client.Cards.Search(ctx, &pokemontcgapi.CardListParams{Q: "name:charizard", Include: []string{"index"}})
info := client.LastResponse()
fmt.Println(*info.CreditsCost, *info.QuotaRemaining)
```

The trial is 800 credits, once, for 30 days, with at most 400 spent in a day; `TrialExpiresAt` on
the same struct says when it ends.

## The change feed

```go
since := store.GetInt64("ptcg_since")
for {
    page, err := client.Changes(ctx, &pokemontcgapi.ChangesParams{Since: since, Limit: 500})
    if err != nil {
        return err
    }
    for _, change := range page.Data {
        apply(change) // Kind, EntityID, Op, Version
    }
    since = page.Meta.NextSince
    store.SetInt64("ptcg_since", since)
    if !page.Meta.HasMore {
        break
    }
}
```

## Also available

- **TypeScript SDK**: [`@pokemontcgapi/sdk`](https://www.npmjs.com/package/@pokemontcgapi/sdk) — [source](https://github.com/pokemontcgapi/sdk-typescript)
- **Python SDK**: `pip install pokemontcgapi` — [source](https://github.com/pokemontcgapi/sdk-python)
- **MCP server** for agents: [`@pokemontcgapi/mcp`](https://www.npmjs.com/package/@pokemontcgapi/mcp) — [source](https://github.com/pokemontcgapi/mcp-server)
- **Docs**: <https://pokemontcgapi.com/docs>
- **Coverage, measured live**: <https://pokemontcgapi.com/coverage>

## Build from source

```bash
go vet ./...
go test -race ./...
```

Go >= 1.23, no dependencies. The tests run offline against the recorded responses in
`testdata/fixtures`; `go test -tags live ./...` with `PTCG_API_KEY` set runs a short smoke test
against the real API (about four credits). CI enforces gofmt, `go vet`, staticcheck and the tests on
Go 1.23 to 1.25, and that a fresh module can import the package.

This package is developed inside the private monorepo that runs
[pokemontcgapi.com](https://pokemontcgapi.com) and mirrored here on each release,
so a merged pull request travels back by hand rather than by merge button. That
is not a reason to send patches elsewhere — open the issue or the PR here, it is
the address that gets read.

README synced from sdk-typescript README as of commit 86a725a.

## Licence

MIT. Data served by the API carries per-source redistribution terms — see
<https://pokemontcgapi.com/legal/attribution>.
