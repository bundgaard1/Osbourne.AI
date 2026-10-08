# API Examples

Real request/response pairs captured from a running stack with `curl`. Every
status line, header and body below was copied out of a `curl -i` transcript —
the `Date` and `Connection` headers excepted, which are elided for readability.

## How this was captured

```bash
# Consumers first, then everything else - see the note below.
docker compose up -d rabbitmq profile-service notification-service
#   ...wait for both consumers to log that they are listening...
docker compose up -d

curl -i -sS -c cookies.txt -X POST http://localhost/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"student@osbourne.local","password":"student123"}'
curl -i -sS -b cookies.txt ...             # every later call reuses that cookie
```

The staged start is not ceremony. `auth-service` publishes `account.created` for
the two seed accounts the moment it boots, onto a topic exchange. Brought up all
at once, a consumer whose queue is not yet bound never sees those events, and
the profile rows and the welcome notification in example 3 do not exist at all.
So the run starts the two consumers, waits for them to report that they are
listening, and only then starts the publishers.

- Base URL: `http://localhost` — the nginx API gateway, the only host-exposed
  application port. Nothing is published per-service, so these paths exist *only*
  because the gateway routes them.
- Stack: commit `90cf51b`, run on 2026-09-29 local time (NZDT) from a clean
  `docker compose down -v`, so the databases were freshly seeded. The `Date`
  headers and the timestamps inside the bodies are UTC, which is why they read
  as the 28th.
- Account: the seeded student `student@osbourne.local` / `student123`
  (`user_id` `12345`).

Two things are true of every response below and are worth knowing before reading
them.

**The JWT is never in a response body.** Login hands out the token as an
`HttpOnly` cookie and blanks it from the JSON. In the transcript the
`Set-Cookie` value is the only place the raw token appears; it is abbreviated
here as `<jwt>`. It is a real token, signed with the stack's dev secret, and it
expired two hours after the capture.

**Every response carries an `X-Request-Id`, and it is the same id in the
service logs.** The gateway generates it, promotes it into gRPC metadata, and
echoes it back. Quote it when reporting a failure.

Field naming follows the protobuf names, not camelCase — except `isRead`, whose
proto field is genuinely spelled that way. 64-bit integers (`size`) are JSON
strings, which is what `protojson` does with `int64`.

---

## 1. `POST /api/auth/login` — auth-service

The session boundary. The response body identifies the user; the cookie is what
authenticates everything after it.

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

Three things to read off this one response:

- **`Set-Cookie` is the only copy of the token.** `HttpOnly` keeps it out of
  reach of JavaScript, so an XSS bug cannot read the session; `SameSite=Lax` and
  `Max-Age=7200` match the token's own 120-minute lifetime, so the cookie cannot
  outlive a token the server would reject. The cookie is minted by the service
  from inside the RPC and emitted by the gateway, which is why it appears
  alongside the body rather than being set by a handler.
- **No `token` field in the body.** It is deliberately blanked before
  serialisation, and proto3 drops the empty string, so the field is absent
  entirely rather than present-and-empty.
- **`role` is derived server-side** from the stored account, not chosen by the
  caller — the login request cannot ask to be a teacher.

The gateway then promotes this cookie into `Authorization: Bearer <jwt>` on
every later request and drops the raw `Cookie` header, so the services never see
the session cookie itself. A caller may also send `Authorization` directly
instead of using the cookie; the client's header wins, which is why `curl` and
Postman work without a cookie jar at all.

**Related, same service:** `POST /api/auth/validate` introspects a token the
client holds, and `POST /api/auth/logout` expires the cookie. Logout reads no
token and consults no session store — a user whose token has already expired must
still be able to drop the cookie — and it answers `200` with
`Set-Cookie: osbourne_session=; Max-Age=0`.

---

## 2. `POST /api/enrollments` — course-catalogue-service

A synchronous write that also publishes an event. The enrolment is persisted
first, and only then is `course.enrolled` published, so a broker failure cannot
roll back a successful enrolment.

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

**Who got enrolled is not in the request.** There is no `user_id` in the body
and none is needed: the subject comes from the verified JWT, and a `user_id` in
the payload would be ignored. Sending one anyway would enrol nobody else — but
the field is absent here because a route that took an id would invite the
mistake.

**What the gateway did to get here.** `POST /api/enrollments` is proxied to
`course-catalogue-service:8080`, whose grpc-gateway turns it back into a
`coursecatalogue.CourseCatalogueService/EnrollUser` gRPC call, so the same auth
and logging interceptors apply as on the internal path. Note that
`/api/courses/1/assignments` also starts with `/api/courses/` and belongs to a
different service entirely — the gateway disambiguates by regex, and the
ordering of those rules is the only reason the routing works.

**The event it caused**, from `docker compose logs course-catalogue-service`,
sharing the `request_id` from the response header above:

```
{"msg":"published course.enrolled event","student_id":"12345","course_id":"1",
 "event_id":"17bddb43-2c13-447d-9dba-3d92f82ffaf7",
 "request_id":"bae3082f8481d2d7a6db6f66a3fe2181","user_id":"12345"}
```

`request_id` is the `X-Request-Id` from the HTTP response. That is the whole
correlation story in one line: the browser request, the nginx access line, the
gRPC handler log and the event publish all carry the same id, so this event can
be traced back to the click that caused it.

---

## 3. `GET /api/notifications` — notification-service

The asynchronous consequence of example 2. Nothing in this request caused the
notifications; they are the result of events consumed from RabbitMQ by a service
that never talked to the caller.

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

Three notifications, each produced by a different service — none of which the
caller talked to in order to cause it:

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

**This is eventually consistent, not transactional.** The enrolment in example 2
returned `200` before this notification existed. Polling immediately after
publishing can legitimately return an empty list; the run that produced this
transcript retried once a second until the grade notification appeared, and it
took under a second. A UI should treat a missing notification as "not yet", not
as an error.

**Marking one read** is the other route on this service. The id comes from the
URL, so it is attacker-controlled, and the service checks the notification
belongs to the caller rather than trusting the path:

```bash
curl -i -sS -b cookies.txt -X POST \
  http://localhost/api/notifications/440f429c-2a23-45ae-b86b-6afb03d28626/read
```

```
HTTP/1.1 200 OK

{"success":true}
```

The next `GET /api/notifications` shows `"isRead":true` on that entry and nothing
else changed. `isRead` is camelCase in the response because it is camelCase in
the proto; every other field here uses the proto snake_case spelling.

---

## Failure cases

Both shapes below come from one error handler in `common/gateway.go`, so every
REST failure in the system has the same body: the HTTP status repeated as `code`,
`success: false`, and a message.

**401 — the auth interceptor rejecting a request with no token.** Run without a
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

The same 401 comes back from the two streaming file routes
(`POST /api/assignments/{id}/submissions` and
`GET /api/submissions/{id}/file`). Those are not served by generated code, so
they are not covered by the unary auth interceptor; they are protected by a
separate stream interceptor, and the identical response from
`GET /api/submissions/198c2c6f-d2dd-4c4b-a7ef-722614937437/file` is the
evidence that it is actually running on them.

**404 — an unrouted path answered by the gateway itself.** This one never
reaches a service:

```bash
curl -i -sS http://localhost/api/does-not-exist
```

```
HTTP/1.1 404 Not Found
Content-Type: application/json
X-Request-Id: 1a7eb3a61af5514945d868894129b48b

{"code":404,"success":false,"message":"unknown API route"}
```

Without that catch-all, a mistyped `/api/` URL would fall through to the
frontend and be answered with a page of HTML — a syntax error for any client
parsing JSON, rather than a status it can act on.

A 404 from a service looks the same but names the resource, e.g.
`GET /api/courses/1/modules/does-not-exist`:

```
{"code":404,"success":false,"message":"module does-not-exist was not found"}
```


