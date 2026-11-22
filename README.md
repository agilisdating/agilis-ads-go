# Agilis Advertising API for Go

The official Go client for the [Agilis Advertising REST API](https://www.agilis.dating/api-doc): create and manage Offers, publish Events, and receive webhooks.

- Go 1.22 or later
- Standard library only

## Install

```sh
go get github.com/agilisdating/agilis-ads-go
```

```go
import agilisads "github.com/agilisdating/agilis-ads-go"
```

## Quick start in Test mode

Create a Test key in the Ad Portal (**Manage API Keys**, in Test mode). Test Offers and Events are never shown to members and nothing is charged.

```go
package main

import (
	"context"
	"fmt"
	"log"

	agilisads "github.com/agilisdating/agilis-ads-go"
)

func main() {
	client := agilisads.NewClient("") // reads AGILIS_API_KEY
	if client.Mode() != agilisads.ModeTest {
		log.Fatal("use a Test key")
	}

	categories, err := client.Categories.List(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range categories.Categories {
		fmt.Println(c.CategoryID, c.CategoryName)
	}
}
```

More in [`examples/`](examples): a Test-mode script and a webhook receiver.

## The client

```go
client := agilisads.NewClient(apiKey,
	agilisads.WithBaseURL("https://ads.agilis.dating"), // the default
	agilisads.WithTimeout(60*time.Second),              // per attempt; the default
	agilisads.WithMaxRetries(2),                        // the default
	agilisads.WithHTTPClient(&http.Client{}),           // any *http.Client
)
```

- **API key:** pass it, or pass `""` to read `AGILIS_API_KEY`. Keep keys in environment variables or a secrets manager, never in code.
- **Mode:** the key decides it: `pk_test_` is Test and `pk_live_` is Live. `client.Mode()` returns `agilisads.ModeTest` or `agilisads.ModeLive`, so you can refuse to run against Live. Operations that take an `Environment` fill it in from the key when you leave it empty.
- The key is sent in the `x-api-key` header only. It is never logged, never part of a URL and never in an error message.
- A `*Client` is safe for concurrent use.

## Resources and methods

Every method takes a `context.Context`, then any path parameters, then a parameters struct where the operation has parameters. Optional fields are pointers; use the helpers `agilisads.String`, `Int`, `Int64`, `Float64` and `Bool`.

| Resource | Method | HTTP |
|---|---|---|
| `client.Categories` | `List(ctx)` | `GET /categories` |
| `client.Interests` | `List(ctx)` | `GET /interests` |
| `client.Offers` | `EstimatePrice(ctx, *EstimateOfferPriceParams)` | `POST /offers/price-estimate` |
| | `Create(ctx, *CreateOfferParams)` | `POST /offers` |
| | `List(ctx, *ListOffersParams)` | `GET /offers` |
| | `ListAutoPaging(ctx, *ListOffersParams)` | every page of `GET /offers` |
| | `Pause(ctx, offerID)` | `POST /offers/{offer_id}/pause` |
| | `Resume(ctx, offerID)` | `POST /offers/{offer_id}/resume` |
| | `Analytics(ctx, offerID, *AnalyticsParams)` | `GET /offers/{offer_id}/analytics` |
| | `Delete(ctx, offerID, *DeleteParams)` | `DELETE /offers/{offer_id}` |
| `client.EventCategories` | `List(ctx)` | `GET /event-categories` |
| `client.EventCurrencies` | `List(ctx)` | `GET /event-currencies` |
| `client.EventConfig` | `Get(ctx)` | `GET /event-config` |
| `client.Events` | `Preview(ctx, *PreviewEventParams)` | `POST /events/preview` |
| | `Publish(ctx, *PublishEventParams)` | `POST /events` |
| | `List(ctx, *ListEventsParams)` | `GET /events` |
| | `ListAutoPaging(ctx, *ListEventsParams)` | every page of `GET /events` |
| | `Get(ctx, eventID)` | `GET /events/{event_id}` |
| | `Edit(ctx, eventID, *EditEventParams)` | `PATCH /events/{event_id}`, sent as `POST` with `X-HTTP-Method-Override: PATCH` |
| | `Delete(ctx, eventID, *DeleteParams)` | `DELETE /events/{event_id}` |
| | `Reschedule(ctx, eventID, *RescheduleEventParams)` | `POST /events/{event_id}/reschedule` |
| | `Cancel(ctx, eventID, *DeleteParams)` | `POST /events/{event_id}/cancel` |
| | `Analytics(ctx, eventID, *AnalyticsParams)` | `GET /events/{event_id}/analytics` |
| `client.WebhookEndpoints` | `List(ctx)` | `GET /webhook-endpoints` |
| | `Create(ctx, *CreateWebhookEndpointParams)` | `POST /webhook-endpoints` |
| | `Get(ctx, endpointID)` | `GET /webhook-endpoints/{endpoint_id}` |
| | `Update(ctx, endpointID, *UpdateWebhookEndpointParams)` | `PATCH /webhook-endpoints/{endpoint_id}` |
| | `Delete(ctx, endpointID)` | `DELETE /webhook-endpoints/{endpoint_id}` |
| | `RollSecret(ctx, endpointID, *RollWebhookSecretParams)` | `POST /webhook-endpoints/{endpoint_id}/roll-secret` |
| | `SendTest(ctx, endpointID, *SendTestWebhookParams)` | `POST /webhook-endpoints/{endpoint_id}/test` |
| `client.WebhookEvents` | `List(ctx, *ListWebhookEventsParams)` | `GET /webhook-events` |
| | `ListAutoPaging(ctx, *ListWebhookEventsParams)` | every page of `GET /webhook-events` |
| | `Get(ctx, webhookEventID)` | `GET /webhook-events/{webhook_event_id}` |
| | `Resend(ctx, webhookEventID, *ResendWebhookEventParams)` | `POST /webhook-events/{webhook_event_id}/resend` |
| package | `ConstructEvent(payload, header, secret, ...WebhookOption)` | verifies a received webhook |

The field names and rules are those of the [API reference](https://www.agilis.dating/api-doc).

### Creating an Offer

```go
res, err := client.Offers.Create(ctx, &agilisads.CreateOfferParams{
	CategoryID:              1,
	Title:                   "20% off summer sale",
	Description:             "Discount valid through July",
	PushNotificationTitle:   "20% off this summer",
	PushNotificationMessage: "Show this at the till for 20% off everything.",
	ButtonPreset:            agilisads.ButtonBookNow,
	URL:                     "https://www.example.com/offer",
	Image:                   agilisads.FileFromPath("offer.png"),
	Location:                agilisads.LatLng{Latitude: 51.5074, Longitude: -0.1278},
	Radius:                  10,
	Gender:                  []string{agilisads.GenderFemale},
	Interests:               []int64{3, 7},
	StartDatetime:           "2027-07-10T08:00:00Z",
	ExpiryDatetime:          "2027-07-12T20:30:00Z",
	PaymentMethod:           agilisads.PaymentMethodBalance,
	DryRun:                  true, // check the cost first; nothing is created
})
```

An image can be `FileFromPath(path)`, `FileFromBytes(name, data)` or `FileFromReader(name, r)`. The content type comes from the file name (`.jpg`, `.png`, `.gif`); set it with `WithContentType` if the name has none.

Array fields are sent as repeated `location[]`, `gender[]` and `interests[]` parts, and an Event crop as `crop[x]`, `crop[y]` and `crop[w]`.

### Responses

Each method returns a typed struct. Every response embeds `agilisads.Response`:

```go
res.StatusCode             // the HTTP status
res.Header.Get("X-RateLimit-Remaining")
res.RateLimitRemaining()   // the same, as an int (-1 when absent)
res.RawJSON                // the body as received
res.Field("balance_after") // the raw JSON of any top-level field
```

Fields the SDK does not know yet never break decoding: they stay in `RawJSON`. A dry run's figures (`price`, `balance_after`, `amount_to_charge` and so on) are read with `Field`.

## Pagination

`List` returns one page. `ListAutoPaging` follows `next_cursor` until `has_more` is false:

```go
it := client.Offers.ListAutoPaging(ctx, &agilisads.ListOffersParams{OfferStatus: "active"})
for it.Next() {
	offer := it.Current()
	fmt.Println(offer.OfferID, offer.Title)
}
if err := it.Err(); err != nil {
	log.Fatal(err)
}
```

## Idempotency and retries

- `Offers.Create` and `Events.Publish` send an `Idempotency-Key`: yours (`IdempotencyKey`) or a random UUID v4, generated once per call and reused on every retry. The response's `IdempotencyKey` is the key that was sent.
- `DryRun: true` on `Offers.Create` and `Events.Publish`, and `&agilisads.DeleteParams{DryRun: true}` on `Offers.Delete`, `Events.Delete` and `Events.Cancel`, send `dry_run=true`: nothing is created, changed or charged.
- **Retried** (up to `WithMaxRetries`, default 2): connection errors and timeouts; HTTP 429; and HTTP 500, 502, 503 and 504 for `GET` and `DELETE`, and for a `POST` that carries an Idempotency-Key.
- **Not retried:** other `POST` and `PATCH` requests on a 5xx, because the change may have happened.
- **The wait:** `Retry-After` when the API sends it; otherwise 0.5 s × 2^attempt with ±25% jitter, at most 8 s. A cancelled context stops the retries.

## Errors

Every API error unwraps to `*agilisads.AgilisError`, which carries `StatusCode`, `ReturnCode`, `Message`, `Body`, `Data` (the parsed body) and `Header`. Check for a specific kind with `errors.As`:

| Type | When |
|---|---|
| `*AuthenticationError` | 401 |
| `*PermissionError` | 403 |
| `*NotFoundError` | 404 |
| `*ConflictError` | 409 |
| `*InvalidRequestError` | 400, 422, and a refusal that arrives as HTTP 200 with a `return_code` starting with `E` |
| `*RateLimitError` | 429, with `RetryAfter` |
| `*APIError` | 5xx and anything else |
| `*ConnectionError` | no response (`Err` holds the cause) |
| `*SignatureVerificationError` | `ConstructEvent`, with `Reason` |

```go
_, err := client.Offers.Create(ctx, params)
var invalid *agilisads.InvalidRequestError
if errors.As(err, &invalid) && invalid.ReturnCode == "E3" {
	// the balance is too low: top up and retry with the same IdempotencyKey
}
```

Some refusals come as HTTP 200 with an `E` code (for example `E3`, balance too low): these are `*InvalidRequestError` with `StatusCode` 200. `S1` and `S2` are both success.

## Webhooks

`ConstructEvent` checks the `Agilis-Signature` header against the raw body with your endpoint's secret, in constant time, accepting any matching `v1` (there are two while a secret is being rolled) and a timestamp within 5 minutes. It returns the event, or a `*SignatureVerificationError`.

```go
http.HandleFunc("/agilis/webhooks", func(w http.ResponseWriter, r *http.Request) {
	payload, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	event, err := agilisads.ConstructEvent(payload, r.Header.Get(agilisads.SignatureHeader), secret)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent) // answer quickly, then do the work

	switch event.Type {
	case agilisads.EventTypeOfferStarted:
		offer, _ := event.Data.Offer()
		log.Println("offer started:", offer.OfferID)
	case agilisads.EventTypeEventPublished:
		ev, _ := event.Data.Event()
		log.Println("event published:", ev.EventID)
	}
})
```

- Pass the body exactly as received: do not decode and re-encode it first.
- `event.Data.Object` is the raw `data.object`; `event.Data.Offer()`, `event.Data.Event()` and `event.Data.DecodeObject(&v)` decode it, and `event.Data.Fields` holds every field of `data`, such as `amount_charged`.
- Options: `agilisads.WithTolerance(d)` (default 5 minutes) and `agilisads.WithNow(f)`. `VerifySignature` checks without decoding.
- Use `event.ID` to ignore repeats and `event.Created` to order events.

## Tests

```sh
go test -race ./...
```

The Test-mode run uses a Test key and creates nothing that lasts: it reads, and sends an Offer create and an Event publish as dry runs. It refuses to run with a Live key.

```sh
AGILIS_TEST_API_KEY=pk_test_... go test -tags integration -run Integration ./...
```

## Links

- [API reference](https://www.agilis.dating/api-doc) ([Markdown](https://www.agilis.dating/api-doc.md))
- [OpenAPI document](https://www.agilis.dating/openapi.json)
- [AsyncAPI document](https://www.agilis.dating/asyncapi.json) (webhooks)
- [agilis-ads-api](https://github.com/agilisdating/agilis-ads-api)

## Licence

MIT. See [LICENSE](LICENSE). To report a security problem, see [SECURITY.md](SECURITY.md).
