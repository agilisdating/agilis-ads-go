//go:build integration

// The Test-mode run. It needs a Test key in AGILIS_TEST_API_KEY and is
// skipped without one:
//
//	AGILIS_TEST_API_KEY=pk_test_... go test -tags integration -run Integration ./...
//
// It only reads, and sends an Offer create and an Event publish as dry runs:
// it creates nothing that lasts.
package agilisads_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	agilisads "github.com/agilisdating/agilis-ads-go"
)

func integrationClient(t *testing.T) *agilisads.Client {
	t.Helper()
	key := strings.TrimSpace(os.Getenv("AGILIS_TEST_API_KEY"))
	if key == "" {
		t.Skip("AGILIS_TEST_API_KEY is not set")
	}
	if strings.HasPrefix(key, "pk_live_") {
		t.Fatal("refusing to run with a Live key")
	}
	c := agilisads.NewClient(key)
	if c.Mode() != agilisads.ModeTest {
		t.Fatal("AGILIS_TEST_API_KEY is not a Test key")
	}
	return c
}

func isoIn(d time.Duration) string {
	return time.Now().UTC().Add(d).Truncate(time.Second).Format("2006-01-02T15:04:05Z")
}

// allowRefusal passes a refusal that depends on the account rather than the
// SDK (a Test balance too low, or a key without the webhook role).
func allowRefusal(t *testing.T, what string, err error, codes ...string) bool {
	t.Helper()
	var ae *agilisads.AgilisError
	if !errors.As(err, &ae) {
		return false
	}
	for _, c := range codes {
		if ae.ReturnCode == c {
			t.Logf("%s refused for this account: %v", what, err)
			return true
		}
	}
	return false
}

func TestIntegrationReads(t *testing.T) {
	c := integrationClient(t)
	ctx := context.Background()

	cats, err := c.Categories.List(ctx)
	if err != nil {
		t.Fatalf("categories: %v", err)
	}
	if len(cats.Categories) == 0 {
		t.Error("no categories")
	}
	if _, err := c.Interests.List(ctx); err != nil {
		t.Fatalf("interests: %v", err)
	}
	if _, err := c.EventCategories.List(ctx); err != nil {
		t.Fatalf("event categories: %v", err)
	}
	if _, err := c.EventCurrencies.List(ctx); err != nil {
		t.Fatalf("event currencies: %v", err)
	}
	cfg, err := c.EventConfig.Get(ctx)
	if err != nil {
		t.Fatalf("event config: %v", err)
	}
	if cfg.Environment != "TEST" {
		t.Errorf("event config environment = %q", cfg.Environment)
	}
	offers, err := c.Offers.List(ctx, &agilisads.ListOffersParams{Limit: 5})
	if err != nil {
		t.Fatalf("offers: %v", err)
	}
	t.Logf("offers page: %d, has more: %v, remaining this minute: %d", len(offers.Data), offers.Pagination.HasMore, offers.RateLimitRemaining())
	if _, err := c.Events.List(ctx, &agilisads.ListEventsParams{Limit: 5}); err != nil {
		t.Fatalf("events: %v", err)
	}
	est, err := c.Offers.EstimatePrice(ctx, &agilisads.EstimateOfferPriceParams{
		StartDatetime: isoIn(time.Hour),
		EndDatetime:   isoIn(49 * time.Hour),
		Location:      agilisads.LatLng{Latitude: 51.5074, Longitude: -0.1278},
		Radius:        10,
	})
	if err != nil {
		t.Fatalf("price estimate: %v", err)
	}
	t.Logf("estimate: %v %s", est.EstimatedCost, est.Currency)
	if _, err := c.WebhookEndpoints.List(ctx); err != nil && !allowRefusal(t, "webhook endpoints", err, "E2") {
		t.Fatalf("webhook endpoints: %v", err)
	}
	if _, err := c.WebhookEvents.List(ctx, &agilisads.ListWebhookEventsParams{Limit: 5}); err != nil && !allowRefusal(t, "webhook events", err, "E2") {
		t.Fatalf("webhook events: %v", err)
	}
}

func TestIntegrationDryRuns(t *testing.T) {
	c := integrationClient(t)
	ctx := context.Background()

	cats, err := c.Categories.List(ctx)
	if err != nil || len(cats.Categories) == 0 {
		t.Fatalf("categories: %v", err)
	}
	offer, err := c.Offers.Create(ctx, &agilisads.CreateOfferParams{
		CategoryID:              cats.Categories[0].CategoryID,
		Title:                   "SDK dry run",
		Description:             "A dry run from the Go SDK's tests. Nothing is created.",
		PushNotificationTitle:   "SDK dry run",
		PushNotificationMessage: "A dry run from the Go SDK's tests.",
		URL:                     "https://www.agilis.dating",
		Image:                   agilisads.FileFromPath("testdata/sample-offer.png"),
		Location:                agilisads.LatLng{Latitude: 51.5074, Longitude: -0.1278},
		Radius:                  10,
		StartDatetime:           isoIn(time.Hour),
		ExpiryDatetime:          isoIn(49 * time.Hour),
		PaymentMethod:           agilisads.PaymentMethodBalance,
		DryRun:                  true,
	})
	switch {
	case err == nil:
		if !offer.DryRun {
			t.Error("the Offer response is not marked dry_run")
		}
		t.Logf("offer dry run: price %s", offer.Field("price"))
	case allowRefusal(t, "offer dry run", err, "E3"):
	default:
		t.Fatalf("offer dry run: %v", err)
	}

	// Events need an image of at least 1280 × 720.
	img := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	for y := 0; y < 720; y++ {
		for x := 0; x < 1280; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: uint8(x % 256), B: uint8(y % 256), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	ev, err := c.Events.Publish(ctx, &agilisads.PublishEventParams{
		Title:       "SDK dry run",
		Description: "A dry run from the Go SDK's tests. Nothing is published.",
		VenueName:   "Test venue",
		Address:     "1 Test Street",
		City:        "London",
		CountryCode: "GB",
		Location:    agilisads.LatLng{Latitude: 51.5133, Longitude: -0.13},
		StartsAt:    isoIn(30 * 24 * time.Hour),
		EndsAt:      isoIn(30*24*time.Hour + 2*time.Hour),
		URL:         "https://www.agilis.dating",
		Image:       agilisads.FileFromBytes("event.png", buf.Bytes()),
		DryRun:      true,
	})
	switch {
	case err == nil:
		if !ev.DryRun {
			t.Error("the Event response is not marked dry_run")
		}
		t.Logf("event dry run: amount to charge %s", ev.Field("amount_to_charge"))
	case allowRefusal(t, "event dry run", err, "E3"):
	default:
		t.Fatalf("event dry run: %v", err)
	}
}
