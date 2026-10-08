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

## Demo Video

Make a video of the system in action,
- showing the frontend, 
- showingand how the system reacts with 
- logging and events when 
  - a student enrolls in a course,
  - a student submits an assignment, 
  - and receives a grade.

## Tech stack

- All services are written in **Go**.
- Frontend is a server-rendered web UI built with **Go + Templ** and the `chi` router. It serves HTML and nothing else.
- API Gateway is built on **Nginx**; it routes `/api/*` to the owning service and everything else to the frontend, and it injects `X-Request-ID`.
- Every backend service exposes a **dual listener**: a gRPC server for internal service-to-service calls, and a **grpc-gateway** REST listener in front of that same gRPC server for browser traffic.
- **Synchronous** inter-service calls use **gRPC**.
- **Asynchronous** event processing uses **RabbitMQ** (durable `university.events` topic exchange).
- **Per-service databases**: each service owns an embedded **SQLite** database (GORM), with the exception of the Course Content Service which uses the **CloverDB** document store. There is no shared database.

## Architecture

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

## Docs

- [API examples](docs/api-examples.md) — real `curl` request/response pairs captured from a running stack, including the failure cases.
- [Design document](docs/design.md) — architecture, isolation, service endpoints, event catalog, and request correlation.
- [Plan / issues](docs/plan.md) — the implementation plan and known issues/Deltas.
- [Notes](docs/notes.md) — additional project notes.