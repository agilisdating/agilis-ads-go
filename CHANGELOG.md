# Changelog

All notable changes to this module are listed here. Versions follow [Semantic Versioning](https://semver.org).

## Unreleased

## 0.1.0 (23 November 2026)

### Added

- The client, with the API key from the caller or `AGILIS_API_KEY`, and options for the base URL, timeout, retries and HTTP client.
- Every operation of the Agilis Advertising REST API: categories, interests, Offers, Event categories, Event currencies, Event config, Events, webhook endpoints and webhook events.
- Auto-paging iterators for Offers, Events and webhook events.
- Idempotency keys for creating Offers and publishing Events, kept across retries.
- Retries with `Retry-After` and exponential backoff.
- Typed errors that work with `errors.As`.
- `ConstructEvent` and `VerifySignature` for webhook messages.
