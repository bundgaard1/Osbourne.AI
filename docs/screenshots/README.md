# Evidence capture plan

This folder holds the screenshots that back up the README. Nothing has been
captured yet — this file is the plan to follow, and the checklist at the bottom
is the tracking list. When each image lands here under the filename given, wire
it into the README `## Evidence` section using the gallery snippet at the end.

The point of these captures is the assignment tip: show more than the UI. Each
shot below is chosen to prove a *service boundary* — routing, the message queue,
request correlation, error handling, or persistence — not just that a page
renders.

## 0. Preparation

Capture from a **clean** stack so the seed data is predictable, and start the
two consumers before the publishers. This staged boot is load-bearing:
`auth-service` publishes `account.created` for the seed accounts the moment it
starts, and a topic exchange drops a message with no bound queue, so a consumer
started afterwards never sees it (see `docs/api-examples.md`).

```bash
docker compose down -v            # wipe volumes for a known-clean run
docker compose up -d rabbitmq profile-service notification-service
# wait for both consumers to log that they are listening:
docker compose logs -f profile-service notification-service
#   "ProfileConsumer listening on RabbitMQ for account.created"
#   "NotificationConsumer listening on RabbitMQ"
docker compose up -d              # then the rest
docker compose ps
```

Use the seeded student for the flows:
`student@osbourne.local` / `student123`.

Keep a cookie jar for the `curl` captures:

```bash
curl -i -sS -c cookies.txt -X POST http://localhost/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"student@osbourne.local","password":"student123"}'
```

## 1. `docker compose ps` — the whole stack up

- **Shows:** one container per service (7 backends + nginx + rabbitmq), all
  `Up`, only nginx (`80`) and rabbitmq (`15672`) published.
- **Command:** `docker compose ps`
- **Filename:** `01-compose-ps.png`
- **Make it evidence:** keep the `PORTS` column visible so the reader can see
  no service is individually exposed.

## 2. Login and dashboard — the UI works end to end

- **Shows:** the login page, then the rendered dashboard after signing in, with
  the profile weather/summary widgets populated from the backend over gRPC.
- **How:** open `http://localhost/`, log in as the seeded student.
- **Filename:** `02-login-dashboard.png` (login), `03-dashboard.png` (after).
- **Make it evidence:** the dashboard's enrolment list is data, not static HTML,
  so its presence proves the frontend → profile/catalogue gRPC path works.

## 3. Enrol → notification, plus RabbitMQ counters

The centrepiece: one browser click, then the asynchronous consequence.

1. Enrol the student in a course from the catalogue
   (`POST /api/enrollments`), staying on the page.
   - **Filename:** `04-enrol.png`
2. Open the notification inbox at `/notifications` and show the new
   `Enrolled in Course: …` entry.
   - **Filename:** `05-notification.png`
3. Open the RabbitMQ management console at `http://localhost:15672`
   (`guest` / `guest`) → **Queues and Streams**, and show
   `notification_service_queue` (and `profile_service_queue`). Capture the
   message counters.
   - **Filename:** `06-rabbitmq-queues.png`
   - **Make it evidence:** the notification exists because a *different*
     service consumed an event the caller never touched. The queue view shows
     the exchange → queue binding and the publish/ack counters that connect
     the two.
4. Optional but strong: the publish and consume log lines for the same
   `event_id`:
   ```bash
   docker compose logs course-catalogue-service notification-service | grep course.enrolled
   ```
   - **Filename:** `07-event-logs.png`

## 4. `request_id` correlated across the stack

- **Shows:** one id — the `X-Request-Id` value in a browser network response —
  appearing in nginx, frontend and the backend service logs.
- **How:**
  1. In the browser, open the network tab, load a page, copy the response
     `X-Request-Id`.
  2. `docker compose logs | grep <that-id>`
- **Filename:** `08-request-id-trace.png`
- **Make it evidence:** keep the id visible in the terminal output and in the
  network tab so the match is checkable, not asserted.

## 5. Error handling

Three captures; each proves a different layer rejects the request **with the
shared JSON error shape**, not an HTML page.

| # | Scenario | Command | Expected | Filename |
| --- | --- | --- | --- | --- |
| 5a | No token (auth interceptor) | `curl -i -sS http://localhost/api/profile` | `401 {"code":401,"success":false,"message":"missing or invalid bearer token"}` | `09-error-401.png` |
| 5b | Unknown `/api/` path (gateway catch-all) | `curl -i -sS http://localhost/api/does-not-exist` | `404 {"code":404,"success":false,"message":"unknown API route"}` | `10-error-gateway-404.png` |
| 5c | Missing resource (service 404, the fixed bug) | `curl -i -sS -b cookies.txt http://localhost/api/courses/9999` | `404 {"code":404,"success":false,"message":"course not found"}` | `11-error-service-404.png` |

- **Make it evidence:** 5a proves the token is required; 5b proves an unrouted
  path is JSON rather than the frontend's HTML; 5c is the regression fix from
  `plan.md` issue **#588** (a missing course used to be a 500).

## 6. Persistence across a restart

- **Shows:** data created before a restart is still there afterwards, i.e. the
  named volumes, not in-container state.
- **How:**
  1. Note the current notifications and enrolment (e.g. `GET /api/notifications`
     or the UI).
  2. `docker compose restart`
  3. Reload the same page / re-run the same call against the restarted stack.
- **Filename:** `12a-before-restart.png`, `12b-after-restart.png`
- **Make it evidence:** the same notification ids / rows must appear after the
  restart. Optionally show `docker volume ls | grep osbourne` to name the
  volumes that hold the data.

## 7. Wire the images into the README

Once captured, add an `### Screenshots` gallery under the README `## Evidence`
section, e.g.:

```markdown
### Screenshots

| | |
| --- | --- |
| ![Docker stack](docs/screenshots/01-compose-ps.png) | ![Dashboard](docs/screenshots/03-dashboard.png) |
| ![Enrol](docs/screenshots/04-enrol.png) | ![Notification](docs/screenshots/05-notification.png) |
| ![RabbitMQ queues](docs/screenshots/06-rabbitmq-queues.png) | ![Correlated request id](docs/screenshots/08-request-id-trace.png) |

Failure cases: [401](docs/screenshots/09-error-401.png) ·
[gateway 404](docs/screenshots/10-error-gateway-404.png) ·
[service 404](docs/screenshots/11-error-service-404.png).

Persistence: [before restart](docs/screenshots/12a-before-restart.png) →
[after restart](docs/screenshots/12b-after-restart.png).
```

Do **not** include `.env`, tokens, or the raw `Set-Cookie` value in any shot.

## Checklist

- [ ] `01-compose-ps.png` — full stack up (`docker compose ps`)
- [ ] `02-login-dashboard.png` — login page
- [ ] `03-dashboard.png` — dashboard after login
- [ ] `04-enrol.png` — enrolment from the UI
- [ ] `05-notification.png` — enrolment notification in the inbox
- [ ] `06-rabbitmq-queues.png` — `localhost:15672` queue counters
- [ ] `07-event-logs.png` — publisher/consumer logs, same `event_id` (optional)
- [ ] `08-request-id-trace.png` — one `request_id` across nginx → frontend → service
- [ ] `09-error-401.png` — 401 with no token
- [ ] `10-error-gateway-404.png` — gateway 404 for an unknown `/api/` path
- [ ] `11-error-service-404.png` — service 404 for `GET /api/courses/9999`
- [ ] `12a-before-restart.png` / `12b-after-restart.png` — persistence
- [ ] README `## Evidence` gallery wired in
