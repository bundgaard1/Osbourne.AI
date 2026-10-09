# API Examples

Real request and response pairs from a running stack. Every status line, header,
and body comes from a `curl -i` transcript. The `Date` and `Connection` headers
are removed.

## How to capture

```bash
# Start the consumers before the publishers so their queues are bound.
docker compose up -d rabbitmq profile-service notification-service
docker compose up -d

curl -i -sS -c cookies.txt -X POST http://localhost/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"student@osbourne.local","password":"student123"}'
curl -i -sS -b cookies.txt ...             # later calls reuse that cookie
```

auth-service publishes `account.created` for the seed accounts when it boots.
A topic exchange drops a message with no bound queue, so the consumers must be
up first (`docs/screenshots/README.md` has the full capture plan).

- Base URL: `http://localhost`, the nginx API gateway and the only host-exposed
  application port.
- Stack: commit `90cf51b`, run on 2026-09-29 from a clean `docker compose down -v`.
- Account: the seeded student `student@osbourne.local` / `student123`
  (`user_id` `12345`).

**No JWT appears in a response body.** Login sends the token as an `HttpOnly`
cookie and blanks it from the JSON, so `Set-Cookie` is the only place the raw
token appears (abbreviated `<jwt>` here).

**Every response has an `X-Request-Id`, and it is the same id in the service
logs.** Quote it when you report a failure.

Field names use the protobuf names, not camelCase, except `isRead`, whose proto
field is spelled that way.

---

## 1. `POST /api/auth/login` - auth-service

The session boundary. The response body identifies the user; the cookie
authenticates every later call.

**Request**

```bash
curl -i -sS -c cookies.txt -X POST http://localhost/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"student@osbourne.local","password":"student123"}'
```

**Response**

```
HTTP/1.1 200 OK
Server: nginx/1.31.3
Content-Type: application/json
Content-Length: 71
Set-Cookie: osbourne_session=<jwt>; Path=/; Max-Age=7200; HttpOnly; SameSite=Lax
X-Request-Id: 62641180d692f77475cc498c9f62e95f

{"user_id":"12345", "email":"student@osbourne.local", "role":"student"}
```

- **`Set-Cookie` is the only copy of the token.** `HttpOnly` keeps it from
  JavaScript, and `SameSite=Lax` with `Max-Age=7200` matches the 120-minute
  token lifetime.
- **The body has no `token` field.**
- **The service derives `role` on the server**, so the login request cannot ask
  to be a teacher.

The gateway then promotes this cookie to `Authorization: Bearer <jwt>` on every
later request and drops the raw `Cookie` header, so the services never see the
session cookie. A caller can also send `Authorization` directly; the client
header wins, so `curl` and Postman work without a cookie jar.

**Related routes:** `POST /api/auth/validate` introspects a token the client
holds; `POST /api/auth/logout` expires the cookie and answers `200` with
`Set-Cookie: osbourne_session=; Max-Age=0`. Logout reads no token and uses no
session store, so a user with an expired token can still drop the cookie.

---

## 2. `POST /api/enrollments` - course-catalogue-service

A synchronous write that also publishes an event. The enrolment is persisted
first, then `course.enrolled` is published, so a broker failure cannot roll back
a successful enrolment.

**Request**

```bash
curl -i -sS -b cookies.txt -X POST http://localhost/api/enrollments \
  -H 'Content-Type: application/json' \
  -d '{"course_id":"1"}'
```

**Response**

```
HTTP/1.1 200 OK
Server: nginx/1.31.3
Content-Type: application/json
Content-Length: 16
X-Request-Id: bae3082f8481d2d7a6db6f66a3fe2181

{"success":true}
```

The request does not say who is enrolled. There is no `user_id` in the body, and
none is needed: the subject comes from the verified JWT, and the service ignores
a `user_id` in the payload.

The gateway sends `POST /api/enrollments` to
`course-catalogue-service:8080`, whose grpc-gateway turns it back into a
`coursecatalogue.CourseCatalogueService/EnrollUser` gRPC call, so the same auth
and logging interceptors apply as on the internal path. `/api/courses/1/assignments`
also starts with `/api/courses/` but belongs to a different service; the gateway
disambiguates by regex, and the order of those rules is the only reason the
routing works.

**The event it caused**, from `docker compose logs course-catalogue-service`:

```
{"msg":"published course.enrolled event","student_id":"12345","course_id":"1",
 "event_id":"17bddb43-2c13-447d-9dba-3d92f82ffaf7",
 "request_id":"bae3082f8481d2d7a6db6f66a3fe2181","user_id":"12345"}
```

`request_id` is the `X-Request-Id` from the response header above. The browser
request, the nginx access line, the gRPC handler log, and the event publish all
carry the same id, so this event traces back to the click that caused it.

---

## 3. `GET /api/notifications` - notification-service

The asynchronous result of example 2. Nothing in this request caused the
notifications; a service that never talked to the caller consumed events from
RabbitMQ.

**Request**

```bash
curl -i -sS -b cookies.txt http://localhost/api/notifications
```

**Response**

```
HTTP/1.1 200 OK
Server: nginx/1.31.3
Content-Type: application/json
Content-Length: 697
X-Request-Id: b1aefe61a93df8e0bde3dec06de765ec

{"notifications":[
  {"id":"b9e3e979-9f90-4507-a28e-704b51e04f46","user_id":"12345",
   "title":"Welcome to Osbourne!",
   "msg":"Hello Andy Osborne, welcome to Osbourne! We are excited to have you on board.",
   "timestamp":"2026-09-28T23:49:35.657448413Z"},
  {"id":"09e98644-b27e-44c4-b666-bbe205b9782b","user_id":"12345",
   "title":"Enrolled in Course: CS101",
   "msg":"You have been enrolled in the course: Introduction to Computer Science.",
   "timestamp":"2026-09-28T23:49:41.396714170Z"},
  {"id":"440f429c-2a23-45ae-b86b-6afb03d28626","user_id":"12345",
   "title":"Grade published",
   "msg":"Grade updated for Assignment: INFS605 evidence capture; \n Grade: 88; \n Course: 1.",
   "timestamp":"2026-09-28T23:49:41.540918952Z"}]}
```

Three notifications, each from a different service the caller never talked to:

| Notification | Published by | Consumed as |
| --- | --- | --- |
| Welcome to Osbourne! | auth-service, on seed | `account.created` |
| Enrolled in Course: CS101 | course-catalogue-service, by the call in example 2 | `course.enrolled` |
| Grade published | assignment-service, by a grading call | `grade.published` |

`docker compose logs notification-service` shows the consumer side, and the
`event_id`s match the publishers':

```
{"msg":"received event","event_id":"17bddb43-2c13-447d-9dba-3d92f82ffaf7","event_type":"course.enrolled"}
{"msg":"received event","event_id":"8c5dcb7d-7baf-41ca-893d-c8ddcc4e762e","event_type":"grade.published"}
```

This is eventually consistent, not transactional. The enrolment in example 2
returned `200` before this notification existed, so a poll immediately after
publish can return an empty list. A UI should treat a missing notification as
"not yet", not as an error.

**Marking one read** is the other route on this service. The id comes from the
URL, so it is attacker-controlled; the service checks the notification belongs
to the caller.

```bash
curl -i -sS -b cookies.txt -X POST \
  http://localhost/api/notifications/440f429c-2a23-45ae-b86b-6afb03d28626/read
```

```
HTTP/1.1 200 OK

{"success":true}
```

The next `GET /api/notifications` shows `"isRead":true` on that entry and
nothing else changed. `isRead` is camelCase because it is camelCase in the
proto; every other field uses the proto snake_case spelling.

---

## Failure cases

Both shapes below come from one error handler in `common/gateway.go`, so every
REST failure has the same body: the HTTP status as `code`, `success: false`, and
a message.

**401: the auth interceptor rejects a request with no token.** Run without a
cookie and without an `Authorization` header:

```bash
curl -i -sS http://localhost/api/profile
```

```
HTTP/1.1 401 Unauthorized
Content-Type: application/json
X-Request-Id: 2ab6f7226b7e6c8bfb7f0f6ae4eae592

{"code":401,"success":false,"message":"missing or invalid bearer token"}
```

The two streaming file routes (`POST /api/assignments/{id}/submissions` and
`GET /api/submissions/{id}/file`) return the same 401. They are not served by
generated code, so the unary auth interceptor does not cover them; a separate
stream interceptor protects them, and this identical response is the evidence
that it runs on them.

**404: an unrouted path, answered by the gateway.** This one never reaches a
service:

```bash
curl -i -sS http://localhost/api/does-not-exist
```

```
HTTP/1.1 404 Not Found
Content-Type: application/json
X-Request-Id: 1a7eb3a61af5514945d868894129b48b

{"code":404,"success":false,"message":"unknown API route"}
```

A service 404 looks the same but names the resource, e.g.
`GET /api/courses/1/modules/does-not-exist`:

```
{"code":404,"success":false,"message":"module does-not-exist was not found"}
```
