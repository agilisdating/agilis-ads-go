package agilisads

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// EventsService publishes and manages Events.
type EventsService struct{ client *Client }

// EventCategoryRef is the category determined for an Event.
type EventCategoryRef struct {
	CategoryID   int64  `json:"category_id"`
	CategoryName string `json:"category_name"`
}

// EventOwner is the user who published an Event.
type EventOwner struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

// Band is a figure the API gives as a band, such as "10-25", never an exact
// number. It also accepts a JSON number.
type Band string

// UnmarshalJSON accepts a string, a number or null.
func (b *Band) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*b = Band(s)
		return nil
	}
	*b = Band(data)
	return nil
}

// Event is an Event as the list returns it.
type Event struct {
	EventID           int64            `json:"event_id"`
	Title             string           `json:"title"`
	Description       string           `json:"description"`
	Category          EventCategoryRef `json:"category"`
	VenueName         string           `json:"venue_name"`
	Address           string           `json:"address"`
	City              string           `json:"city"`
	CountryCode       string           `json:"country_code"`
	Latitude          float64          `json:"latitude"`
	Longitude         float64          `json:"longitude"`
	StartsAt          string           `json:"starts_at"`
	EndsAt            string           `json:"ends_at"`
	URL               string           `json:"url"`
	AdvertisedName    *string          `json:"advertised_name"`
	PriceMin          *float64         `json:"price_min"`
	PriceMax          *float64         `json:"price_max"`
	Currency          *string          `json:"currency"`
	Image             string           `json:"image"`
	ImageLarge        string           `json:"image_large"`
	ImageThumb        string           `json:"image_thumb"`
	EventStatus       string           `json:"event_status"`
	Attendees         Band             `json:"attendees"`
	Owner             *EventOwner      `json:"owner"`
	Environment       string           `json:"environment"`
	RescheduleCount   int              `json:"reschedule_count"`
	ClonedFromEventID *int64           `json:"cloned_from_event_id"`
	CreationDatetime  string           `json:"creation_datetime"`
	LastEditedAt      *string          `json:"last_edited_at"`
}

// CropArea is the area of an image in use, in pixels.
type CropArea struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// EventImageMaster is the stored original of an Event image.
type EventImageMaster struct {
	Width  int      `json:"width"`
	Height int      `json:"height"`
	Crop   CropArea `json:"crop"`
	URL    *string  `json:"url"`
}

// EventHistoryEntry is one action taken on an Event.
type EventHistoryEntry struct {
	Action        string          `json:"action"`
	Channel       string          `json:"channel"`
	ActedByUserID *int64          `json:"acted_by_user_id"`
	ActedBy       *string         `json:"acted_by"`
	Details       json.RawMessage `json:"details"`
	Acknowledged  bool            `json:"acknowledged"`
	At            string          `json:"at"`
}

// EventDetails is the response of Events.Get: every field of the list row,
// plus the cost, permissions, image and history.
type EventDetails struct {
	Response
	Event
	CanManage            bool                `json:"can_manage"`
	CanDelete            bool                `json:"can_delete"`
	ListingPrice         *float64            `json:"listing_price"`
	BillingCurrency      *string             `json:"billing_currency"`
	InvoiceID            *string             `json:"invoice_id"`
	AppSyncedAt          *string             `json:"app_synced_at"`
	HasMemberInteraction bool                `json:"has_member_interaction"`
	ImageMaster          *EventImageMaster   `json:"image_master"`
	History              []EventHistoryEntry `json:"history"`
}

// PreviewEventParams are the parameters of Events.Preview. Every field is
// optional except the environment, which is filled in from the key when
// empty.
type PreviewEventParams struct {
	Title          string
	Description    string
	VenueName      string
	Address        string
	City           string
	CountryCode    string
	Location       *LatLng
	StartsAt       string
	EndsAt         string
	URL            string
	AdvertisedName *string
	PriceMin       *float64
	PriceMax       *float64
	Currency       string
	Image          *File
	Crop           *Crop
	PaymentMethod  string
	Environment    Environment
}

// EventPreviewImage is the processed image of a preview.
type EventPreviewImage struct {
	PreviewURL   string   `json:"preview_url"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	MasterWidth  int      `json:"master_width"`
	MasterHeight int      `json:"master_height"`
	Crop         CropArea `json:"crop"`

	// PreviewToken can be passed to Events.Publish instead of the image.
	PreviewToken string `json:"preview_token"`
	ExpiresAt    string `json:"expires_at"`
}

// EventPreview is the response of Events.Preview.
type EventPreview struct {
	Response
	ReturnCode         string             `json:"return_code"`
	Category           EventCategoryRef   `json:"category"`
	ListingFee         *float64           `json:"listing_fee"`
	Currency           string             `json:"currency"`
	Balance            float64            `json:"balance"`
	BalanceAfter       float64            `json:"balance_after"`
	BalanceSufficient  bool               `json:"balance_sufficient"`
	LivePublishingOpen bool               `json:"live_publishing_open"`
	PriceGBP           *float64           `json:"price_gbp"`
	Image              *EventPreviewImage `json:"image"`
}

// Preview shows what publishing would do, without publishing or charging
// (POST /events/preview).
func (s *EventsService) Preview(ctx context.Context, params *PreviewEventParams) (*EventPreview, error) {
	if params == nil {
		return nil, errNilParams
	}
	f := newForm()
	f.str("title", params.Title)
	f.str("description", params.Description)
	f.str("venue_name", params.VenueName)
	f.str("address", params.Address)
	f.str("city", params.City)
	f.str("country_code", params.CountryCode)
	if params.Location != nil {
		f.location(*params.Location)
	}
	f.str("starts_at", params.StartsAt)
	f.str("ends_at", params.EndsAt)
	f.str("url", params.URL)
	f.optStr("advertised_name", params.AdvertisedName)
	f.optFloat("price_min", params.PriceMin)
	f.optFloat("price_max", params.PriceMax)
	f.str("currency", params.Currency)
	f.crop(params.Crop)
	f.str("payment_method", params.PaymentMethod)
	f.str("environment", string(s.client.environment(params.Environment)))
	f.file("image", params.Image)
	r := &request{method: http.MethodPost, path: "/events/preview"}
	if err := r.setForm(f); err != nil {
		return nil, err
	}
	out := &EventPreview{}
	if err := s.client.do(ctx, r, out); err != nil {
		return nil, err
	}
	return out, nil
}

// PublishEventParams are the parameters of Events.Publish. Give one image
// source: Image, PreviewToken or SourceEventID.
type PublishEventParams struct {
	Title       string
	Description string
	VenueName   string
	Address     string
	City        string

	// CountryCode is ISO 3166-1 alpha-2, for example GB.
	CountryCode string
	Location    LatLng

	// StartsAt and EndsAt are ISO 8601, UTC.
	StartsAt string
	EndsAt   string

	// URL is your own booking page.
	URL            string
	AdvertisedName *string

	// PriceMin, PriceMax and Currency are the ticket price. Omit all three to
	// show no price; send 0 for both to mark the event as free.
	PriceMin      *float64
	PriceMax      *float64
	Currency      string
	Image         *File
	PreviewToken  string
	SourceEventID *int64
	Crop          *Crop

	// PaymentMethod defaults to PaymentMethodBalance, the only one Events take.
	PaymentMethod string

	// Environment is filled in from the key when empty.
	Environment Environment

	// IdempotencyKey is generated (a UUID v4) when empty, and reused on
	// every retry of the call.
	IdempotencyKey string

	// DryRun checks everything and answers with the cost, creating nothing.
	DryRun bool
}

// PublishedEvent is the response of Events.Publish. A dry run's
// listing_fee, amount_to_charge and balance_after are reachable with Field.
type PublishedEvent struct {
	Response
	ReturnCode    string           `json:"return_code"`
	EventID       int64            `json:"event_id"`
	Status        string           `json:"status"`
	PaymentURL    *string          `json:"payment_url"`
	AmountCharged float64          `json:"amount_charged"`
	Currency      string           `json:"currency"`
	Category      EventCategoryRef `json:"category"`
	DryRun        bool             `json:"dry_run"`

	// IdempotencyKey is the key that was sent.
	IdempotencyKey string `json:"-"`
}

// Publish publishes an Event, paid from the balance (POST /events).
func (s *EventsService) Publish(ctx context.Context, params *PublishEventParams) (*PublishedEvent, error) {
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
	payment := params.PaymentMethod
	if payment == "" {
		payment = PaymentMethodBalance
	}
	f := newForm()
	f.field("title", params.Title)
	f.field("description", params.Description)
	f.field("venue_name", params.VenueName)
	f.field("address", params.Address)
	f.field("city", params.City)
	f.field("country_code", params.CountryCode)
	f.location(params.Location)
	f.field("starts_at", params.StartsAt)
	f.field("ends_at", params.EndsAt)
	f.field("url", params.URL)
	f.optStr("advertised_name", params.AdvertisedName)
	f.optFloat("price_min", params.PriceMin)
	f.optFloat("price_max", params.PriceMax)
	f.str("currency", params.Currency)
	f.str("preview_token", params.PreviewToken)
	f.optInt64("source_event_id", params.SourceEventID)
	f.crop(params.Crop)
	f.field("payment_method", payment)
	f.str("environment", string(s.client.environment(params.Environment)))
	f.file("image", params.Image)
	r := &request{method: http.MethodPost, path: "/events", query: dryRunQuery(params.DryRun), idempotencyKey: key}
	if err := r.setForm(f); err != nil {
		return nil, err
	}
	out := &PublishedEvent{}
	if err := s.client.do(ctx, r, out); err != nil {
		return nil, err
	}
	out.IdempotencyKey = key
	return out, nil
}

// ListEventsParams are the parameters of Events.List. All are optional.
type ListEventsParams struct {
	// Environment is filled in from the key when empty.
	Environment Environment

	// Limit is the page size.
	Limit int

	// Cursor is the next_cursor of the previous page.
	Cursor string

	// EventStatus is upcoming, scheduled, rescheduled, cancelled or ended.
	EventStatus string

	// OwnerUserID lists only the Events of this user.
	OwnerUserID int64

	// CreatedAfter and CreatedBefore are ISO 8601, UTC.
	CreatedAfter  string
	CreatedBefore string
}

// EventList is one page of Events.
type EventList struct {
	Response
	Data       []Event    `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// List returns one page of Events (GET /events).
func (s *EventsService) List(ctx context.Context, params *ListEventsParams) (*EventList, error) {
	var p ListEventsParams
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
	setNonEmpty(q, "event_status", p.EventStatus)
	if p.OwnerUserID != 0 {
		q.Set("owner_user_id", strconv.FormatInt(p.OwnerUserID, 10))
	}
	setNonEmpty(q, "created_after", p.CreatedAfter)
	setNonEmpty(q, "created_before", p.CreatedBefore)
	out := &EventList{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/events", query: q}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListAutoPaging returns an iterator over every Event, following next_cursor
// until has_more is false.
func (s *EventsService) ListAutoPaging(ctx context.Context, params *ListEventsParams) *Iter[Event] {
	var p ListEventsParams
	if params != nil {
		p = *params
	}
	return newIter(ctx, p.Cursor, func(ctx context.Context, cursor string) ([]Event, Pagination, error) {
		page := p
		page.Cursor = cursor
		res, err := s.List(ctx, &page)
		if err != nil {
			return nil, Pagination{}, err
		}
		return res.Data, res.Pagination, nil
	})
}

// Get returns one Event (GET /events/{event_id}).
func (s *EventsService) Get(ctx context.Context, eventID int64) (*EventDetails, error) {
	out := &EventDetails{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/events/" + pathInt(eventID)}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// EditEventParams are the parameters of Events.Edit. Set only the fields to
// change. Dates cannot be edited: use Events.Reschedule.
type EditEventParams struct {
	Title       *string
	Description *string
	URL         *string

	// AdvertisedName set to "" goes back to the trading name.
	AdvertisedName *string
	VenueName      *string
	Address        *string
	City           *string
	CountryCode    *string
	Location       *LatLng

	// A price change must send PriceMin, PriceMax and Currency together.
	PriceMin *float64
	PriceMax *float64
	Currency *string

	// ClearPrice sends all three price fields blank, to show no price.
	ClearPrice    bool
	Image         *File
	PreviewToken  *string
	SourceEventID *int64

	// Crop on its own re-frames the existing image; with Image it is the area
	// of the new picture.
	Crop *Crop
}

// EditedEvent is the response of Events.Edit.
type EditedEvent struct {
	Response
	ReturnCode    string           `json:"return_code"`
	Message       string           `json:"message"`
	EventID       int64            `json:"event_id"`
	Status        string           `json:"status"`
	Category      EventCategoryRef `json:"category"`
	ChangedFields []string         `json:"changed_fields"`
	Image         string           `json:"image"`
	ImageLarge    string           `json:"image_large"`
	VenueChanged  bool             `json:"venue_changed"`
}

// Edit changes a published Event, free of charge (PATCH /events/{event_id}).
// It is always sent as a multipart POST with X-HTTP-Method-Override: PATCH,
// with or without an image.
func (s *EventsService) Edit(ctx context.Context, eventID int64, params *EditEventParams) (*EditedEvent, error) {
	if params == nil {
		return nil, errNilParams
	}
	f := newForm()
	f.optStr("title", params.Title)
	f.optStr("description", params.Description)
	f.optStr("url", params.URL)
	f.optStr("advertised_name", params.AdvertisedName)
	f.optStr("venue_name", params.VenueName)
	f.optStr("address", params.Address)
	f.optStr("city", params.City)
	f.optStr("country_code", params.CountryCode)
	if params.Location != nil {
		f.location(*params.Location)
	}
	if params.ClearPrice {
		f.field("price_min", "")
		f.field("price_max", "")
		f.field("currency", "")
	} else {
		f.optFloat("price_min", params.PriceMin)
		f.optFloat("price_max", params.PriceMax)
		f.optStr("currency", params.Currency)
	}
	f.optStr("preview_token", params.PreviewToken)
	f.optInt64("source_event_id", params.SourceEventID)
	f.crop(params.Crop)
	f.file("image", params.Image)
	// Always a multipart POST with the override header, with or without an
	// image: multipart fields are read only from a POST body.
	r := &request{method: http.MethodPost, path: "/events/" + pathInt(eventID), methodOverride: http.MethodPatch}
	if err := r.setForm(f); err != nil {
		return nil, err
	}
	out := &EditedEvent{}
	if err := s.client.do(ctx, r, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete deletes an Event (DELETE /events/{event_id}). params may be nil.
func (s *EventsService) Delete(ctx context.Context, eventID int64, params *DeleteParams) (*ActionResult, error) {
	out := &ActionResult{}
	r := &request{method: http.MethodDelete, path: "/events/" + pathInt(eventID), query: dryRunQuery(params != nil && params.DryRun)}
	if err := s.client.do(ctx, r, out); err != nil {
		return nil, err
	}
	return out, nil
}

// RescheduleEventParams are the parameters of Events.Reschedule.
type RescheduleEventParams struct {
	// StartsAt and EndsAt are ISO 8601, UTC.
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

// RescheduledEvent is the response of Events.Reschedule.
type RescheduledEvent struct {
	Response
	ReturnCode string `json:"return_code"`
	Message    string `json:"message"`
	EventID    int64  `json:"event_id"`
	Status     string `json:"status"`
	StartsAt   string `json:"starts_at"`
	EndsAt     string `json:"ends_at"`
}

// Reschedule moves an Event to new dates
// (POST /events/{event_id}/reschedule).
func (s *EventsService) Reschedule(ctx context.Context, eventID int64, params *RescheduleEventParams) (*RescheduledEvent, error) {
	if params == nil {
		return nil, errNilParams
	}
	r := &request{method: http.MethodPost, path: "/events/" + pathInt(eventID) + "/reschedule"}
	if err := r.setJSON(params); err != nil {
		return nil, err
	}
	out := &RescheduledEvent{}
	if err := s.client.do(ctx, r, out); err != nil {
		return nil, err
	}
	return out, nil
}

// CancelledEvent is the response of Events.Cancel.
type CancelledEvent struct {
	Response
	ReturnCode string `json:"return_code"`
	Message    string `json:"message"`
	EventID    int64  `json:"event_id"`
	Status     string `json:"status"`
	DryRun     bool   `json:"dry_run"`
}

// Cancel cancels an Event (POST /events/{event_id}/cancel). params may be nil.
func (s *EventsService) Cancel(ctx context.Context, eventID int64, params *DeleteParams) (*CancelledEvent, error) {
	out := &CancelledEvent{}
	r := &request{method: http.MethodPost, path: "/events/" + pathInt(eventID) + "/cancel", query: dryRunQuery(params != nil && params.DryRun)}
	if err := s.client.do(ctx, r, out); err != nil {
		return nil, err
	}
	return out, nil
}

// EventMetrics are an Event's figures. Clicks is exact; the others are bands.
type EventMetrics struct {
	Clicks          int64 `json:"clicks"`
	Attendees       Band  `json:"attendees"`
	InvitesSent     Band  `json:"invites_sent"`
	InvitesAccepted Band  `json:"invites_accepted"`
}

// EventAnalytics is the response of Events.Analytics.
type EventAnalytics struct {
	Response
	EventID  int64        `json:"event_id"`
	Interval string       `json:"interval"`
	StartsAt string       `json:"starts_at"`
	EndsAt   string       `json:"ends_at"`
	Status   string       `json:"status"`
	Metrics  EventMetrics `json:"metrics"`

	// AsOf is when the figures were refreshed, nil until the first member
	// interaction.
	AsOf *string `json:"as_of"`
}

// Analytics returns an Event's figures over an interval
// (GET /events/{event_id}/analytics).
func (s *EventsService) Analytics(ctx context.Context, eventID int64, params *AnalyticsParams) (*EventAnalytics, error) {
	if params == nil {
		return nil, errNilParams
	}
	q := url.Values{"interval": {params.Interval}}
	out := &EventAnalytics{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/events/" + pathInt(eventID) + "/analytics", query: q}, out); err != nil {
		return nil, err
	}
	return out, nil
}
