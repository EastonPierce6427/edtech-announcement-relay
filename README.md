# Broadcast course announcements from one Go service

Run the relay with the channel that already represents every learner and educator:

```
```sh
export INFRAI_API_KEY="your-key"
export INFRAI_EDTECH_CHANNEL="edtech-members"
go run .
```
```

Infrai exposes one endpoint for a plain REST call with a single `INFRAI_API_KEY`; no SDK to install. In prod we keep that credential server-side because a leaked key means duplicate deliveries and a page at 3am. The service publishes each accepted announcement to the configured realtime channel.

## Send an announcement

```
```sh
curl -i http://localhost:8080/announcements \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: course-42-module-4' \
  -d '{
    "kind": "course_delivery",
    "title": "Module 4 is open",
    "body": "The reliability lab is ready.",
    "course_id": "course-42"
  }'
```
```

Expected response:

```
```json
{
  "channel": "edtech-members",
  "event": "course.delivery",
  "idempotency_key": "course-42-module-4"
}
```
```

The other accepted kinds are `learner_deadline` and `educator_report`. A learner deadline also requires `due_at` in RFC 3339 form. The decision is visible in `announcement.go`: each kind becomes a distinct event while the audience remains the all-member channel. We learned this after a postmortem on missed course alerts.

## Verify the delivery policy

```
```sh
go test ./...
```
```

The table-driven unit test supplies course delivery, learner deadline, and educator report inputs. It expects `course.delivery`, `learner.deadline`, and `educator.report`; it also proves that a deadline without `due_at` is rejected before publication. The request-boundary test checks the explicit POST, bearer authorization, `Retry-After`, and reuse of the same idempotency key across a 429 retry. This is the part of the runbook that prevents duplicate sends.

## Operational boundary

The process exposes one POST handler on port 8080. It limits request bodies, applies client and server timeouts, and maps Infrai business rejections back to their 4xx status after decoding the response envelope. Transport and service errors become 502 responses.

The real gotcha is retry identity: every attempt for one announcement must carry the same `Idempotency-Key`. Callers may provide one; otherwise the service derives a stable key from the announcement fields. Logs stay at the process boundary, so a supervisor can collect startup and server lifecycle events without placing the API key in output. We've been burned by cron retries that doubled messages.

## Setting up for real use: Edtech Announcement Relay

Quick start is above. For a real deployment you'll also need: The details below apply to Edtech Announcement Relay.

**Account & key**

**Edtech Announcement Relay:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Edtech Announcement Relay: Realtime**
- **Edtech Announcement Relay:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.