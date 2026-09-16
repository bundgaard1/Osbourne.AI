# Osborne.AI - Microservices Project

This project is made for the INFS605 (Microservices) course project. The goal is to create a Student Services Dashboard for university operations using a microservices architecture.

## Setup steps

Prerequisites:

- Go 1.26+
- Docker + Docker Compose
- `buf`, `templ`, `protoc-gen-go`, `protoc-gen-go-grpc` (installed automatically by `make tools`)

Run the following commands to set up the project:

```bash
make generate   # Generates protobufs/gRPC stubs and the frontend templ code
docker compose up --build   # Builds and starts all services in Docker containers
```

- Open the dashboard at **http://localhost:8080**
- The API Gateway (Nginx) is also exposed on **http://localhost:80**
- RabbitMQ management console: **http://localhost:15672** (`guest` / `guest`)

### Demo accounts

The services seed two accounts the first time they start:

| Role    | Email                    | Password    |
| ------- | ------------------------ | ----------- |
| Student | `student@osbourne.local` | `student123` |
| Teacher | `teacher@osbourne.local` | `teacher123` |

## Tech stack

- All services are written in **Go** (`log/slog` structured logging).
- Frontend is a server-rendered web UI built with **Go + Templ** and the `chi` router.
- API Gateway is built on **Nginx** (routes to the frontend and injects `X-Request-ID`).
- **Synchronous** inter-service calls use **gRPC** (Protobuf).
- **Asynchronous** event processing uses **RabbitMQ** (durable `university.events` topic exchange).
- **Per-service databases**: each service owns an embedded **SQLite** database (GORM), with the exception of the Course Content Service which uses the **CloverDB** document store. There is no shared database.

## Architecture

The platform is split into small, independently deployable services:

- **Authentication Service** (`auth-service`): creds, JWT issuance/validation and the `account.created` event.
- **Student Profile Service** (`profile-service`): student/staff profile master data, materialised from `account.created`.
- **Course Catalogue Service** (`course-catalogue-service`): courses, catalog, and course enrolment; publishes `course.enrolled`.
- **Course Content Service** (`course-content-service`): course content/modules (CloverDB document store).
- **Assignment / Grading Service** (`assignment-service`): assignments, submissions (file upload/download), and grading; publishes `grade.published`.
- **Notification Service** (`notification-service`): inbox notifications consumed from `account.created`, `course.enrolled` and `grade.published`.
- **Frontend UI** (`frontend`): user-facing dashboard that talks to all services over gRPC.

Shared code (logging, JWT parsing, gRPC interceptors, request-ID correlation) lives in the `auth-common` module.

## Observability

Every process emits **JSON structured logs** to stdout via `log/slog`:

```json
{"time":"...","level":"INFO","msg":"received enroll_user request","service":"course-catalogue-service","course_id":"1","request_id":"...","user_id":"12345"}
```

Log lines are enriched with:

- `service` — which component logged it.
- `request_id` — a correlation ID propagated across the whole stack (Nginx `$request_id` → `X-Request-ID` header → frontend gRPC metadata → downstream services). Whole traces can be followed by grepping on one ID.
- `user_id` — injected from the verified JWT claims.

Per-service log level is configurable via the `LOG_LEVEL` env var (`debug | info | warn | error`); GORM and go-rabbitmq chatter is routed through `slog` so the stream stays pure JSON.

## Docs

- [Design document](docs/design.md) — architecture, isolation, service endpoints, event catalog, and request correlation.
- [Plan / issues](docs/plan.md) — the implementation plan and known issues/Deltas.
- [Notes](docs/notes.md) — additional project notes.