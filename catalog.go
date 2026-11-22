package agilisads

import (
	"context"
	"net/http"
)

// Category is an Offer category.
type Category struct {
	CategoryID   int64  `json:"category_id"`
	CategoryName string `json:"category_name"`
}

// CategoryList is the response of Categories.List.
type CategoryList struct {
	Response
	Categories []Category `json:"categories"`
}

// CategoriesService lists Offer categories.
type CategoriesService struct{ client *Client }

// List returns the Offer categories (GET /categories).
func (s *CategoriesService) List(ctx context.Context) (*CategoryList, error) {
	out := &CategoryList{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/categories"}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Interest is a member interest that an Offer can target.
type Interest struct {
	InterestID   int64  `json:"interest_id"`
	InterestName string `json:"interest_name"`
}

// InterestList is the response of Interests.List.
type InterestList struct {
	Response
	Interests []Interest `json:"interests"`
}

// InterestsService lists interests.
type InterestsService struct{ client *Client }

// List returns the interests Offers can target (GET /interests).
func (s *InterestsService) List(ctx context.Context) (*InterestList, error) {
	out := &InterestList{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/interests"}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// EventCategory is an Event category.
type EventCategory struct {
	CategoryID   int64  `json:"category_id"`
	Code         string `json:"code"`
	CategoryName string `json:"category_name"`
}

// EventCategoryList is the response of EventCategories.List.
type EventCategoryList struct {
	Response
	Categories []EventCategory `json:"categories"`
}

// EventCategoriesService lists Event categories.
type EventCategoriesService struct{ client *Client }

// List returns the Event categories (GET /event-categories).
func (s *EventCategoriesService) List(ctx context.Context) (*EventCategoryList, error) {
	out := &EventCategoryList{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/event-categories"}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// EventCurrencyList is the response of EventCurrencies.List.
type EventCurrencyList struct {
	Response
	Currencies []string `json:"currencies"`
}

// EventCurrenciesService lists the currencies an Event's ticket price can use.
type EventCurrenciesService struct{ client *Client }

// List returns the currencies an Event's ticket price can use
// (GET /event-currencies).
func (s *EventCurrenciesService) List(ctx context.Context) (*EventCurrencyList, error) {
	out := &EventCurrencyList{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/event-currencies"}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// EventImageLimits are the limits on an Event image.
type EventImageLimits struct {
	MaxMB     float64 `json:"max_mb"`
	MinWidth  int     `json:"min_width"`
	MinHeight int     `json:"min_height"`
}

// EventConfig is the response of EventConfig.Get: what publishing an Event
// costs, the balance of the key's mode and the image limits.
type EventConfig struct {
	Response

	// ListingFee, Currency, ExchangeRatePerGBP and ExchangeRateDate are nil
	// when the billing currency cannot be billed right now.
	ListingFee          *float64         `json:"listing_fee"`
	Currency            *string          `json:"currency"`
	ExchangeRatePerGBP  *float64         `json:"exchange_rate_per_gbp"`
	ExchangeRateDate    *string          `json:"exchange_rate_date"`
	Environment         string           `json:"environment"`
	Balance             float64          `json:"balance"`
	OrganisationBalance float64          `json:"organisation_balance"`
	LivePublishingOpen  bool             `json:"live_publishing_open"`
	Currencies          []string         `json:"currencies"`
	Image               EventImageLimits `json:"image"`
	MaxYearsAhead       int              `json:"max_years_ahead"`
}

// EventConfigService reads the Event configuration.
type EventConfigService struct{ client *Client }

// Get returns the Event configuration (GET /event-config).
func (s *EventConfigService) Get(ctx context.Context) (*EventConfig, error) {
	out := &EventConfig{}
	if err := s.client.do(ctx, &request{method: http.MethodGet, path: "/event-config"}, out); err != nil {
		return nil, err
	}
	return out, nil
}
