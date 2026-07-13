# fakepal

Mock Payment Gateway (PG) API written in Go. PayPal Payments v2–shaped endpoints for local development and automated tests.

## Quick start

```bash
go run ./cmd/fakepal
```

Defaults:

| Env | Default | Description |
|-----|---------|-------------|
| `PORT` | `8080` | Listen port |
| `API_KEY` | `test-api-key` | Bearer token required on all routes except `/healthz` |

```bash
export API_KEY=test-api-key
curl -s http://localhost:8080/healthz
```

## Auth

```http
Authorization: Bearer test-api-key
```

## API (phase 1)

| Method | Path | Notes |
|--------|------|-------|
| `GET` | `/healthz` | No auth |
| `POST` | `/v2/payments/authorizations` | Create authorization |
| `GET` | `/v2/payments/authorizations/{id}` | Get authorization |
| `POST` | `/v2/payments/authorizations/{id}/capture` | Capture (omit body for full remaining) |
| `POST` | `/v2/payments/authorizations/{id}/void` | Void (only if no captures) |
| `GET` | `/v2/payments/captures/{id}` | Get capture |
| `POST` | `/v2/payments/captures/{id}/refund` | Refund (omit body for full remaining) |
| `GET` | `/v2/payments/refunds/{id}` | Get refund |

### Create authorization

```bash
curl -s -X POST http://localhost:8080/v2/payments/authorizations \
  -H "Authorization: Bearer test-api-key" \
  -H "Content-Type: application/json" \
  -d '{"amount":{"currency_code":"USD","value":"100.00"}}'
```

### Capture / void / refund

```bash
# Partial capture
curl -s -X POST http://localhost:8080/v2/payments/authorizations/$AUTH_ID/capture \
  -H "Authorization: Bearer test-api-key" \
  -H "Content-Type: application/json" \
  -H "PayPal-Request-Id: capture-1" \
  -d '{"amount":{"currency_code":"USD","value":"40.00"}}'

# Void (only before any capture)
curl -s -o /dev/null -w "%{http_code}\n" -X POST \
  http://localhost:8080/v2/payments/authorizations/$AUTH_ID/void \
  -H "Authorization: Bearer test-api-key"

# Refund
curl -s -X POST http://localhost:8080/v2/payments/captures/$CAP_ID/refund \
  -H "Authorization: Bearer test-api-key" \
  -H "Content-Type: application/json" \
  -d '{"amount":{"currency_code":"USD","value":"10.00"}}'
```

## Failure injection

- Amount ending in `.13` → `INSTRUMENT_DECLINED` (422)
- Header `PayPal-Mock-Response: {"mock_application_codes":"INSTRUMENT_DECLINED"}` → injected decline

## Idempotency

Send `PayPal-Request-Id` on capture/refund. The same key returns the same resource.

## Point your app at fakepal

Set your PayPal/PG base URL to `http://localhost:8080` and use `API_KEY` as the Bearer token (skip real OAuth in local/test config).

## Tests

```bash
go test ./...
```
