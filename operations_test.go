package agilisads

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestHeaders(t *testing.T) {
	f := newFakeAPI(t, reply{Body: `{"categories":[]}`})
	c, _ := newTestClient(t, f)
	if _, err := c.Categories.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	rec := f.last()
	wantEqual(t, "x-api-key", rec.Header.Get("x-api-key"), testKey)
	wantEqual(t, "Accept", rec.Header.Get("Accept"), "application/json")
	ua := rec.Header.Get("User-Agent")
	if !strings.HasPrefix(ua, "agilis-ads-go/"+Version+" (go") {
		t.Errorf("User-Agent = %q", ua)
	}
	if strings.Contains(rec.Query, testKey) || strings.Contains(rec.Path, testKey) {
		t.Error("the API key is in the URL")
	}
}

func TestSimpleGets(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		path string
		body string
		call func(c *Client) (any, error)
		get  func(v any) string
	}{
		{"categories.list", "/categories", `{"categories":[{"category_id":1,"category_name":"Retail"}]}`,
			func(c *Client) (any, error) { return c.Categories.List(ctx) },
			func(v any) string { return v.(*CategoryList).Categories[0].CategoryName }},
		{"interests.list", "/interests", `{"interests":[{"interest_id":3,"interest_name":"Cooking"}]}`,
			func(c *Client) (any, error) { return c.Interests.List(ctx) },
			func(v any) string { return v.(*InterestList).Interests[0].InterestName }},
		{"eventCategories.list", "/event-categories", `{"categories":[{"category_id":9,"code":"classes","category_name":"Classes & workshops"}]}`,
			func(c *Client) (any, error) { return c.EventCategories.List(ctx) },
			func(v any) string { return v.(*EventCategoryList).Categories[0].Code }},
		{"eventCurrencies.list", "/event-currencies", `{"currencies":["EUR","GBP"]}`,
			func(c *Client) (any, error) { return c.EventCurrencies.List(ctx) },
			func(v any) string { return strings.Join(v.(*EventCurrencyList).Currencies, ",") }},
		{"eventConfig.get", "/event-config", `{"listing_fee":13.41,"currency":"USD","exchange_rate_per_gbp":1.3412,"exchange_rate_date":"2027-09-29","environment":"TEST","balance":42.5,"organisation_balance":42.5,"live_publishing_open":true,"currencies":["USD"],"image":{"max_mb":10,"min_width":1280,"min_height":720},"max_years_ahead":5}`,
			func(c *Client) (any, error) { return c.EventConfig.Get(ctx) },
			func(v any) string {
				cfg := v.(*EventConfig)
				return cfg.Environment + " " + formatFloat(*cfg.ListingFee) + " " + formatFloat(*cfg.ExchangeRatePerGBP)
			}},
		{"webhookEndpoints.list", "/webhook-endpoints", `{"return_code":"S1","data":[{"id":"we_1","url":"https://hooks.example.com","enabled_events":["*"],"status":"enabled","livemode":false,"created":"2027-09-20T10:00:00Z"}]}`,
			func(c *Client) (any, error) { return c.WebhookEndpoints.List(ctx) },
			func(v any) string { return v.(*WebhookEndpointList).Data[0].ID }},
		{"webhookEndpoints.get", "/webhook-endpoints/we_1", `{"return_code":"S1","id":"we_1","url":"https://hooks.example.com","description":"Orders","enabled_events":["*"],"status":"enabled"}`,
			func(c *Client) (any, error) { return c.WebhookEndpoints.Get(ctx, "we_1") },
			func(v any) string { return *v.(*WebhookEndpoint).Description }},
		{"webhookEvents.get", "/webhook-events/whevt_1", `{"return_code":"S1","id":"whevt_1","type":"offer.started","created":1821341702,"data":{"object":{"offer_id":45}},"request":{"source":"system","idempotency_key":null},"deliveries":[{"endpoint_id":"we_1","status":"succeeded","attempts":1,"last_http_status":200}]}`,
			func(c *Client) (any, error) { return c.WebhookEvents.Get(ctx, "whevt_1") },
			func(v any) string {
				ev := v.(*WebhookEvent)
				o, err := ev.Data.Offer()
				if err != nil {
					return err.Error()
				}
				return ev.Type + " " + pathInt(o.OfferID) + " " + ev.Deliveries[0].Status
			}},
		{"events.get", "/events/3104", `{"event_id":3104,"title":"Salsa","category":{"category_id":9,"category_name":"Classes"},"attendees":"10-25","can_manage":true,"listing_price":10,"image_master":{"width":2400,"height":1800,"crop":{"x":0,"y":225,"w":2400,"h":1350},"url":null},"history":[{"action":"published","channel":"api","details":{"environment":1},"at":"2027-09-20T10:00:00Z"}]}`,
			func(c *Client) (any, error) { return c.Events.Get(ctx, 3104) },
			func(v any) string {
				e := v.(*EventDetails)
				return e.Title + " " + string(e.Attendees) + " " + e.History[0].Action + " " + pathInt(int64(e.ImageMaster.Crop.H))
			}},
	}
	want := map[string]string{
		"categories.list":       "Retail",
		"interests.list":        "Cooking",
		"eventCategories.list":  "classes",
		"eventCurrencies.list":  "EUR,GBP",
		"eventConfig.get":       "TEST 13.41 1.3412",
		"webhookEndpoints.list": "we_1",
		"webhookEndpoints.get":  "Orders",
		"webhookEvents.get":     "offer.started 45 succeeded",
		"events.get":            "Salsa 10-25 published 1350",
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAPI(t, reply{Body: tc.body})
			c, _ := newTestClient(t, f)
			v, err := tc.call(c)
			if err != nil {
				t.Fatal(err)
			}
			rec := f.last()
			wantEqual(t, "method", rec.Method, "GET")
			wantEqual(t, "path", rec.Path, tc.path)
			wantEqual(t, "query", rec.Query, "")
			wantEqual(t, "decoded", tc.get(v), want[tc.name])
		})
	}
}

func TestOffersEstimatePrice(t *testing.T) {
	f := newFakeAPI(t, reply{Body: `{"return_code":"S1","price_estimate":12,"matching_users":340,"ad_units":4,"ad_unit_price":2.5,"estimated_cost":10.5,"currency":"GBP","exchange_rate_per_gbp":1,"exchange_rate_date":null}`})
	c, _ := newTestClient(t, f)
	res, err := c.Offers.EstimatePrice(context.Background(), &EstimateOfferPriceParams{
		StartDatetime: "2027-07-10T08:00:00Z",
		EndDatetime:   "2027-07-12T20:30:00Z",
		Location:      LatLng{Latitude: 51.5074, Longitude: -0.1278},
		Radius:        10,
		Gender:        []string{GenderMale, GenderFemale},
		Interests:     []int64{3, 7},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "estimated_cost", res.EstimatedCost, 10.5)
	wantEqual(t, "matching_users", res.MatchingUsers, int64(340))
	rec := f.last()
	wantEqual(t, "method", rec.Method, "POST")
	wantEqual(t, "path", rec.Path, "/offers/price-estimate")
	body := decodeJSONBody(t, rec)
	wantEqual(t, "environment", body["environment"], any("TEST"))
	wantEqual(t, "radius", body["radius"], any(float64(10)))
	loc, _ := body["location"].([]any)
	if len(loc) != 2 || loc[0] != 51.5074 || loc[1] != -0.1278 {
		t.Errorf("location = %v", body["location"])
	}
	wantEqual(t, "gender", len(body["gender"].([]any)), 2)
	wantEqual(t, "interests", len(body["interests"].([]any)), 2)
}

func TestOffersCreate(t *testing.T) {
	f := newFakeAPI(t, reply{Body: `{"return_code":"S1","offer_id":45,"status":"Awaiting Payment","payment_url":"https://checkout.stripe.com/c/pay/x","currency":"GBP"}`})
	c, _ := newTestClient(t, f)
	png := []byte("\x89PNG fake image")
	res, err := c.Offers.Create(context.Background(), &CreateOfferParams{
		CategoryID:              1,
		Title:                   "20% off summer sale",
		Description:             "Discount valid through July",
		TermsAndConditions:      String("Valid online only."),
		PushNotificationTitle:   "20% off this summer",
		PushNotificationMessage: "Show this at the till for 20% off.",
		ButtonPreset:            ButtonCustom,
		ButtonText:              String("Show this"),
		URL:                     "https://advertiseroffer.com",
		Image:                   FileFromBytes("offer.png", png),
		Location:                LatLng{Latitude: 51.5074, Longitude: -0.1278},
		Radius:                  10,
		Gender:                  []string{GenderMale, GenderFemale},
		Interests:               []int64{3, 7},
		StartDatetime:           "2027-07-10T08:00:00Z",
		ExpiryDatetime:          "2027-07-12T20:30:00Z",
		PaymentMethod:           PaymentMethodLink,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "offer_id", res.OfferID, int64(45))
	wantEqual(t, "payment_url", *res.PaymentURL, "https://checkout.stripe.com/c/pay/x")
	rec := f.last()
	wantEqual(t, "method", rec.Method, "POST")
	wantEqual(t, "path", rec.Path, "/offers")
	wantEqual(t, "query", rec.Query, "")
	key := rec.Header.Get("Idempotency-Key")
	if !uuidV4.MatchString(key) {
		t.Errorf("Idempotency-Key = %q, want a UUID v4", key)
	}
	wantEqual(t, "returned key", res.IdempotencyKey, key)
	values, files := parseMultipart(t, rec)
	wantValues(t, values, "category_id", "1")
	wantValues(t, values, "title", "20% off summer sale")
	wantValues(t, values, "terms_and_conditions", "Valid online only.")
	wantValues(t, values, "button_preset", "custom")
	wantValues(t, values, "button_text", "Show this")
	wantValues(t, values, "location[]", "51.5074", "-0.1278")
	wantValues(t, values, "radius", "10")
	wantValues(t, values, "gender[]", "Male", "Female")
	wantValues(t, values, "interests[]", "3", "7")
	wantValues(t, values, "start_datetime", "2027-07-10T08:00:00Z")
	wantValues(t, values, "expiry_datetime", "2027-07-12T20:30:00Z")
	wantValues(t, values, "payment_method", "E")
	wantValues(t, values, "environment", "TEST")
	if _, ok := values["advertised_name"]; ok {
		t.Error("advertised_name was sent though it was not set")
	}
	img := files["image"]
	wantEqual(t, "image name", img.FileName, "offer.png")
	wantEqual(t, "image type", img.ContentType, "image/png")
	wantEqual(t, "image data", string(img.Data), string(png))
}

func TestOffersCreateDryRunAndOwnKey(t *testing.T) {
	f := newFakeAPI(t, reply{Body: `{"return_code":"S1","dry_run":true,"price":10,"balance_after":32.5}`})
	c, _ := newTestClient(t, f)
	res, err := c.Offers.Create(context.Background(), &CreateOfferParams{
		Title:          "x",
		Image:          FileFromPath("testdata/sample-offer.png"),
		IdempotencyKey: "my-own-key",
		Environment:    EnvironmentTest,
		DryRun:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || string(res.Field("balance_after")) != "32.5" {
		t.Errorf("dry run = %v, balance_after = %s", res.DryRun, res.Field("balance_after"))
	}
	rec := f.last()
	wantEqual(t, "query", rec.Query, "dry_run=true")
	wantEqual(t, "Idempotency-Key", rec.Header.Get("Idempotency-Key"), "my-own-key")
	_, files := parseMultipart(t, rec)
	wantEqual(t, "image name", files["image"].FileName, "sample-offer.png")
	if len(files["image"].Data) < 1000 {
		t.Errorf("image is %d bytes", len(files["image"].Data))
	}
}

func TestOffersList(t *testing.T) {
	f := newFakeAPI(t, reply{Body: `{"data":[{"offer_id":45,"title":"Sale","advertised_name":null,"latitude":51.5,"longitude":-0.12,"radius":10,"gender":["Male"],"interests":[{"interest_id":3,"interest_name":"Cooking"}],"offer_status":"Active","status":"enabled","brand_new_field":{"a":1}}],"pagination":{"limit":20,"count":1,"has_more":false,"next_cursor":null}}`})
	c, _ := newTestClient(t, f)
	res, err := c.Offers.List(context.Background(), &ListOffersParams{Limit: 20, Cursor: "abc", OfferStatus: "active", CreatedAfter: "2027-01-01T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	rec := f.last()
	wantEqual(t, "path", rec.Path, "/offers")
	q, _ := url.ParseQuery(rec.Query)
	wantEqual(t, "environment", q.Get("environment"), "TEST")
	wantEqual(t, "limit", q.Get("limit"), "20")
	wantEqual(t, "cursor", q.Get("cursor"), "abc")
	wantEqual(t, "offer_status", q.Get("offer_status"), "active")
	wantEqual(t, "created_after", q.Get("created_after"), "2027-01-01T00:00:00Z")
	wantEqual(t, "offer", res.Data[0].Interests[0].InterestName, "Cooking")
	if res.Data[0].AdvertisedName != nil {
		t.Error("advertised_name should be nil")
	}
	if !strings.Contains(string(res.RawJSON), "brand_new_field") {
		t.Error("unknown fields were not kept")
	}
}

func TestOfferActions(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name, method, path, query string
		call                      func(c *Client) error
	}{
		{"pause", "POST", "/offers/45/pause", "", func(c *Client) error { _, err := c.Offers.Pause(ctx, 45); return err }},
		{"resume", "POST", "/offers/45/resume", "", func(c *Client) error { _, err := c.Offers.Resume(ctx, 45); return err }},
		{"analytics", "GET", "/offers/45/analytics", "interval=7d", func(c *Client) error {
			_, err := c.Offers.Analytics(ctx, 45, &AnalyticsParams{Interval: Interval7Days})
			return err
		}},
		{"delete", "DELETE", "/offers/45", "", func(c *Client) error { _, err := c.Offers.Delete(ctx, 45, nil); return err }},
		{"delete dry run", "DELETE", "/offers/45", "dry_run=true", func(c *Client) error {
			_, err := c.Offers.Delete(ctx, 45, &DeleteParams{DryRun: true})
			return err
		}},
		{"events.delete", "DELETE", "/events/3104", "dry_run=true", func(c *Client) error {
			_, err := c.Events.Delete(ctx, 3104, &DeleteParams{DryRun: true})
			return err
		}},
		{"events.cancel", "POST", "/events/3104/cancel", "dry_run=true", func(c *Client) error {
			_, err := c.Events.Cancel(ctx, 3104, &DeleteParams{DryRun: true})
			return err
		}},
		{"events.cancel real", "POST", "/events/3104/cancel", "", func(c *Client) error { _, err := c.Events.Cancel(ctx, 3104, nil); return err }},
		{"events.analytics", "GET", "/events/3104/analytics", "interval=all", func(c *Client) error {
			_, err := c.Events.Analytics(ctx, 3104, &AnalyticsParams{Interval: IntervalAll})
			return err
		}},
		{"webhookEndpoints.delete", "DELETE", "/webhook-endpoints/we_1", "", func(c *Client) error {
			_, err := c.WebhookEndpoints.Delete(ctx, "we_1")
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAPI(t, reply{Body: `{"return_code":"S1","message":"done"}`})
			c, _ := newTestClient(t, f)
			if err := tc.call(c); err != nil {
				t.Fatal(err)
			}
			rec := f.last()
			wantEqual(t, "method", rec.Method, tc.method)
			wantEqual(t, "path", rec.Path, tc.path)
			wantEqual(t, "query", rec.Query, tc.query)
			wantEqual(t, "body", len(rec.Body), 0)
			wantEqual(t, "Idempotency-Key", rec.Header.Get("Idempotency-Key"), "")
		})
	}
}

func TestAnalyticsDecoding(t *testing.T) {
	f := newFakeAPI(t,
		reply{Body: `{"offer_id":45,"interval":"7d","status":"Active","metrics":{"views":1200,"clicks":88}}`},
		reply{Body: `{"event_id":3104,"interval":"30d","status":"Scheduled","metrics":{"clicks":188,"attendees":"26-50","invites_sent":"26-50","invites_accepted":0},"as_of":null}`},
	)
	c, _ := newTestClient(t, f)
	oa, err := c.Offers.Analytics(context.Background(), 45, &AnalyticsParams{Interval: Interval7Days})
	if err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "views", oa.Metrics.Views, int64(1200))
	ea, err := c.Events.Analytics(context.Background(), 3104, &AnalyticsParams{Interval: Interval30Days})
	if err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "clicks", ea.Metrics.Clicks, int64(188))
	wantEqual(t, "attendees", ea.Metrics.Attendees, Band("26-50"))
	wantEqual(t, "invites_accepted", ea.Metrics.InvitesAccepted, Band("0"))
	if ea.AsOf != nil {
		t.Error("as_of should be nil")
	}
}

func TestEventsPreview(t *testing.T) {
	f := newFakeAPI(t, reply{Body: `{"return_code":"S1","category":{"category_id":9,"category_name":"Classes"},"listing_fee":10,"currency":"GBP","balance":42.5,"balance_after":42.5,"balance_sufficient":true,"image":{"preview_url":"https://cdn.example/p.jpg","width":1920,"height":1080,"crop":{"x":0,"y":225,"w":2400,"h":1350},"preview_token":"evp_1","expires_at":"2027-09-23T10:30:00Z"}}`})
	c, _ := newTestClient(t, f)
	res, err := c.Events.Preview(context.Background(), &PreviewEventParams{
		Title:     "Beginners Cuban Salsa",
		VenueName: "Bar Salsa",
		Location:  &LatLng{Latitude: 51.5133, Longitude: -0.13},
		PriceMin:  Float64(12),
		PriceMax:  Float64(12),
		Currency:  "GBP",
		Image:     FileFromReader("salsa.jpg", strings.NewReader("jpeg")),
		Crop:      &Crop{X: 0, Y: 225, W: 2400},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "preview_token", res.Image.PreviewToken, "evp_1")
	rec := f.last()
	wantEqual(t, "method", rec.Method, "POST")
	wantEqual(t, "path", rec.Path, "/events/preview")
	wantEqual(t, "Idempotency-Key", rec.Header.Get("Idempotency-Key"), "")
	values, files := parseMultipart(t, rec)
	wantValues(t, values, "title", "Beginners Cuban Salsa")
	wantValues(t, values, "venue_name", "Bar Salsa")
	wantValues(t, values, "location[]", "51.5133", "-0.13")
	wantValues(t, values, "price_min", "12")
	wantValues(t, values, "currency", "GBP")
	wantValues(t, values, "crop[x]", "0")
	wantValues(t, values, "crop[y]", "225")
	wantValues(t, values, "crop[w]", "2400")
	wantValues(t, values, "environment", "TEST")
	if _, ok := values["description"]; ok {
		t.Error("description was sent though it was empty")
	}
	wantEqual(t, "image type", files["image"].ContentType, "image/jpeg")
	wantEqual(t, "image data", string(files["image"].Data), "jpeg")
}

func TestEventsPublish(t *testing.T) {
	f := newFakeAPI(t, reply{Body: `{"return_code":"S1","event_id":3104,"status":"Scheduled","payment_url":null,"amount_charged":0,"currency":"GBP","category":{"category_id":9,"category_name":"Classes"}}`})
	c, _ := newTestClient(t, f)
	res, err := c.Events.Publish(context.Background(), &PublishEventParams{
		Title:        "Beginners Cuban Salsa",
		Description:  "Drop-in class",
		VenueName:    "Bar Salsa",
		Address:      "96 Charing Cross Road",
		City:         "London",
		CountryCode:  "GB",
		Location:     LatLng{Latitude: 51.5133, Longitude: -0.13},
		StartsAt:     "2027-10-16T18:30:00Z",
		EndsAt:       "2027-10-16T23:00:00Z",
		URL:          "https://www.example.com/classes",
		PreviewToken: "evp_1",
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "event_id", res.EventID, int64(3104))
	rec := f.last()
	wantEqual(t, "path", rec.Path, "/events")
	wantEqual(t, "query", rec.Query, "dry_run=true")
	if !uuidV4.MatchString(rec.Header.Get("Idempotency-Key")) {
		t.Errorf("Idempotency-Key = %q", rec.Header.Get("Idempotency-Key"))
	}
	values, files := parseMultipart(t, rec)
	wantValues(t, values, "country_code", "GB")
	wantValues(t, values, "location[]", "51.5133", "-0.13")
	wantValues(t, values, "starts_at", "2027-10-16T18:30:00Z")
	wantValues(t, values, "preview_token", "evp_1")
	wantValues(t, values, "payment_method", "B")
	wantValues(t, values, "environment", "TEST")
	if len(files) != 0 {
		t.Error("a file was sent")
	}
}

func TestEventsList(t *testing.T) {
	f := newFakeAPI(t, reply{Body: `{"data":[{"event_id":1,"title":"A","price_min":12.5,"owner":{"user_id":41,"name":"Owner"}}],"pagination":{"limit":5,"count":1,"has_more":false,"next_cursor":null}}`})
	c, _ := newTestClient(t, f, WithMaxRetries(0))
	res, err := c.Events.List(context.Background(), &ListEventsParams{Environment: EnvironmentTest, Limit: 5, EventStatus: "upcoming", OwnerUserID: 41})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := url.ParseQuery(f.last().Query)
	wantEqual(t, "environment", q.Get("environment"), "TEST")
	wantEqual(t, "limit", q.Get("limit"), "5")
	wantEqual(t, "event_status", q.Get("event_status"), "upcoming")
	wantEqual(t, "owner_user_id", q.Get("owner_user_id"), "41")
	wantEqual(t, "price_min", *res.Data[0].PriceMin, 12.5)
	wantEqual(t, "owner", res.Data[0].Owner.UserID, int64(41))
}

func TestEventsEdit(t *testing.T) {
	t.Run("without a file is a POST with the override", func(t *testing.T) {
		f := newFakeAPI(t, reply{Body: `{"return_code":"S1","event_id":3104,"changed_fields":["title","advertised_name"],"venue_changed":false}`})
		c, _ := newTestClient(t, f)
		res, err := c.Events.Edit(context.Background(), 3104, &EditEventParams{
			Title:          String("Thursday Comedy Club"),
			AdvertisedName: String(""),
			Location:       &LatLng{Latitude: 1.5, Longitude: 2},
			Crop:           &Crop{X: 1, Y: 2, W: 1280},
			ClearPrice:     true,
		})
		if err != nil {
			t.Fatal(err)
		}
		wantEqual(t, "changed", strings.Join(res.ChangedFields, ","), "title,advertised_name")
		rec := f.last()
		wantEqual(t, "method", rec.Method, "POST")
		wantEqual(t, "path", rec.Path, "/events/3104")
		wantEqual(t, "override", rec.Header.Get("X-HTTP-Method-Override"), "PATCH")
		values, _ := parseMultipart(t, rec)
		wantValues(t, values, "title", "Thursday Comedy Club")
		wantValues(t, values, "advertised_name", "")
		wantValues(t, values, "location[]", "1.5", "2")
		wantValues(t, values, "crop[w]", "1280")
		wantValues(t, values, "price_min", "")
		wantValues(t, values, "price_max", "")
		wantValues(t, values, "currency", "")
		if _, ok := values["description"]; ok {
			t.Error("description was sent though it was not set")
		}
	})
	t.Run("with a file is a POST with the override", func(t *testing.T) {
		f := newFakeAPI(t, reply{Body: `{"return_code":"S1","event_id":3104}`})
		c, _ := newTestClient(t, f)
		_, err := c.Events.Edit(context.Background(), 3104, &EditEventParams{
			URL:      String("https://www.example.com/comedy"),
			PriceMin: Float64(5),
			PriceMax: Float64(7.5),
			Currency: String("EUR"),
			Image:    FileFromBytes("new.JPEG", []byte("jpeg")),
		})
		if err != nil {
			t.Fatal(err)
		}
		rec := f.last()
		wantEqual(t, "method", rec.Method, "POST")
		wantEqual(t, "override", rec.Header.Get("X-HTTP-Method-Override"), "PATCH")
		values, files := parseMultipart(t, rec)
		wantValues(t, values, "url", "https://www.example.com/comedy")
		wantValues(t, values, "price_max", "7.5")
		wantValues(t, values, "currency", "EUR")
		wantEqual(t, "image type", files["image"].ContentType, "image/jpeg")
	})
}

func TestEventsReschedule(t *testing.T) {
	f := newFakeAPI(t, reply{Body: `{"return_code":"S1","event_id":3104,"starts_at":"2027-10-23T18:30:00Z"}`})
	c, _ := newTestClient(t, f)
	res, err := c.Events.Reschedule(context.Background(), 3104, &RescheduleEventParams{StartsAt: "2027-10-23T18:30:00Z", EndsAt: "2027-10-23T23:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "starts_at", res.StartsAt, "2027-10-23T18:30:00Z")
	rec := f.last()
	wantEqual(t, "method", rec.Method, "POST")
	wantEqual(t, "path", rec.Path, "/events/3104/reschedule")
	body := decodeJSONBody(t, rec)
	wantEqual(t, "starts_at", body["starts_at"], any("2027-10-23T18:30:00Z"))
	wantEqual(t, "ends_at", body["ends_at"], any("2027-10-23T23:00:00Z"))
}

func TestWebhookEndpointWrites(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name, method, path, body string
		call                     func(c *Client) error
	}{
		{"create", "POST", "/webhook-endpoints", `{"enabled_events":["offer.started","event.published"],"url":"https://hooks.example.com/agilis","description":"Orders"}`, func(c *Client) error {
			ep, err := c.WebhookEndpoints.Create(ctx, &CreateWebhookEndpointParams{
				URL:           "https://hooks.example.com/agilis",
				EnabledEvents: []string{EventTypeOfferStarted, EventTypeEventPublished},
				Description:   String("Orders"),
			})
			if err == nil && ep.Secret != "whsec_x" {
				t.Errorf("secret = %q", ep.Secret)
			}
			return err
		}},
		{"update", "PATCH", "/webhook-endpoints/we_1", `{"status":"disabled"}`, func(c *Client) error {
			_, err := c.WebhookEndpoints.Update(ctx, "we_1", &UpdateWebhookEndpointParams{Status: String("disabled")})
			return err
		}},
		{"rollSecret", "POST", "/webhook-endpoints/we_1/roll-secret", `{"expires_in":"24h"}`, func(c *Client) error {
			_, err := c.WebhookEndpoints.RollSecret(ctx, "we_1", &RollWebhookSecretParams{ExpiresIn: "24h"})
			return err
		}},
		{"rollSecret without params", "POST", "/webhook-endpoints/we_1/roll-secret", `{}`, func(c *Client) error {
			_, err := c.WebhookEndpoints.RollSecret(ctx, "we_1", nil)
			return err
		}},
		{"sendTest", "POST", "/webhook-endpoints/we_1/test", `{"type":"offer.started"}`, func(c *Client) error {
			res, err := c.WebhookEndpoints.SendTest(ctx, "we_1", &SendTestWebhookParams{Type: EventTypeOfferStarted})
			if err == nil && res.Delivery.Status != "succeeded" {
				t.Errorf("delivery = %+v", res.Delivery)
			}
			return err
		}},
		{"resend", "POST", "/webhook-events/whevt_1/resend", `{"endpoint_id":"we_1"}`, func(c *Client) error {
			_, err := c.WebhookEvents.Resend(ctx, "whevt_1", &ResendWebhookEventParams{EndpointID: "we_1"})
			return err
		}},
		{"resend to all", "POST", "/webhook-events/whevt_1/resend", `{}`, func(c *Client) error {
			_, err := c.WebhookEvents.Resend(ctx, "whevt_1", nil)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAPI(t, reply{Body: `{"return_code":"S1","id":"we_1","secret":"whsec_x","delivery":{"endpoint_id":"we_1","status":"succeeded","attempts":1}}`})
			c, _ := newTestClient(t, f)
			if err := tc.call(c); err != nil {
				t.Fatal(err)
			}
			rec := f.last()
			wantEqual(t, "method", rec.Method, tc.method)
			wantEqual(t, "path", rec.Path, tc.path)
			got := decodeJSONBody(t, rec)
			var want map[string]any
			if err := jsonUnmarshalString(tc.body, &want); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Errorf("body = %s, want %s", rec.Body, tc.body)
			}
			for k, v := range want {
				if jsonString(got[k]) != jsonString(v) {
					t.Errorf("body[%s] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestWebhookEventsList(t *testing.T) {
	f := newFakeAPI(t, reply{Body: `{"return_code":"S1","data":[{"id":"whevt_1","type":"offer.started","created":1821341702,"data":{"object":{"offer_id":45},"extra":true},"deliveries":[]}],"pagination":{"limit":20,"count":1,"has_more":false,"next_cursor":null}}`})
	c, _ := newTestClient(t, f)
	res, err := c.WebhookEvents.List(context.Background(), &ListWebhookEventsParams{Type: EventTypeOfferStarted, DeliverySuccess: Bool(false), Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := url.ParseQuery(f.last().Query)
	wantEqual(t, "type", q.Get("type"), "offer.started")
	wantEqual(t, "delivery_success", q.Get("delivery_success"), "false")
	wantEqual(t, "limit", q.Get("limit"), "20")
	wantEqual(t, "extra", string(res.Data[0].Data.Fields["extra"]), "true")
}

func TestNilParams(t *testing.T) {
	c := NewClient(testKey)
	ctx := context.Background()
	if _, err := c.Offers.Create(ctx, nil); err != errNilParams {
		t.Errorf("Offers.Create(nil) = %v", err)
	}
	if _, err := c.Events.Publish(ctx, nil); err != errNilParams {
		t.Errorf("Events.Publish(nil) = %v", err)
	}
	if _, err := c.Events.Edit(ctx, 1, nil); err != errNilParams {
		t.Errorf("Events.Edit(nil) = %v", err)
	}
}
