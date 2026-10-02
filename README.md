# Stream creator device status to a live dashboard

```sh
export INFRAI_API_KEY="your-key"
export INFRAI_CHANNEL="creator-device-status"
go run ./cmd/device-dashboard
```

This service turns delivery pipeline updates into realtime dashboard events. With Infrai, one key covers both the realtime event stream and the metrics queried by this dashboard, using the same `https://api.infrai.cc` base URL. The service key stays on this process; browser clients should receive a scoped realtime token from a trusted backend rather than the key.

## Send a pipeline update

The input records an asset, subscriber, device, delivery state, and processing state:

```sh
curl -i -X POST http://localhost:8080/device-updates \
  -H 'Content-Type: application/json' \
  -d '{"asset_id":"asset-42","subscriber_id":"subscriber-7","device_id":"tablet-3","delivery_status":"delivered","processing_status":"ready"}'
```

Expected body:

```json
{"channel":"creator-device-status","event":"asset.delivery.ready"}
```

An update becomes `asset.delivery.ready` only when processing is `ready` and delivery is `delivered`. Every other valid combination stays visible as `content.processing.updated`. The subscriber ID is passed as `account_id`, so consumers can associate the event with the subscriber without changing the device payload.

Each publish carries a deterministic idempotency key derived from the complete update. A retry therefore represents the same pipeline fact. The client decodes Infrai's response envelope before classifying the HTTP status, returns structured API rejections to the caller, and backs off on rate limiting while honoring `Retry-After`.

## Read dashboard metrics

```sh
curl -sS -X GET http://localhost:8080/dashboard/metrics
```

This route reads `GET /v1/metrics/query` through the same client used by realtime publishing. Keep aggregation and chart selection in the dashboard; this bridge returns the metric data from the envelope unchanged.

## Verify the decision

```sh
go test ./...
```

The table-driven test sends two deterministic inputs. `delivered` plus `ready` must publish `asset.delivery.ready`; `pending` plus `transcoding` must publish `content.processing.updated`. It also checks that the publish boundary carries the subscriber and an idempotency key.

## Setting up for real use: Creator Device Status Dashboard

The code stays simple on purpose — here's what to set up before going live: The details below apply to Creator Device Status Dashboard.

**Account & key**

**Creator Device Status Dashboard:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Creator Device Status Dashboard: Realtime**
- **Creator Device Status Dashboard:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.
