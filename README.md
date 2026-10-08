# Osborne.AI - Microservices Project

This project is made for the INFS605 (Microservices) course project. The goal is to create a Student Services Dashboard for university operations using a microservices architecture.

Github: https://github.com/bundgaard1/osbourne.ai

## Setup steps

Prerequisites:

- Go 1.26+
- Docker + Docker Compose
- `buf`, `templ`, `protoc-gen-go`, `protoc-gen-go-grpc` (installed automatically by `make tools`)

Run the following commands to set up the project:

```bash
make generate   # Generates protobufs/gRPC stubs and the frontend templ code
cp .env.example .env   # Optional: override configuration (defaults match the code)
docker compose up --build   # Builds and starts all services in Docker containers
```

- Open the dashboard at **http://localhost:80** — the API Gateway (Nginx) is the only host-exposed application port.
- RabbitMQ management console: **http://localhost:15672** (`guest` / `guest`)

The services are not published on host ports. In particular the frontend is deliberately unexposed: it serves HTML only, and a directly reachable copy would have pages whose `/api/*` calls have nowhere to go.

### Demo accounts

The services seed two accounts the first time they start. The same details are printed on the login page for convenience.

| Role    | Email                    | Password    | Name           |
| ------- | ------------------------ | ----------- | -------------- |
| Student | `student@osbourne.local` | `student123` | Andy Osborne   |
| Teacher | `teacher@osbourne.local` | `teacher123` | Dr. Jane Teacher |

Account creation is not exposed anywhere, so these seeds are the only accounts that exist.

## Evidence

The behaviour described above was captured from a running stack rather than
asserted:

- **[docs/api-examples.md](docs/api-examples.md)** — real `curl` request/response
  pairs for the session boundary, an enrolment and its asynchronous
  notification, plus the shared error shape for 401 and 404.
- **[docs/screenshots/](docs/screenshots/)** — the Docker stack, the UI end to
  end, RabbitMQ queue counters, a single `request_id` traced across the services,
  the error responses, and data surviving `docker compose restart`. The capture
  plan and the filenames live in that folder's `README.md`.

## Tech stack

- All services are written in **Go**.
- Frontend is a server-rendered web UI built with **Go + Templ** and the `chi` router. It serves HTML and nothing else.
- API Gateway is built on **Nginx**; it routes `/api/*` to the owning service and everything else to the frontend, and it injects `X-Request-ID`.
- Every backend service exposes a **dual listener**: a gRPC server for internal service-to-service calls, and a **grpc-gateway** REST listener in front of that same gRPC server for browser traffic.
- **Synchronous** inter-service calls use **gRPC**.
- **Asynchronous** event processing uses **RabbitMQ** (durable `university.events` topic exchange).
- **Per-service databases**: each service owns an embedded **SQLite** database (GORM), with the exception of the Course Content Service which uses the **CloverDB** document store. There is no shared database.

## Architecture

![Architecture diagram](docs/arch-diagram.png)

The platform is split into small, independently deployable services:

- **Authentication Service** (`auth-service`): creds, JWT issuance/validation and the `account.created` event. It owns the session boundary.
- **Student Profile Service** (`profile-service`): student/staff profile master data, materialised from `account.created`.
- **Course Catalogue Service** (`course-catalogue-service`): courses, catalog, and course enrolment; publishes `course.enrolled`.
- **Course Content Service** (`course-content-service`): course content/modules (CloverDB document store).
- **Assignment / Grading Service** (`assignment-service`): assignments, submissions (file upload/download), and grading; publishes `grade.published`.
- **Notification Service** (`notification-service`): inbox notifications consumed from `account.created`, `course.enrolled` and `grade.published`.
- **Frontend UI** (`frontend`): server-rendered dashboard that reads from all services over gRPC.

There are two non-overlapping synchronous paths. The frontend uses **gRPC** to fetch what it needs to render a page. The browser's `/api/*` calls go through the gateway to the owning service's REST listener, which translates them back into gRPC so the same auth and logging interceptors apply. The browser never receives a JWT: auth-service delivers it as an `HttpOnly` cookie and strips it from the JSON body, and the user's role is derived server-side rather than chosen at login.

Shared code (logging, JWT parsing, gRPC interceptors, the gateway listener, request-ID correlation) lives in the `common` module.

## API endpoints

Every browser-facing call reaches the gateway at `http://localhost` and is served
by the owning service's REST listener; the frontend serves HTML only. The surface
is **25 endpoints** — 23 generated from the protobuf `google.api.http`
annotations, plus two hand-written streaming routes. Authenticated routes need
the session cookie (or an explicit `Authorization: Bearer` header), and every
failure uses the one JSON shape documented in
[docs/api-examples.md](docs/api-examples.md).

### auth-service

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/auth/login` | Validate credentials, set the `HttpOnly` session cookie |
| `POST` | `/api/auth/validate` | Introspect a token |
| `POST` | `/api/auth/logout` | Expire the session cookie |

### profile-service

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/profile` | Current user's profile |
| `PUT` | `/api/profile` | Replace the current user's profile |

### course-catalogue-service

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/courses` | List courses (paginated: `page`, `page_size`) |
| `GET` | `/api/courses/{course_id}` | One course |
| `POST` | `/api/enrollments` | Enrol the current user in a course |
| `GET` | `/api/enrollments/me` | Courses the current user is enrolled in |

### course-content-service

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/courses/{course_id}/modules` | List a course's modules |
| `POST` | `/api/courses/{course_id}/modules` | Create a module |
| `GET` | `/api/courses/{course_id}/modules/{module_id}` | One module |
| `PUT` | `/api/courses/{course_id}/modules/{module_id}` | Update a module |
| `DELETE` | `/api/courses/{course_id}/modules/{module_id}` | Delete a module |

### assignment-service

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/courses/{course_id}/assignments` | Assignments for a course |
| `POST` | `/api/courses/{course_id}/assignments` | Create an assignment |
| `GET` | `/api/assignments/{assignment_id}` | One assignment |
| `GET` | `/api/assignments/{assignment_id}/submissions` | All submissions for an assignment |
| `GET` | `/api/assignments/{assignment_id}/submissions/mine` | The caller's submissions |
| `GET` | `/api/submissions/{submission_id}` | One submission |
| `POST` | `/api/submissions/{submission_id}/grade` | Grade a submission |
| `POST` | `/api/assignments/{assignment_id}/submissions` | Upload a submission (multipart, streaming) |
| `GET` | `/api/submissions/{submission_id}/file` | Download a submission (binary, streaming) |

The last two are hand-written (`mux.HandlePath`) rather than generated, so a file
is streamed instead of base64-encoded inside JSON. The application caps uploads
at 10 MB; nginx allows 12 MB to leave room for the multipart envelope.

### notification-service

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/notifications` | The caller's notifications |
| `POST` | `/api/notifications/{notification_id}/read` | Mark one notification read |

## Testing process

**Go tests and vet, per module.** The repo is a `go.work` workspace, so `./...`
has to be evaluated inside each module:

```bash
for m in common frontend auth-service profile-service notification-service \
         course-catalogue-service course-content-service assignment-service; do
  (cd "$m" && go test ./... && go vet ./...)
done
```

**Gateway routing.** The `/api/*` routes are regex-ordered, and `nginx -t` cannot
see a misordering — every route parses, the wrong one just wins. `routing-test.sh`
runs the whole route table against stub upstreams and asserts which service
receives each path, the JSON 404 catch-all, query-string preservation,
cookie → bearer promotion, and the 12 MB body limit:

```bash
./nginx/routing-test.sh   # requires docker
```

**Manual end-to-end.** `docker compose up --build`, then exercise the UI and the
documented `curl` calls. The captured results are the evidence in
[docs/api-examples.md](docs/api-examples.md) and
[docs/screenshots/](docs/screenshots/).

## Known limitations

- **gRPC is unencrypted.** All internal service-to-service traffic uses
  `insecure` credentials — no TLS or mTLS. Acceptable on the single private
  Compose network, not production-ready.
- **RabbitMQ uses the stock `guest:guest` credentials** and the management
  dashboard is reachable on `localhost:15672`. Docker Compose does not
  interpolate `.env` into the services' `environment:` blocks, so these cannot be
  overridden from `.env` as shipped (noted in `.env.example`).
- **No rate limiting** on the gateway or any service.
- **No health checks** on the Go services — none expose `/healthz` and none have
  a Docker `HEALTHCHECK`; only RabbitMQ does.
- **The notification consumer is not idempotent.** RabbitMQ delivery is
  at-least-once, so a redelivered event can produce a duplicate notification.

## Docs

- [API examples](docs/api-examples.md) — real `curl` request/response pairs captured from a running stack, including the failure cases.
- [Design document](docs/design.md) — architecture, isolation, service endpoints, event catalog, and request correlation.
- [Plan / issues](docs/plan.md) — the implementation plan and known issues/Deltas.
- [Notes](docs/notes.md) — additional project notes.