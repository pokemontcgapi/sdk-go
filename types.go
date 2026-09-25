package pokemontcgapi

import (
	"encoding/json"
	"net/url"
)

// The shapes the API returns. They are written by hand from the TypeScript
// SDK's types.ts, field by field, and the comments say what the fields
// actually carry: game text exists only on part of the catalogue, and a type
// that promises attacks on every card makes people write code that never runs.
// Nullable fields are pointers; absent-or-null cannot be told apart, so read
// Meta.Withheld where it matters (index_eur on list rows).

// Locale values with rows in the table (measured 2026-09-16): en, fr, de, ja,
// it, es, pt, zh. The API accepts ko too, but it has zero rows.
const (
	LocaleEN = "en"
	LocaleFR = "fr"
	LocaleDE = "de"
	LocaleJA = "ja"
	LocaleIT = "it"
	LocaleES = "es"
	LocalePT = "pt"
	LocaleZH = "zh"
)

// Print regions. KR exists in the server schema but matches no set: filtering
// on it returns an empty page, not an error.
const (
	RegionWest = "WEST"
	RegionJP   = "JP"
	RegionCN   = "CN"
	RegionKR   = "KR"
)

// Price sources.
const (
	SourceTCGplayer     = "TCGPLAYER"
	SourcePriceCharting = "PRICECHARTING"
	SourceCardmarket    = "CARDMARKET"
	SourceCardTrader    = "CARDTRADER"
	SourceEbay          = "EBAY"
	SourcePTCGIndex     = "PTCG_INDEX"
	SourceCommunity     = "COMMUNITY"
)

// Price bases. DERIVED is computed by us; GUIDE is a figure published upstream.
const (
	BasisGuide   = "GUIDE"
	BasisDerived = "DERIVED"
	BasisSold    = "SOLD"
	BasisAsking  = "ASKING"
)

// Values accepted by CardGetParams.Include and CardListParams.Include.
const (
	IncludeIndex        = "index"
	IncludePrices       = "prices"
	IncludeTranslations = "translations"
	IncludeImages       = "images"
	IncludeSet          = "set"
	IncludeArtist       = "artist"
)

// Grading is the company and score of a graded copy.
type Grading struct {
	Company string `json:"company"`
	Score   string `json:"score"`
}

// Price is one observed price row: who saw it, on what basis, in which
// currency, for which printing and condition, and on which day.
type Price struct {
	Source  string `json:"source"`
	Variant string `json:"variant"`
	Basis   string `json:"basis"`
	// Amount is the figure, with its currency in the sibling field.
	Amount    float64  `json:"amount"`
	Currency  string   `json:"currency"`
	Locale    *string  `json:"locale"`
	Condition *string  `json:"condition"`
	Printing  *string  `json:"printing"`
	Grading   *Grading `json:"grading"`
	// AsOf is the day the observation refers to. Never today: every source is delayed.
	AsOf string `json:"as_of"`
	// SampleN is how many observations sit behind the figure, where the source says.
	SampleN *int `json:"sample_n"`
	// Provenance is the attribution string to print next to the number.
	Provenance string `json:"provenance"`
}

// CardImage is one rendition of a card face.
type CardImage struct {
	Face        string  `json:"face"`
	Size        string  `json:"size"`
	Locale      *string `json:"locale"`
	URL         string  `json:"url"`
	ImageSource *string `json:"image_source"`
	// Width and Height are modelled but not populated: reserve space with a 5:7 ratio.
	Width  *int `json:"width"`
	Height *int `json:"height"`
}

// Translation is a card name in one locale.
type Translation struct {
	Locale string `json:"locale"`
	Name   string `json:"name"`
}

// Card is one printing.
type Card struct {
	ID string `json:"id"`
	// LegacyID is an alternate identifier in the same shape. It resolves on the same route.
	LegacyID       *string  `json:"legacy_id"`
	Name           string   `json:"name"`
	Number         string   `json:"number"`
	NumberSort     *int     `json:"number_sort"`
	Supertype      *string  `json:"supertype"`
	HP             *int     `json:"hp"`
	Level          *string  `json:"level"`
	EvolvesFrom    *string  `json:"evolves_from"`
	EvolvesTo      []string `json:"evolves_to"`
	Rarity         *string  `json:"rarity"`
	RegulationMark *string  `json:"regulation_mark"`

	SetCode     string  `json:"set_code"`
	SetName     string  `json:"set_name"`
	SetTotal    *int    `json:"set_total"`
	PtcgoCode   *string `json:"ptcgo_code"`
	Series      *string `json:"series"`
	ReleaseDate *string `json:"release_date"`
	PrintRegion string  `json:"print_region"`

	ArtistName *string `json:"artist_name"`
	ArtistSlug *string `json:"artist_slug"`

	// IndexEUR is the composite index in euro. Always present on a single
	// card; on list and batch rows only with Include index (1 credit per 50
	// cards). Without it the field is absent and Meta.Withheld contains "index".
	IndexEUR    *float64 `json:"index_eur,omitempty"`
	LastPriceAt *string  `json:"last_price_at,omitempty"`

	TCGplayerID  *int `json:"tcgplayer_id"`
	CardmarketID *int `json:"cardmarket_id"`
	// JPTwinID is the Japanese printing of the same card, where the pairing is known.
	JPTwinID *string `json:"jp_twin_id"`

	RowVersion int    `json:"row_version"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`

	// Relations, only with Include.
	Prices       []Price       `json:"prices,omitempty"`
	Images       []CardImage   `json:"images,omitempty"`
	Translations []Translation `json:"translations,omitempty"`
	Set          *CardSet      `json:"set,omitempty"`
	Artist       *Artist       `json:"artist,omitempty"`

	// Game text. Present in English on Western printings and unevenly:
	// attacks on about a third of the catalogue, nothing on Japanese and
	// Chinese printings. nil means "not held", never "the card has no attacks".
	Attacks                []json.RawMessage `json:"attacks"`
	Abilities              []json.RawMessage `json:"abilities"`
	Weaknesses             []json.RawMessage `json:"weaknesses"`
	Resistances            []json.RawMessage `json:"resistances"`
	Subtypes               []string          `json:"subtypes"`
	RetreatCost            []string          `json:"retreat_cost"`
	ConvertedRetreatCost   *int              `json:"converted_retreat_cost"`
	Rules                  []string          `json:"rules"`
	FlavorText             *string           `json:"flavor_text"`
	Types                  []string          `json:"types"`
	NationalPokedexNumbers []int             `json:"national_pokedex_numbers"`
}

// CardSet is one set.
type CardSet struct {
	ID           string  `json:"id"`
	Code         string  `json:"code"`
	Slug         string  `json:"slug"`
	LegacyID     *string `json:"legacy_id"`
	Name         string  `json:"name"`
	Series       *string `json:"series"`
	Region       string  `json:"region"`
	ReleaseDate  *string `json:"release_date"`
	Total        *int    `json:"total"`
	PrintedTotal *int    `json:"printed_total"`
	PtcgoCode    *string `json:"ptcgo_code"`
	SymbolURL    *string `json:"symbol_url"`
	LogoURL      *string `json:"logo_url"`
	UpdatedAt    string  `json:"updated_at,omitempty"`
}

// Artist is an illustrator.
type Artist struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	CardCount int    `json:"card_count"`
	// Links is set only by Artists.Get: the ready-made search for the artist's cards.
	Links *struct {
		Cards string `json:"cards,omitempty"`
	} `json:"links,omitempty"`
}

// CatalogStatus is the answer of /v1/status: counts and freshness per source.
type CatalogStatus struct {
	Status  string `json:"status"`
	Catalog struct {
		Sets    int `json:"sets"`
		Cards   int `json:"cards"`
		Sealed  int `json:"sealed"`
		Artists int `json:"artists"`
	} `json:"catalog"`
	Sources []struct {
		Source        string   `json:"source"`
		LastSuccessAt *string  `json:"last_success_at"`
		AgeHours      *float64 `json:"age_hours"`
		// State is fresh, stale, critical or never_run.
		State string `json:"state"`
	} `json:"sources"`
	Upstream struct {
		ContractOK bool    `json:"contract_ok"`
		Error      *string `json:"error"`
	} `json:"upstream"`
	Version string `json:"version"`
}

// Health is the liveness probe.
type Health struct {
	Status  string  `json:"status"`
	DB      bool    `json:"db"`
	UptimeS float64 `json:"uptime_s"`
	Version string  `json:"version"`
}

// CollectionMeta describes a page of a collection.
type CollectionMeta struct {
	Limit      int  `json:"limit"`
	Count      int  `json:"count"`
	TotalCount *int `json:"total_count,omitempty"`
	HasMore    bool `json:"has_more"`
	// Withheld names what the response left out: "index" when select names
	// index_eur without Include index, or the price rows the plan does not cover.
	Withheld []string `json:"withheld,omitempty"`
	// Warnings are grace-period parameter warnings; distinct from search hints.
	Warnings []string `json:"warnings,omitempty"`
	// Hints are present when a search by number or set name has a useful suggestion.
	Hints []SearchHint `json:"hints,omitempty"`
}

// SearchHint is one search suggestion. Code says which fields are filled:
// NUMBER_NORMALIZED (Received, Matched), TRY_POKEDEX_NUMBER (SuggestedQ,
// Matches, AtLeast), SET_ALIAS_MATCHED / SET_WORDS_MATCHED (Matched),
// SET_ALIAS_ELSEWHERE (SetCode, SetName).
type SearchHint struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Received   string `json:"received,omitempty"`
	Matched    string `json:"matched,omitempty"`
	SuggestedQ string `json:"suggested_q,omitempty"`
	// Matches is capped at 50; read AtLeast when the real count is higher.
	Matches int    `json:"matches,omitempty"`
	AtLeast bool   `json:"at_least,omitempty"`
	SetCode string `json:"set_code,omitempty"`
	SetName string `json:"set_name,omitempty"`
}

// Collection is the envelope of every list route.
type Collection[T any] struct {
	Data  []T            `json:"data"`
	Meta  CollectionMeta `json:"meta"`
	Links *struct {
		Next string `json:"next,omitempty"`
	} `json:"links,omitempty"`
}

// MissingCard is one id a batch could not resolve.
type MissingCard struct {
	ID string `json:"id"`
	// SuggestedID is present only for an existing historical alias in a different canonical set.
	SuggestedID string `json:"suggested_id,omitempty"`
}

// BatchResult is the answer of the batch routes.
type BatchResult[T any] struct {
	Data      []T `json:"data"`
	Requested int `json:"requested"`
	Found     int `json:"found"`
	// Missing is absent when every requested id resolves; repeated ids appear once.
	Missing  []MissingCard `json:"missing,omitempty"`
	Withheld []string      `json:"withheld,omitempty"`
}

// ListParams are the parameters shared by the simple list routes.
//
// Q is the search grammar. Field names there are camelCase and dotted
// (set.code, nationalPokedexNumbers) while response keys are snake_case
// (set_code): two vocabularies, and mixing them gives a 400 listing the valid ones.
type ListParams struct {
	Q       string
	Select  []string
	OrderBy string
	Limit   int
	Cursor  string
}

func (p *ListParams) values() url.Values {
	q := newQuery()
	if p != nil {
		q.str("q", p.Q)
		q.list("select", p.Select)
		q.str("orderBy", p.OrderBy)
		q.int("limit", p.Limit)
		q.str("cursor", p.Cursor)
	}
	return q.Values
}

// CardListParams are the parameters of Cards.Search and Sets.Cards.
type CardListParams struct {
	Q       string
	Select  []string
	OrderBy string
	Limit   int
	Cursor  string
	Include []string
	// Lang replaces Name with the name in that locale; falls back to en.
	Lang string
	// Set narrows to one or more sets by code, slug or alternate id, instead of q=set.id:...
	Set []string
}

func (p *CardListParams) values() url.Values {
	q := newQuery()
	if p != nil {
		q.str("q", p.Q)
		q.list("select", p.Select)
		q.str("orderBy", p.OrderBy)
		q.int("limit", p.Limit)
		q.str("cursor", p.Cursor)
		q.list("include", p.Include)
		q.str("lang", p.Lang)
		q.list("set", p.Set)
	}
	return q.Values
}

// CardGetParams are the parameters of Cards.Get and Cards.Batch.
type CardGetParams struct {
	Select  []string
	Include []string
	Lang    string
}

func (p *CardGetParams) values() url.Values {
	q := newQuery()
	if p != nil {
		q.list("select", p.Select)
		q.list("include", p.Include)
		q.str("lang", p.Lang)
	}
	return q.Values
}

// SetListParams are the parameters of Sets.List.
type SetListParams struct {
	Q       string
	OrderBy string
	Limit   int
	Cursor  string
	Region  string
	Series  string
	Lang    string
}

func (p *SetListParams) values() url.Values {
	q := newQuery()
	if p != nil {
		q.str("q", p.Q)
		q.str("orderBy", p.OrderBy)
		q.int("limit", p.Limit)
		q.str("cursor", p.Cursor)
		q.str("region", p.Region)
		q.str("series", p.Series)
		q.str("lang", p.Lang)
	}
	return q.Values
}

// SetGetParams are the parameters of Sets.Get and Sealed.Get.
type SetGetParams struct {
	Lang string
}

func (p *SetGetParams) values() url.Values {
	q := newQuery()
	if p != nil {
		q.str("lang", p.Lang)
	}
	return q.Values
}

// Series is one series (Scarlet & Violet, Sword & Shield, ...).
type Series struct {
	ID       string `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	SetCount int    `json:"set_count"`
}

// SealedProduct is a booster box, ETB, tin, blister or collection.
type SealedProduct struct {
	ID   string `json:"id"`
	SKU  string `json:"sku"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	// Kind is BOOSTER_BOX and the like.
	Kind        string   `json:"kind"`
	SetCode     *string  `json:"set_code"`
	SetName     *string  `json:"set_name"`
	ImageURL    *string  `json:"image_url"`
	ReleaseDate *string  `json:"release_date"`
	PackCount   *int     `json:"pack_count"`
	Languages   []string `json:"languages"`
	// IndexEUR is on lists only with Include index; on the single product always.
	IndexEUR    *float64 `json:"index_eur,omitempty"`
	LastPriceAt *string  `json:"last_price_at,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// SealedListParams are the parameters of Sealed.List.
type SealedListParams struct {
	Q       string
	Set     []string
	Kind    string
	Lang    string
	Include []string
	OrderBy string
	Limit   int
	Cursor  string
}

func (p *SealedListParams) values() url.Values {
	q := newQuery()
	if p != nil {
		q.str("q", p.Q)
		q.list("set", p.Set)
		q.str("kind", p.Kind)
		q.str("lang", p.Lang)
		q.list("include", p.Include)
		q.str("orderBy", p.OrderBy)
		q.int("limit", p.Limit)
		q.str("cursor", p.Cursor)
	}
	return q.Values
}

// PriceIndex is the composite index of a card, with one series per (locale, printing).
type PriceIndex struct {
	EUR     float64 `json:"eur"`
	AsOf    string  `json:"as_of"`
	SampleN *int    `json:"sample_n"`
	// ByLocale has one series per (locale, printing): the head index is the English one.
	ByLocale []struct {
		Locale   string  `json:"locale"`
		Printing *string `json:"printing"`
		EUR      float64 `json:"eur"`
		AsOf     string  `json:"as_of"`
		SampleN  *int    `json:"sample_n"`
	} `json:"by_locale"`
}

// PriceFilterParams narrow a price read to a source, variant or locale.
type PriceFilterParams struct {
	Source  string
	Variant string
	Locale  string
}

func (p *PriceFilterParams) values() url.Values {
	q := newQuery()
	if p != nil {
		q.str("source", p.Source)
		q.str("variant", p.Variant)
		q.str("locale", p.Locale)
	}
	return q.Values
}

// CardPrices is the index and current quotes of one card.
type CardPrices struct {
	CardID string      `json:"card_id"`
	Index  *PriceIndex `json:"index"`
	Quotes []Price     `json:"quotes"`
}

// SealedPrices is the current quotes of one sealed product. Index is always nil: the composite index does not cover sealed products.
type SealedPrices struct {
	SealedID string      `json:"sealed_id"`
	Index    *PriceIndex `json:"index"`
	Quotes   []Price     `json:"quotes"`
}

// PricesResponse is the envelope of the dedicated price routes.
type PricesResponse[T any] struct {
	Data T `json:"data"`
	Meta struct {
		Quotes       int `json:"quotes"`
		DelayedHours int `json:"delayed_hours"`
		// Withheld names the rows the plan does not cover: graded, non_english_locales. Absent if nothing is withheld.
		Withheld []string `json:"withheld,omitempty"`
	} `json:"meta"`
}

// HistoryParams are the parameters of Prices.History.
type HistoryParams struct {
	Source   string
	Variant  string
	Locale   string
	Printing string
	// From and To are YYYY-MM-DD. A window wider than the plan gives *UpgradeRequiredError.
	From   string
	To     string
	Bucket string // day, week or month
}

func (p *HistoryParams) values() url.Values {
	q := newQuery()
	if p != nil {
		q.str("source", p.Source)
		q.str("variant", p.Variant)
		q.str("locale", p.Locale)
		q.str("printing", p.Printing)
		q.str("from", p.From)
		q.str("to", p.To)
		q.str("bucket", p.Bucket)
	}
	return q.Values
}

// HistoryPoint is one bucket of the price history.
type HistoryPoint struct {
	Date     string  `json:"date"`
	Source   string  `json:"source"`
	Variant  string  `json:"variant"`
	Locale   *string `json:"locale"`
	Printing *string `json:"printing"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	SampleN  *int    `json:"sample_n"`
}

// HistoryResponse is the answer of Prices.History.
type HistoryResponse struct {
	Data []HistoryPoint `json:"data"`
	Meta struct {
		CardID    string `json:"card_id"`
		From      string `json:"from"`
		To        string `json:"to"`
		Bucket    string `json:"bucket"`
		Count     int    `json:"count"`
		Truncated bool   `json:"truncated"`
		Capped    bool   `json:"capped"`
		// PlanWindowDays is nil when the plan gives the whole history.
		PlanWindowDays *int `json:"plan_window_days"`
	} `json:"meta"`
}

// StatsParams are the parameters of Prices.Stats.
type StatsParams struct {
	Window string
	Locale string
}

func (p *StatsParams) values() url.Values {
	q := newQuery()
	if p != nil {
		q.str("window", p.Window)
		q.str("locale", p.Locale)
	}
	return q.Values
}

// PriceStats summarises the index over a window.
type PriceStats struct {
	Window    string   `json:"window"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Low       *float64 `json:"low"`
	High      *float64 `json:"high"`
	Median    *float64 `json:"median"`
	First     *float64 `json:"first"`
	Last      *float64 `json:"last"`
	ChangePct *float64 `json:"change_pct"`
	SampleN   int      `json:"sample_n"`
	Currency  string   `json:"currency"`
}

// StatsResponse is the answer of Prices.Stats.
type StatsResponse struct {
	Data PriceStats `json:"data"`
	Meta struct {
		CardID string `json:"card_id"`
		Source string `json:"source"`
	} `json:"meta"`
}

// MoversParams are the parameters of Prices.Movers.
type MoversParams struct {
	Window    string
	Direction string // gainers or losers
	// MinValue in euro keeps cards worth a few cents out.
	MinValue float64
	Locale   string
	// Limit is 1..50.
	Limit int
}

func (p *MoversParams) values() url.Values {
	q := newQuery()
	if p != nil {
		q.str("window", p.Window)
		q.str("direction", p.Direction)
		q.float("min_value", p.MinValue)
		q.str("locale", p.Locale)
		q.int("limit", p.Limit)
	}
	return q.Values
}

// Mover is one card in the movers list.
type Mover struct {
	CardID    string  `json:"card_id"`
	Name      string  `json:"name"`
	SetCode   string  `json:"set_code"`
	From      float64 `json:"from"`
	To        float64 `json:"to"`
	ChangePct float64 `json:"change_pct"`
	Currency  string  `json:"currency"`
}

// MoversResponse is the answer of Prices.Movers.
type MoversResponse struct {
	Data []Mover `json:"data"`
	Meta struct {
		Window    string  `json:"window"`
		From      string  `json:"from"`
		To        string  `json:"to"`
		Direction string  `json:"direction"`
		MinValue  float64 `json:"min_value"`
		Count     int     `json:"count"`
		Source    string  `json:"source"`
	} `json:"meta"`
}

// PriceSourceInfo describes one price source and its declared delay.
type PriceSourceInfo struct {
	Source        string `json:"source"`
	Label         string `json:"label"`
	MinDelayHours int    `json:"min_delay_hours"`
	IsOwn         bool   `json:"is_own"`
}

// Change is one row of the incremental feed.
type Change struct {
	ID int64 `json:"id"`
	// Kind is SET, CARD, ...: the real list is change_kinds in /v1/reference.
	Kind      string `json:"kind"`
	EntityID  string `json:"entity_id"`
	Op        string `json:"op"`
	Version   int    `json:"version"`
	ChangedAt string `json:"changed_at"`
}

// ChangesParams are the parameters of Client.Changes.
type ChangesParams struct {
	// Since is the last next_since received. Zero means from the oldest available.
	Since int64
	Kind  string
	Limit int
}

func (p *ChangesParams) values() url.Values {
	q := newQuery()
	if p != nil {
		q.int64("since", p.Since)
		q.str("kind", p.Kind)
		q.int("limit", p.Limit)
	}
	return q.Values
}

// ChangesResponse is the answer of Client.Changes.
type ChangesResponse struct {
	Data []Change `json:"data"`
	Meta struct {
		Count   int  `json:"count"`
		HasMore bool `json:"has_more"`
		// NextSince is to be stored and sent back as Since: the feed has no other state.
		NextSince       int64 `json:"next_since"`
		Watermark       int64 `json:"watermark"`
		OldestAvailable int64 `json:"oldest_available"`
		Behind          int64 `json:"behind"`
	} `json:"meta"`
	Links *struct {
		Next string `json:"next,omitempty"`
	} `json:"links,omitempty"`
}

// Vision decisions. Ambiguous is not a failure: it is the normal case on
// reprints, where two printings share the illustration and cannot be told
// apart from the image alone.
const (
	VisionMatch     = "match"
	VisionAmbiguous = "ambiguous"
	VisionNoMatch   = "no_match"
)

// VisionCandidate is one ranked candidate.
type VisionCandidate struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number string `json:"number"`
	Set    struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		PrintRegion string `json:"print_region"`
	} `json:"set"`
	Rarity   *string `json:"rarity"`
	ImageURL *string `json:"image_url"`
	// Distance is a Hamming distance, 0..512: real matches sit under 150 even on a noisy photo, and nothing above 170 is returned.
	Distance int `json:"distance"`
	// Confidence is the same information rescaled to 0..1.
	Confidence float64 `json:"confidence"`
}

// VisionResult is the outcome of a photo recognition.
type VisionResult struct {
	Decision       string `json:"decision"`
	DecisionReason string `json:"decision_reason"` // clear, reprint, close_call, none, ocr_number, ocr_set_number
	// ID is set ONLY when Decision is match. Otherwise nil.
	ID         *string           `json:"id"`
	Candidates []VisionCandidate `json:"candidates"`
}

// VisionMeta describes how the recognition went.
type VisionMeta struct {
	Count            int    `json:"count"`
	CardsIndexed     int    `json:"cards_indexed"`
	IndexBuiltAt     string `json:"index_built_at"`
	IndexLoadedAt    string `json:"index_loaded_at"`
	SignatureVersion int    `json:"signature_version"`
	// RegionsDetected is how many card-like quadrilaterals were isolated. Zero with a no_match means the card was not found in the photo, not that it is not in the catalogue.
	RegionsDetected int `json:"regions_detected"`
	HypothesesTried int `json:"hypotheses_tried"`
	ElapsedMs       int `json:"elapsed_ms"`
	OCR             *struct {
		Applied   bool   `json:"applied"`
		Eligible  int    `json:"eligible"`
		Level     int    `json:"level,omitempty"`
		Number    int    `json:"number,omitempty"`
		Ms        int    `json:"ms"`
		CropMs    int    `json:"crop_ms"`
		Reason    string `json:"reason,omitempty"`
		SetCode   string `json:"set_code,omitempty"`
		SetReason string `json:"set_reason,omitempty"`
	} `json:"ocr,omitempty"`
}

// VisionResponse is the answer of Vision.Identify.
type VisionResponse struct {
	Data VisionResult `json:"data"`
	Meta VisionMeta   `json:"meta"`
}

// IdentifyOptions narrow a photo recognition.
type IdentifyOptions struct {
	// TopK is how many candidates, 1..10.
	TopK int
	// Set restricts to one set. It is the hint that resolves a reprint.
	Set string
	// Region restricts to a print region. Same purpose.
	Region string
}
