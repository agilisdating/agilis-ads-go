package agilisads

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"
)

type signatureVectors struct {
	Secret           string `json:"secret"`
	NowUnix          int64  `json:"now_unix"`
	ToleranceSeconds int64  `json:"tolerance_seconds"`
	Body             string `json:"body"`
	Cases            []struct {
		Name   string `json:"name"`
		Header string `json:"header"`
		Valid  bool   `json:"valid"`
		Error  string `json:"error"`
	} `json:"cases"`
}

func loadVectors(t *testing.T) signatureVectors {
	t.Helper()
	b, err := os.ReadFile("testdata/webhook-signature-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v signatureVectors
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Cases) == 0 {
		t.Fatal("no cases")
	}
	return v
}

func TestSignatureVectors(t *testing.T) {
	v := loadVectors(t)
	now := time.Unix(v.NowUnix, 0)
	for _, tc := range v.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			ev, err := ConstructEvent([]byte(v.Body), tc.Header, v.Secret,
				WithNow(func() time.Time { return now }),
				WithTolerance(time.Duration(v.ToleranceSeconds)*time.Second))
			if tc.Valid {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				wantEqual(t, "type", ev.Type, EventTypeEventPublished)
				wantEqual(t, "id", ev.ID, "whevt_01JD0S6Q4M8K1Z0X7C2V9B3N5R")
				wantEqual(t, "livemode", ev.Livemode, true)
				wantEqual(t, "source", ev.Request.Source, "api")
				e, err := ev.Data.Event()
				if err != nil {
					t.Fatal(err)
				}
				wantEqual(t, "event_id", e.EventID, int64(3104))
				wantEqual(t, "attendees", e.Attendees, Band("10-25"))
				wantEqual(t, "amount_charged", string(ev.Data.Fields["amount_charged"]), "10.0")
				wantEqual(t, "raw", string(ev.RawJSON), v.Body)
				return
			}
			if err == nil {
				t.Fatal("accepted")
			}
			var sve *SignatureVerificationError
			if !errors.As(err, &sve) {
				t.Fatalf("err = %T %v", err, err)
			}
			wantEqual(t, "reason", sve.Reason, tc.Error)
			var base *AgilisError
			if !errors.As(err, &base) {
				t.Error("does not unwrap to *AgilisError")
			}
			if VerifySignature([]byte(v.Body), tc.Header, v.Secret, WithNow(func() time.Time { return now })) == nil {
				t.Error("VerifySignature accepted it")
			}
		})
	}
}

func TestSignatureDefaultToleranceUsesTheClock(t *testing.T) {
	v := loadVectors(t)
	// The vectors carry a fixed timestamp, so with the real clock and the
	// default tolerance the valid header is outside the tolerance.
	_, err := ConstructEvent([]byte(v.Body), v.Cases[0].Header, v.Secret)
	var sve *SignatureVerificationError
	if !errors.As(err, &sve) || sve.Reason != "timestamp" {
		t.Fatalf("err = %v", err)
	}
	// With the check turned off it is accepted.
	if _, err := ConstructEvent([]byte(v.Body), v.Cases[0].Header, v.Secret, WithTolerance(0)); err != nil {
		t.Fatal(err)
	}
}

func TestSignatureFreshlySigned(t *testing.T) {
	secret := "whsec_fresh"
	body := []byte(`{"id":"whevt_1","type":"offer.started","created":1,"livemode":false,"data":{"object":{"offer_id":45,"title":"Sale"}},"request":{"source":"system","idempotency_key":null}}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	header := "t=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil))
	ev, err := ConstructEvent(body, header, secret)
	if err != nil {
		t.Fatal(err)
	}
	o, err := ev.Data.Offer()
	if err != nil {
		t.Fatal(err)
	}
	wantEqual(t, "title", o.Title, "Sale")
	if _, err := ConstructEvent([]byte("not json"), header, secret, WithTolerance(0)); err == nil {
		t.Error("a changed body was accepted")
	}
}
