// Command testmode lists your categories and Offers with a Test key, then
// estimates a price and sends an Offer create as a dry run. It creates
// nothing.
//
//	AGILIS_API_KEY=pk_test_... go run ./examples/testmode
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	agilisads "github.com/agilisdating/agilis-ads-go"
)

func main() {
	client := agilisads.NewClient("") // reads AGILIS_API_KEY
	if client.Mode() != agilisads.ModeTest {
		log.Fatal("this example runs with a Test key (pk_test_...) only")
	}
	ctx := context.Background()

	categories, err := client.Categories.List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range categories.Categories {
		fmt.Printf("category %d: %s\n", c.CategoryID, c.CategoryName)
	}

	it := client.Offers.ListAutoPaging(ctx, &agilisads.ListOffersParams{Limit: 50})
	for it.Next() {
		o := it.Current()
		fmt.Printf("offer %d: %s (%s)\n", o.OfferID, o.Title, o.OfferStatus)
	}
	if err := it.Err(); err != nil {
		log.Fatal(err)
	}

	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	end := start.Add(48 * time.Hour)
	estimate, err := client.Offers.EstimatePrice(ctx, &agilisads.EstimateOfferPriceParams{
		StartDatetime: start.Format(time.RFC3339),
		EndDatetime:   end.Format(time.RFC3339),
		Location:      agilisads.LatLng{Latitude: 51.5074, Longitude: -0.1278},
		Radius:        10,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("estimated cost: %.2f %s for %d members\n", estimate.EstimatedCost, estimate.Currency, estimate.MatchingUsers)

	if len(categories.Categories) == 0 {
		return
	}
	res, err := client.Offers.Create(ctx, &agilisads.CreateOfferParams{
		CategoryID:              categories.Categories[0].CategoryID,
		Title:                   "20% off summer sale",
		Description:             "Discount valid through July",
		PushNotificationTitle:   "20% off this summer",
		PushNotificationMessage: "Show this at the till for 20% off everything.",
		URL:                     "https://www.example.com/offer",
		Image:                   agilisads.FileFromPath("testdata/sample-offer.png"),
		Location:                agilisads.LatLng{Latitude: 51.5074, Longitude: -0.1278},
		Radius:                  10,
		StartDatetime:           start.Format(time.RFC3339),
		ExpiryDatetime:          end.Format(time.RFC3339),
		PaymentMethod:           agilisads.PaymentMethodBalance,
		DryRun:                  true,
	})
	var invalid *agilisads.InvalidRequestError
	switch {
	case errors.As(err, &invalid):
		fmt.Printf("refused: %s (%s)\n", invalid.Message, invalid.ReturnCode)
	case err != nil:
		log.Fatal(err)
	default:
		fmt.Printf("dry run: price %s, balance after %s\n", res.Field("price"), res.Field("balance_after"))
	}
}
