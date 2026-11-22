package agilisads

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Gender values for Offer targeting.
const (
	GenderMale      = "Male"
	GenderFemale    = "Female"
	GenderNonBinary = "Non-binary"
	GenderAll       = "All"
)

// Payment methods. An Event can only be paid from the balance.
const (
	// PaymentMethodLink sends a Stripe payment link by email (Offers only).
	PaymentMethodLink = "E"

	// PaymentMethodBalance deducts the price from the balance of the mode.
	PaymentMethodBalance = "B"
)

// Button presets for an Offer's button.
const (
	ButtonClaimOffer    = "claim_offer"
	ButtonVisitWebsite  = "visit_website"
	ButtonInstallApp    = "install_app"
	ButtonBookNow       = "book_now"
	ButtonGetDirections = "get_directions"
	ButtonCustom        = "custom"
)

// Analytics intervals.
const (
	Interval24Hours = "24h"
	Interval7Days   = "7d"
	Interval30Days  = "30d"
	Interval60Days  = "60d"
	Interval90Days  = "90d"
	IntervalAll     = "all"
)

// OffersService creates and manages Offers.
type OffersService struct{ client *Client }

// EstimateOfferPriceParams are the parameters of Offers.EstimatePrice.
type EstimateOfferPriceParams struct {
	StartDatetime string      `json:"start_datetime"`
	EndDatetime   string      `json:"end_datetime"`
	Location      LatLng      `json:"location"`
	Radius        int         `json:"radius"`
	Gender        []string    `json:"gender,omitempty"`
	Interests     []int64     `json:"interests,omitempty"`
	Environment   Environment `json:"environment,omitempty"`
}

// OfferPriceEstimate is the response of Offers.EstimatePrice.
type OfferPriceEstimate struct {
	Response
	ReturnCode         string   `json:"return_code"`
	PriceEstimate      float64  `json:"price_estimate"`
	MatchingUsers      int64    `json:"matching_users"`
	AdUnits            int64    `json:"ad_units"`
	AdUnitPrice        float64  `json:"ad_unit_price"`
	EstimatedCost      float64  `json:"estimated_cost"`
	Currency           string   `json:"currency"`
	ExchangeRatePerGBP *float64 `json:"exchange_rate_per_gbp"`
	ExchangeRateDate   *string  `json:"exchange_rate_date"`
}

// EstimatePrice estimates what an Offer would cost
// (POST /offers/price-estimate). The environment is filled in from the key
// when left empty.
func (s *OffersService) EstimatePrice(ctx context.Context, params *EstimateOfferPriceParams) (*OfferPriceEstimate, error) {
	if params == nil {
		return nil, errNilParams
	}
	p := *params
	p.Environment = s.client.environment(p.Environment)
	r := &request{method: http.MethodPost, path: "/offers/price-estimate"}
	if err := r.setJSON(p); err != nil {
		return nil, err
	}
	out := &OfferPriceEstimate{}
	if err := s.client.do(ctx, r, out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateOfferParams are the parameters of Offers.Create.
type CreateOfferParams struct {
	CategoryID              int64
	Title                   string
	Description             string
	TermsAndConditions      *string
	AdvertisedName          *string
	PushNotificationTitle   string
	PushNotificationMessage string

	// ButtonPreset is one of the Button constants; empty means claim_offer.
	ButtonPreset string

	// ButtonText is required when ButtonPreset is ButtonCustom.
	ButtonText *string
	URL        string
	Image      *File
	Location   LatLng

	// Radius is in miles.
	Radius    int
	Gender    []string
	Interests []int64

	// StartDatetime and ExpiryDatetime are ISO 8601, UTC.
	StartDatetime  string
	ExpiryDatetime string

	// PaymentMethod is PaymentMethodLink or PaymentMethodBalance.
	PaymentMethod string

	// Environment is filled in from the key when empty.
	Environment Environment

	// IdempotencyKey is generated (a UUID v4) when empty, and reused on
	// every retry of the call.
	IdempotencyKey string

	// DryRun checks everything and answers with the cost, creating nothing.
	DryRun bool
}

// CreatedOffer is the response of Offers.Create. A dry run's price,
// matching_members, balance_after and card_amount are reachable with Field.
type CreatedOffer struct {
	Response
	ReturnCode string  `json:"return_code"`
	OfferID    int64   `json:"offer_id"`
	Status     string  `json:"status"`
	PaymentURL *string `json:"payment_url"`
	Currency   string  `json:"currency"`
	DryRun     bool    `json:"dry_run"`

	// IdempotencyKey is the key that was sent.
	IdempotencyKey string `json:"-"`
}

// Create creates an Offer (POST /offers), sent as multipart/form-data with
// its image.
func (s *OffersService) Create(ctx context.Context, params *CreateOfferParams) (*CreatedOffer, error) {
	if params == nil {
		return nil, errNilParams
	}
	key := params.IdempotencyKey
	if key == "" {
		var err error
		if key, err = newIdempotencyKey(); err != nil {
			return nil, err
		}
	}
	f := newForm()
	f.field("category_id", strconv.FormatInt(params.CategoryID, 10))
	f.field("title", params.Title)
	f.field("description", params.Description)
	f.optStr("terms_and_conditions", params.TermsAndConditions)
	f.optStr("advertised_name", params.AdvertisedName)
	f.field("push_notification_title", params.PushNotificationTitle)
	f.field("push_notification_message", params.PushNotificationMessage)
	f.str("button_preset", params.ButtonPreset)
	f.optStr("button_text", params.ButtonText)
	f.field("url", params.URL)
	f.location(params.Location)
	f.field("radius", strconv.Itoa(params.Radius))
	for _, g := range params.Gender {
		f.field("gender[]", g)
	}
	for _, i := range params.Interests {
		f.field("interests[]", strconv.FormatInt(i, 10))
	}
	f.field("start_datetime", params.StartDatetime)
	f.field("expiry_datetime", params.ExpiryDatetime)
	f.field("payment_method", params.PaymentMethod)
	f.str("environment", string(s.client.environment(params.Environment)))
	f.file("image", params.Image)
	r := &request{method: http.MethodPost, path: "/offers", query: dryRunQuery(params.DryRun), idempotencyKey: key}
	if err := r.setForm(f); err != nil {
		return nil, err
	}
	out := &CreatedOffer{}
	if err := s.client.do(ctx, r, out); err != nil {
		return nil, err
	}
	out.IdempotencyKey = key
	return out, nil
}

// Offer is an Offer as the list returns it.
type Offer struct {
	OfferID                 int64      `json:"offer_id"`
	Title                   string     `json:"title"`
	Category                string     `json:"category"`
	Description             string     `json:"description"`
	TermsAndConditions      *string    `json:"terms_and_conditions"`
	AdvertisedName          *string    `json:"advertised_name"`
	PushNotificationTitle   *string    `json:"push_notification_title"`
	PushNotificationMessage *string    `json:"push_notification_message"`
	ButtonPreset            string     `json:"button_preset"`
	ButtonText              *string    `json:"button_text"`
	URL                     string     `json:"url"`
	Image                   string     `json:"image"`
	Latitude                float64    `json:"latitude"`
	Longitude               float64    `json:"longitude"`
	Radius                  float64    `json:"radius"`
	Gender                  []string   `json:"gender"`
	Interests               []Interest `json:"interests"`
	StartDatetime           string     `json:"start_datetime"`
	ExpiryDatetime          string     `json:"expiry_datetime"`
	CreationDatetime        string     `json:"creation_datetime"`

	// OfferStatus is Awaiting Payment, Active, Scheduled, Paused or Ended.
	OfferStatus string `json:"offer_status"`

	// Status is enabled or disabled.
	Status string `json:"status"`
}

// ListOffersParams are the parameters of Offers.List. All are optional.
type ListOffersParams struct {
	// Environment is filled in from the key when empty.
	Environment Environment

	// Limit is the page size: default 20, at most 100.
	Limit int

	// Cursor is the next_cursor of the previous page.
	Cursor string

	// OfferStatus is awaiting_payment, active, paused or ended.
	OfferStatus string

	// CreatedAfter and CreatedBefore are ISO 8601, UTC.
	CreatedAfter  string
	CreatedBefore string
}

// OfferList is one page of Offers.
type OfferList struct {
	Response
	Data       []Offer    `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// List returns one page of Offers, newest first (GET /offers).
func (s *OffersService) List(ctx context.Context, params *ListOffersParams) (*OfferList, error) {
	var p ListOffersParams
	if params != nil {
		p = *params
	}
	q := url.Values{}
	if env := s.client.environment(p.Environment); env != "" {
		q.Set("environment", string(env))
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	setNonEmpty(q, "cursor", p.Cursor)
	setNonEmpty(q, "offer_status", p.OfferStatus)
	setNonEmpty(q, "created_after", p.CreatedAfter)
	setNonEmpty(q, "created_before", p.CreatedBefore)
	out := &OfferList{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/offers", query: q}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListAutoPaging returns an iterator over every Offer, following next_cursor
// until has_more is false.
func (s *OffersService) ListAutoPaging(ctx context.Context, params *ListOffersParams) *Iter[Offer] {
	var p ListOffersParams
	if params != nil {
		p = *params
	}
	return newIter(ctx, p.Cursor, func(ctx context.Context, cursor string) ([]Offer, Pagination, error) {
		page := p
		page.Cursor = cursor
		res, err := s.List(ctx, &page)
		if err != nil {
			return nil, Pagination{}, err
		}
		return res.Data, res.Pagination, nil
	})
}

// ActionResult is the response of an action such as pausing or deleting.
type ActionResult struct {
	Response
	ReturnCode string `json:"return_code"`
	Message    string `json:"message"`
	DryRun     bool   `json:"dry_run"`
}

// Pause pauses an Offer (POST /offers/{offer_id}/pause).
func (s *OffersService) Pause(ctx context.Context, offerID int64) (*ActionResult, error) {
	out := &ActionResult{}
	if err := s.client.do(ctx, &request{method: http.MethodPost, path: "/offers/" + pathInt(offerID) + "/pause"}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Resume resumes a paused Offer (POST /offers/{offer_id}/resume). Its
// return_code is S2.
func (s *OffersService) Resume(ctx context.Context, offerID int64) (*ActionResult, error) {
	out := &ActionResult{}
	if err := s.client.do(ctx, &request{method: http.MethodPost, path: "/offers/" + pathInt(offerID) + "/resume"}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// AnalyticsParams are the parameters of Offers.Analytics and
// Events.Analytics.
type AnalyticsParams struct {
	// Interval is one of the Interval constants. Required.
	Interval string
}

// OfferMetrics are an Offer's figures.
type OfferMetrics struct {
	Views  int64 `json:"views"`
	Clicks int64 `json:"clicks"`
}

// OfferAnalytics is the response of Offers.Analytics.
type OfferAnalytics struct {
	Response
	OfferID       int64        `json:"offer_id"`
	Interval      string       `json:"interval"`
	StartDatetime string       `json:"start_datetime"`
	EndDatetime   string       `json:"end_datetime"`
	Status        string       `json:"status"`
	Metrics       OfferMetrics `json:"metrics"`
}

// Analytics returns an Offer's figures over an interval
// (GET /offers/{offer_id}/analytics).
func (s *OffersService) Analytics(ctx context.Context, offerID int64, params *AnalyticsParams) (*OfferAnalytics, error) {
	if params == nil {
		return nil, errNilParams
	}
	q := url.Values{"interval": {params.Interval}}
	out := &OfferAnalytics{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/offers/" + pathInt(offerID) + "/analytics", query: q}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteParams are the optional parameters of the delete and cancel
// operations.
type DeleteParams struct {
	// DryRun answers with what the operation would do, changing nothing.
	DryRun bool
}

// Delete deletes an Offer (DELETE /offers/{offer_id}). A dry run's refund is
// reachable with Field("refund"). params may be nil.
func (s *OffersService) Delete(ctx context.Context, offerID int64, params *DeleteParams) (*ActionResult, error) {
	out := &ActionResult{}
	r := &request{method: http.MethodDelete, path: "/offers/" + pathInt(offerID), query: dryRunQuery(params != nil && params.DryRun)}
	if err := s.client.do(ctx, r, out); err != nil {
		return nil, err
	}
	return out, nil
}

func setNonEmpty(q url.Values, name, value string) {
	if value != "" {
		q.Set(name, value)
	}
}
