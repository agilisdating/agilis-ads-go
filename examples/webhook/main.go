// Command webhook is a minimal webhook receiver. It checks the
// Agilis-Signature header of every message with the endpoint's secret.
//
//	AGILIS_WEBHOOK_SECRET=whsec_... go run ./examples/webhook
package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"

	agilisads "github.com/agilisdating/agilis-ads-go"
)

func main() {
	secret := os.Getenv("AGILIS_WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("set AGILIS_WEBHOOK_SECRET to the endpoint's signing secret")
	}

	http.HandleFunc("/agilis/webhooks", func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}
		event, err := agilisads.ConstructEvent(payload, r.Header.Get(agilisads.SignatureHeader), secret)
		var sigErr *agilisads.SignatureVerificationError
		if errors.As(err, &sigErr) {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		// Answer quickly, then do the work. Use event.ID to ignore repeats.
		w.WriteHeader(http.StatusNoContent)

		switch event.Type {
		case agilisads.EventTypeOfferStarted, agilisads.EventTypeOfferEnded:
			offer, err := event.Data.Offer()
			if err == nil {
				log.Printf("%s: offer %d %q", event.Type, offer.OfferID, offer.Title)
			}
		case agilisads.EventTypeEventPublished:
			ev, err := event.Data.Event()
			if err == nil {
				log.Printf("%s: event %d %q", event.Type, ev.EventID, ev.Title)
			}
		default:
			log.Printf("%s (%s)", event.Type, event.ID)
		}
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
