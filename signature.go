package agilisads

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// SignatureHeader is the header that carries a webhook message's signature.
const SignatureHeader = "Agilis-Signature"

// DefaultTolerance is how far a webhook timestamp may be from now.
const DefaultTolerance = 300 * time.Second

// WebhookOption configures ConstructEvent.
type WebhookOption func(*webhookConfig)

type webhookConfig struct {
	tolerance time.Duration
	now       func() time.Time
}

// WithTolerance sets how far the signature's timestamp may be from now. The
// default is DefaultTolerance. Zero or less turns the timestamp check off.
func WithTolerance(d time.Duration) WebhookOption {
	return func(c *webhookConfig) { c.tolerance = d }
}

// WithNow sets the clock used for the timestamp check.
func WithNow(now func() time.Time) WebhookOption {
	return func(c *webhookConfig) {
		if now != nil {
			c.now = now
		}
	}
}

// ConstructEvent verifies a received webhook message and returns its event.
//
// payload is the request body exactly as received, header the value of the
// Agilis-Signature header and secret the endpoint's signing secret (whsec_...).
// The signature is the hex HMAC-SHA256 of "<t>.<payload>"; any matching v1
// is accepted (there are two while a secret is being rolled), compared in
// constant time, and the timestamp must be within the tolerance.
//
// It returns a *SignatureVerificationError when the message cannot be
// verified.
func ConstructEvent(payload []byte, header, secret string, opts ...WebhookOption) (*WebhookEvent, error) {
	cfg := webhookConfig{tolerance: DefaultTolerance, now: time.Now}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if err := verifySignature(payload, header, secret, cfg); err != nil {
		return nil, err
	}
	ev := &WebhookEvent{}
	if err := json.Unmarshal(payload, ev); err != nil {
		return nil, signatureError("payload", "the webhook payload is not valid JSON: "+err.Error())
	}
	ev.RawJSON = append(json.RawMessage(nil), payload...)
	return ev, nil
}

// VerifySignature checks a webhook message's signature without decoding it.
// It returns nil or a *SignatureVerificationError.
func VerifySignature(payload []byte, header, secret string, opts ...WebhookOption) error {
	cfg := webhookConfig{tolerance: DefaultTolerance, now: time.Now}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return verifySignature(payload, header, secret, cfg)
}

func verifySignature(payload []byte, header, secret string, cfg webhookConfig) error {
	var (
		timestamp  int64
		rawTime    string
		haveTime   bool
		signatures []string
	)
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "t":
			if haveTime {
				continue
			}
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return signatureError("header", "the signature header's timestamp is not a number")
			}
			timestamp, rawTime, haveTime = n, v, true
		case "v1":
			if v != "" {
				signatures = append(signatures, v)
			}
		}
	}
	if !haveTime {
		return signatureError("header", "the signature header has no timestamp")
	}
	if len(signatures) == 0 {
		return signatureError("header", "the signature header has no v1 signature")
	}
	if cfg.tolerance > 0 {
		age := cfg.now().Sub(time.Unix(timestamp, 0))
		if age < 0 {
			age = -age
		}
		if age > cfg.tolerance {
			return signatureError("timestamp", "the signature's timestamp is outside the tolerance")
		}
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(rawTime))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := mac.Sum(nil)
	matched := false
	for _, sig := range signatures {
		got, err := hex.DecodeString(sig)
		if err != nil {
			continue
		}
		if hmac.Equal(expected, got) {
			matched = true
		}
	}
	if !matched {
		return signatureError("signature", "no signature matches the payload")
	}
	return nil
}

func signatureError(reason, message string) *SignatureVerificationError {
	return &SignatureVerificationError{AgilisError: AgilisError{Message: message}, Reason: reason}
}
