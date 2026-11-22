package agilisads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// Webhook event types.
const (
	EventTypeAll                   = "*"
	EventTypeOfferCreated          = "offer.created"
	EventTypeOfferPaymentSucceeded = "offer.payment_succeeded"
	EventTypeOfferPaymentFailed    = "offer.payment_failed"
	EventTypeOfferStarted          = "offer.started"
	EventTypeOfferPaused           = "offer.paused"
	EventTypeOfferResumed          = "offer.resumed"
	EventTypeOfferEnded            = "offer.ended"
	EventTypeOfferDeleted          = "offer.deleted"
	EventTypeOfferDisabled         = "offer.disabled"
	EventTypeOfferEnabled          = "offer.enabled"
	EventTypeEventPublished        = "event.published"
	EventTypeEventUpdated          = "event.updated"
	EventTypeEventRescheduled      = "event.rescheduled"
	EventTypeEventCancelled        = "event.cancelled"
	EventTypeEventDeleted          = "event.deleted"
	EventTypeEventEnded            = "event.ended"
	EventTypeBalanceToppedUp       = "balance.topped_up"
	EventTypeBalanceLow            = "balance.low"
	EventTypeInvoiceCreated        = "invoice.created"
	EventTypeTermsVersionIssued    = "terms.version_issued"
)

// WebhookEndpoint is a webhook endpoint.
type WebhookEndpoint struct {
	Response
	ReturnCode              string   `json:"return_code,omitempty"`
	ID                      string   `json:"id"`
	URL                     string   `json:"url"`
	Description             *string  `json:"description"`
	EnabledEvents           []string `json:"enabled_events"`
	Status                  string   `json:"status"`
	Livemode                bool     `json:"livemode"`
	Created                 string   `json:"created"`
	LastSuccessAt           *string  `json:"last_success_at"`
	FailingSince            *string  `json:"failing_since"`
	DisabledBy              *string  `json:"disabled_by"`
	DisabledReason          *string  `json:"disabled_reason"`
	PreviousSecretExpiresAt *string  `json:"previous_secret_expires_at"`

	// Secret is set only when the endpoint is created and when its secret is
	// rolled. Store it: it is not shown again.
	Secret string `json:"secret,omitempty"`
}

// WebhookEndpointList is the response of WebhookEndpoints.List.
type WebhookEndpointList struct {
	Response
	ReturnCode string            `json:"return_code"`
	Data       []WebhookEndpoint `json:"data"`
}

// WebhookEndpointsService manages webhook endpoints.
type WebhookEndpointsService struct{ client *Client }

// List returns the webhook endpoints of the key's mode
// (GET /webhook-endpoints).
func (s *WebhookEndpointsService) List(ctx context.Context) (*WebhookEndpointList, error) {
	out := &WebhookEndpointList{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/webhook-endpoints"}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateWebhookEndpointParams are the parameters of WebhookEndpoints.Create.
type CreateWebhookEndpointParams struct {
	URL           string   `json:"url"`
	EnabledEvents []string `json:"enabled_events"`
	Description   *string  `json:"description,omitempty"`
}

// Create adds a webhook endpoint (POST /webhook-endpoints). The response
// holds the signing secret, once.
func (s *WebhookEndpointsService) Create(ctx context.Context, params *CreateWebhookEndpointParams) (*WebhookEndpoint, error) {
	if params == nil {
		return nil, errNilParams
	}
	return s.send(ctx, http.MethodPost, "/webhook-endpoints", params)
}

// Get returns one webhook endpoint (GET /webhook-endpoints/{endpoint_id}).
func (s *WebhookEndpointsService) Get(ctx context.Context, endpointID string) (*WebhookEndpoint, error) {
	return s.send(ctx, http.MethodGet, "/webhook-endpoints/"+pathString(endpointID), nil)
}

// UpdateWebhookEndpointParams are the parameters of WebhookEndpoints.Update.
// Set only the fields to change.
type UpdateWebhookEndpointParams struct {
	URL           *string  `json:"url,omitempty"`
	EnabledEvents []string `json:"enabled_events,omitempty"`
	Description   *string  `json:"description,omitempty"`

	// Status is enabled or disabled.
	Status *string `json:"status,omitempty"`
}

// Update changes a webhook endpoint (PATCH /webhook-endpoints/{endpoint_id}).
func (s *WebhookEndpointsService) Update(ctx context.Context, endpointID string, params *UpdateWebhookEndpointParams) (*WebhookEndpoint, error) {
	if params == nil {
		return nil, errNilParams
	}
	return s.send(ctx, http.MethodPatch, "/webhook-endpoints/"+pathString(endpointID), params)
}

// DeletedWebhookEndpoint is the response of WebhookEndpoints.Delete.
type DeletedWebhookEndpoint struct {
	Response
	ReturnCode string `json:"return_code"`
	ID         string `json:"id"`
	Deleted    bool   `json:"deleted"`
}

// Delete deletes a webhook endpoint (DELETE /webhook-endpoints/{endpoint_id}).
func (s *WebhookEndpointsService) Delete(ctx context.Context, endpointID string) (*DeletedWebhookEndpoint, error) {
	out := &DeletedWebhookEndpoint{}
	if err := s.client.do(ctx, &request{method: http.MethodDelete, path: "/webhook-endpoints/" + pathString(endpointID)}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// RollWebhookSecretParams are the optional parameters of
// WebhookEndpoints.RollSecret.
type RollWebhookSecretParams struct {
	// ExpiresIn is how long the old secret keeps signing: now, 1h, 24h or 7d.
	ExpiresIn string `json:"expires_in,omitempty"`
}

// RollSecret replaces an endpoint's signing secret
// (POST /webhook-endpoints/{endpoint_id}/roll-secret). The new secret is in
// the response, once. params may be nil.
func (s *WebhookEndpointsService) RollSecret(ctx context.Context, endpointID string, params *RollWebhookSecretParams) (*WebhookEndpoint, error) {
	if params == nil {
		params = &RollWebhookSecretParams{}
	}
	return s.send(ctx, http.MethodPost, "/webhook-endpoints/"+pathString(endpointID)+"/roll-secret", params)
}

// SendTestWebhookParams are the optional parameters of
// WebhookEndpoints.SendTest.
type SendTestWebhookParams struct {
	// Type is the event type to send.
	Type string `json:"type,omitempty"`
}

// WebhookDelivery is the delivery of a webhook event to one endpoint.
type WebhookDelivery struct {
	EndpointID     string  `json:"endpoint_id"`
	Status         string  `json:"status"`
	Attempts       int     `json:"attempts"`
	LastHTTPStatus *int    `json:"last_http_status"`
	LastError      *string `json:"last_error"`
	NextAttemptAt  *string `json:"next_attempt_at"`
	DeliveredAt    *string `json:"delivered_at"`
}

// TestWebhookResult is the response of WebhookEndpoints.SendTest.
type TestWebhookResult struct {
	Response
	ReturnCode     string           `json:"return_code"`
	WebhookEventID string           `json:"webhook_event_id"`
	Type           string           `json:"type"`
	Test           bool             `json:"test"`
	Delivery       *WebhookDelivery `json:"delivery"`
}

// SendTest sends a test event to an endpoint
// (POST /webhook-endpoints/{endpoint_id}/test). params may be nil.
func (s *WebhookEndpointsService) SendTest(ctx context.Context, endpointID string, params *SendTestWebhookParams) (*TestWebhookResult, error) {
	if params == nil {
		params = &SendTestWebhookParams{}
	}
	r := &request{method: http.MethodPost, path: "/webhook-endpoints/" + pathString(endpointID) + "/test"}
	if err := r.setJSON(params); err != nil {
		return nil, err
	}
	out := &TestWebhookResult{}
	if err := s.client.do(ctx, r, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *WebhookEndpointsService) send(ctx context.Context, method, path string, body any) (*WebhookEndpoint, error) {
	r := &request{method: method, path: path}
	if body != nil {
		if err := r.setJSON(body); err != nil {
			return nil, err
		}
	}
	out := &WebhookEndpoint{}
	if err := s.client.do(ctx, r, out); err != nil {
		return nil, err
	}
	return out, nil
}

// WebhookEventRequest says where the change behind a webhook event came from.
type WebhookEventRequest struct {
	// Source is api, mcp, ad_portal, agilis or system.
	Source         string  `json:"source"`
	IdempotencyKey *string `json:"idempotency_key"`
}

// WebhookEventData is the data of a webhook event: the object (the Offer,
// Event or invoice, as the API returns it elsewhere) and any extra facts.
type WebhookEventData struct {
	// Object is the raw JSON of data.object. Decode it with Offer, Event or
	// DecodeObject.
	Object json.RawMessage

	// Fields holds every field of data, including object, as raw JSON. Extra
	// facts such as amount_charged are here.
	Fields map[string]json.RawMessage
}

// UnmarshalJSON keeps every field of data.
func (d *WebhookEventData) UnmarshalJSON(b []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	d.Fields = m
	d.Object = m["object"]
	return nil
}

// MarshalJSON writes the data back as received.
func (d WebhookEventData) MarshalJSON() ([]byte, error) {
	if d.Fields == nil {
		return []byte("null"), nil
	}
	return json.Marshal(d.Fields)
}

// DecodeObject decodes data.object into v.
func (d *WebhookEventData) DecodeObject(v any) error {
	if len(d.Object) == 0 {
		return &AgilisError{Message: "the webhook event has no data.object"}
	}
	return json.Unmarshal(d.Object, v)
}

// Offer decodes data.object as an Offer, for offer.* events.
func (d *WebhookEventData) Offer() (*Offer, error) {
	o := &Offer{}
	if err := d.DecodeObject(o); err != nil {
		return nil, err
	}
	return o, nil
}

// Event decodes data.object as an Event, for event.* events.
func (d *WebhookEventData) Event() (*Event, error) {
	e := &Event{}
	if err := d.DecodeObject(e); err != nil {
		return nil, err
	}
	return e, nil
}

// WebhookEvent is a webhook event: what ConstructEvent returns for a received
// message, and what WebhookEvents.Get and List return.
type WebhookEvent struct {
	Response
	ReturnCode string `json:"return_code,omitempty"`

	// ID is the event id (whevt_...). Use it to ignore repeats.
	ID     string `json:"id"`
	Object string `json:"object,omitempty"`

	// Type is the event type, one of the EventType constants.
	Type string `json:"type"`

	// Created is when the event happened, in Unix seconds.
	Created  int64               `json:"created"`
	Livemode bool                `json:"livemode"`
	Version  int                 `json:"version,omitempty"`
	Test     bool                `json:"test"`
	Data     WebhookEventData    `json:"data"`
	Request  WebhookEventRequest `json:"request"`

	// Deliveries is set by WebhookEvents.Get and List.
	Deliveries []WebhookDelivery `json:"deliveries,omitempty"`
}

// ListWebhookEventsParams are the parameters of WebhookEvents.List. All are
// optional.
type ListWebhookEventsParams struct {
	// Type lists only events of this type.
	Type string

	// DeliverySuccess false lists only events not yet delivered to every
	// endpoint; true only events delivered everywhere.
	DeliverySuccess *bool

	// Limit is the page size, 1 to 100.
	Limit int

	// Cursor is the next_cursor of the previous page.
	Cursor string
}

// WebhookEventList is one page of webhook events.
type WebhookEventList struct {
	Response
	ReturnCode string         `json:"return_code"`
	Data       []WebhookEvent `json:"data"`
	Pagination Pagination     `json:"pagination"`
}

// WebhookEventsService lists and resends webhook events.
type WebhookEventsService struct{ client *Client }

// List returns one page of the webhook events of the last 30 days, newest
// first (GET /webhook-events).
func (s *WebhookEventsService) List(ctx context.Context, params *ListWebhookEventsParams) (*WebhookEventList, error) {
	var p ListWebhookEventsParams
	if params != nil {
		p = *params
	}
	q := url.Values{}
	setNonEmpty(q, "type", p.Type)
	if p.DeliverySuccess != nil {
		q.Set("delivery_success", strconv.FormatBool(*p.DeliverySuccess))
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	setNonEmpty(q, "cursor", p.Cursor)
	out := &WebhookEventList{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/webhook-events", query: q}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListAutoPaging returns an iterator over every webhook event, following
// next_cursor until has_more is false.
func (s *WebhookEventsService) ListAutoPaging(ctx context.Context, params *ListWebhookEventsParams) *Iter[WebhookEvent] {
	var p ListWebhookEventsParams
	if params != nil {
		p = *params
	}
	return newIter(ctx, p.Cursor, func(ctx context.Context, cursor string) ([]WebhookEvent, Pagination, error) {
		page := p
		page.Cursor = cursor
		res, err := s.List(ctx, &page)
		if err != nil {
			return nil, Pagination{}, err
		}
		return res.Data, res.Pagination, nil
	})
}

// Get returns one webhook event with its deliveries
// (GET /webhook-events/{webhook_event_id}).
func (s *WebhookEventsService) Get(ctx context.Context, webhookEventID string) (*WebhookEvent, error) {
	out := &WebhookEvent{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/webhook-events/" + pathString(webhookEventID)}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ResendWebhookEventParams are the optional parameters of
// WebhookEvents.Resend.
type ResendWebhookEventParams struct {
	// EndpointID sends to this endpoint only. Empty sends to every endpoint
	// the event went to.
	EndpointID string `json:"endpoint_id,omitempty"`
}

// ResentWebhookEvent is the response of WebhookEvents.Resend.
type ResentWebhookEvent struct {
	Response
	ReturnCode     string            `json:"return_code"`
	WebhookEventID string            `json:"webhook_event_id"`
	Deliveries     []WebhookDelivery `json:"deliveries"`
}

// Resend sends a webhook event again now
// (POST /webhook-events/{webhook_event_id}/resend). params may be nil.
func (s *WebhookEventsService) Resend(ctx context.Context, webhookEventID string, params *ResendWebhookEventParams) (*ResentWebhookEvent, error) {
	if params == nil {
		params = &ResendWebhookEventParams{}
	}
	r := &request{method: http.MethodPost, path: "/webhook-events/" + pathString(webhookEventID) + "/resend"}
	if err := r.setJSON(params); err != nil {
		return nil, err
	}
	out := &ResentWebhookEvent{}
	if err := s.client.do(ctx, r, out); err != nil {
		return nil, err
	}
	return out, nil
}
