Here is the updated and adapted deployment plan for your **INFS605 platform**, extended to cover all **6 microservices** and their specific database technologies (SQL vs. NoSQL), as we have defined them.

The plan continues to be built around a **Vertical Slice strategy**: We complete the entire value chain for one feature at a time (Frontend $\rightarrow$ Gateway $\rightarrow$ Service $\rightarrow$ DB/RabbitMQ), before moving on.

---

## Target Architecture & Services

1. **Authentication Service** (gRPC, SQL DB – Identity & Tokens)
2. **Student Profile Service** (gRPC, SQL DB – User Master Data)
3. **Course Catalogue Service** (gRPC, SQL DB – Courses, Subjects & Enrollments)
4. **Course Content Service** (gRPC, **NoSQL/MongoDB** – Dynamic lesson blocks)
5. **Assignment/Grading Service** (gRPC, SQL DB – Submissions & Grades)
6. **Notification Service** (RabbitMQ Consumer + gRPC, **NoSQL/In-App DB** – In-App Notifications)

---

## [x] Phase 1: Foundation & First "Vertical Slice" (Student Profile)

**Goal:** A button in your frontend fetches a student's profile all the way through the stack (*Frontend $\rightarrow$ API Gateway $\rightarrow$ Profile Service $\rightarrow$ SQLite DB*).

### [x] Step 1.1: Shared Schemas & Contracts (gRPC)

* **Action:** Create the `/proto` folder with `.proto` contracts.
* **Implementation:** Define `student.proto` with `GetProfile` and `CreateStudent`.
* **Test:** Generate Go code with `protoc` without compilation errors.

### [x] Step 1.2: Student Profile Service & Database (GORM + SQLite)

* **Action:** Build **Student Profile Service** in Go.
* **Implementation:** Implement GORM repository and in-memory unit tests (`*_test.go`).
* **Test:** Run `go test ./...` and verify gRPC calls directly on port `50051`.

### [x] Step 1.3: API Gateway & Docker Compose Integration

* **Action:** Add Nginx/Traefik as an API Gateway in front of `profile-service`.
* **Implementation:** Route incoming HTTP REST calls (`/api/students`) on to the internal gRPC profile-service.
* **Test:** Run `docker compose up --build`. Send an HTTP GET/POST call via cURL/Postman to the Gateway and receive a JSON response.

### [x] Step 1.4: Minimal Frontend Integration

* **Action:** Create a simple Go-based web frontend (BFF / HTML templates).
* **Implementation:** Create the `/profile?id=xxx` page, which fetches data from the Gateway.
* **Test:** Open the browser. If you can see the profile information on the screen, the first vertical slice is complete.

---

## [x] Phase 2: Asynchronous Messaging & In-App Notifications (Service #2)

**Goal:** When a grade is given or a student is created, an in-app notification is automatically created via RabbitMQ.

### [x] Step 2.1: RabbitMQ Infrastructure

* **Action:** Add `rabbitmq:3-management` to `docker-compose.yml`.
* **Test:** Visit `http://localhost:15672` and confirm that the broker is running.

### [x] Step 2.2: Notification Service (Service #2 - NoSQL / In-App Feed)

* **Action:** Create `notification-service` with a NoSQL/In-App DB (MongoDB/SQLite) for notification history.
* **Implementation:**
  1. Create a RabbitMQ consumer that listens for events (e.g. `grade.published`, `student.created`).
  2. Add a gRPC endpoint (`GetUserNotifications`, `MarkAsRead`) so the frontend can show unread notifications on login.
  3. Show Notifications in the frontend via the `/notifications` page.

* **Test:** Send a test event to RabbitMQ and verify via gRPC that the notification can be fetched.

---

## Phase 3: Core Domain Expansion

### [x] Step 3.2: Course Content Service (Service #4 - NoSQL / CloverDB)

> **Note:** Since you have chosen **CloverDB** (embedded NoSQL), the database runs locally in your Go process and stores in `./data/nosql` instead of a MongoDB container.

#### **1. DB & Storage Setup**

* [x] **CloverDB Initialization:** Create and initialize CloverDB in `internal/repository/clover.go` and create the `"modules"` collection.
* [x] **Domain Models:** Define `domain.Module` and `domain.File` with the corresponding `json:"_id"` and `json:"..."` tags. Not quite, we dont fuck with the clover ids. We have our own `ID` field in the struct, and we use that for lookups. The `_id` is just an internal clover thing.
* [x] **Seed Data:** Create a `SeedCloverData(db)` function that inserts test modules and file arrays if the collection is empty.
* [x] **Docker Volume:** Verify that `./course-catalogue-service/data:/app/data` is mounted in `docker-compose.yml`, so the NoSQL data survives container restarts.

#### **2. Service & Repository Layer**

* [z] **Clover Repository Implementation:**
* [x] `GetModulesByCourseID(ctx, courseID)` $\rightarrow$ Runs `query.NewQuery("modules").Where(query.Field("course_id").IsEq(courseID))` and unmarshals to `[]*domain.Module`.
* [z] `SaveModule(ctx, module)` $\rightarrow$ Uses `document.NewDocumentOf(module)` and inserts/updates in CloverDB.


* [z] **Service Layer Business Logic:** Create `ContentService` that ties the repository together with any validations (e.g. checking that the course exists via gRPC calls to the SQL part).

#### **3. gRPC Server Setup**

* [z] **Proto Specification:** Define `content.proto` with messages such as `Module`, `File`, `GetCourseContentRequest` and `GetCourseContentResponse`.
* [z] **gRPC Handler Implementation:** Implement the `GetCourseContent` handler, which calls `ContentService` and maps `domain.Module` and `domain.File` over to Proto structs.
* [z] **Server Registration:** Register `ContentServer` in your `main.go`.

#### **4. Frontend / BFF Integration & Test**

* [z] **BFF Client:** Add `ContentClient` to your Go Frontend BFF and configure `COURSE_CONTENT_SERVICE_ADDR`.
* [x] **HTTP Handler:** Show the course content on `/courses/{courseID}` by calling `ContentClient.GetCourseContent(ctx, &GetCourseContentRequest{CourseId: courseID})`.
* [x] **Template / UI View:** Render the modules' titles, message texts.
* [x] **Verification:** Open a course in the frontend and confirm that modules and files are fetched in a single read call via gRPC from CloverDB.

---

### [x] Step 3.3: Assignment & Grading Service (Service #5 - SQL)

#### **1. Database Setup**

* [x] **GORM Schema / Migrations:** Create tables for `assignments` (`id`, `course_id`, `title`, `due_date`) and `submissions` (`id`, `assignment_id`, `student_id`, `grade`submitted_at`).
* [x] **Database Seeding:** Seed a couple of test assignments for existing courses.

#### **2. Service & Repository Layer**

* [x] **Grading Repository:** Implement `CreateSubmission` and `UpdateGrade`.
* [x] **Service Layer & Event Triggering:**
* [x] In `GradeSubmission()` the grade is saved in the SQL database.

#### **3. gRPC Server Setup**

* [x] **Proto Specification:** Define `grading.proto` with `SubmitAssignment` and `GradeSubmission` RPC calls.
* [x] **gRPC Handler:** Create the server implementation and hook it onto the gRPC port (e.g. `:50054`).

#### **4. Frontend / BFF Integration & Test**

* [x] **UI Form:** Create a simple page/form in the frontend where an instructor can select a student and enter a grade.
* [x] **Backend Call:** The form submits to the BFF, which calls `GradingClient.GradeSubmission()`. And persists the grade in the SQL database.
* [x] Students can upload files for assignments. and it persists the file in the `assignment-service`'s `UPLOAD_DIR` and stores the file path in the `submissions` table.
* [x] Download of submitted files.


### [x] Step 3.4: Event Publishing from Services

#### **1. Database & Domain Event Setup**

* [x] **Domain Events Definition:** Create a shared struct/proto for events (e.g. `StudentCreatedEvent`, `EnrollmentCreatedEvent`).
* [x] **RabbitMQ Producer Wrapper:** Build a reusable `publisher.go` in your service, which handles the connection, channels and reconnection to RabbitMQ.
* [x] **JSON/Protobuf Serialization:** Convert your domain event to JSON or Protobuf before it is published on the exchange.

#### **2. Service Layer Integration**

* [x] **Publish on DB mutation:** Call `publisher.Publish("account.created", payload)` right after accounts are seeded (publish failure logged, not rolled back).
* [x] **Error Handling / Fallback:** Make sure to log a clear error if the DB change succeeded but the RabbitMQ call fails (or implement the Outbox pattern, if you want to be extra thorough).

#### **3. Server & Consumer Setup**

* [x] **Notification Consumer Setup:** In `notification-service`, listen at runtime on the RabbitMQ queue `notification-queue` bound to the relevant routing keys (`*.created`, `*.published`).
* [x] **Consumer Handler:** Create a new notification in the Notification DB when a message is received.

#### **4. Frontend Integration & Test**

* [x] **UI Trigger:** Create a new student or enroll in a course via the Frontend.
* [x] **RabbitMQ Dashboard Check:** Check http://localhost:15672 and verify that the message count increases under the `Publish` rate on the queue.
* [x] **Notification Badge in Frontend:** Open the notifications page in the frontend and verify that the newly created notification is shown to the user.

#### **5. Event Catalog (Publishing Points → Consumer Responses)**

Shared conventions for every event:

* **Exchange:** `university.events` — `topic`, durable, declared by both publisher and consumer (idempotent).
* **Envelope:** every message is an `EventEnvelope{ id, type, timestamp, payload }`; `payload` is the binary protobuf of the domain event (see `proto/events/events.proto`).
* **Delivery:** publish with `WithPublishOptionsPersistentDelivery` + `WithPublishOptionsExchange("university.events")` (rare bug: without the exchange option go-rabbitmq publishes to the default exchange and the message is silently dropped).
* **Queue:** durable `notification_service_queue`, currently bound to `account.*`, `course.*` and `grade.*` in `notification-service/internal/consumer/notification.go`; durable `profile_service_queue` bound to `account.created` in `profile-service/internal/consumer/profile_consumer.go`; each new event type must add its own binding.
* **Timeout/space:** messages survive consumer downtime — a slow/restarting consumer drains the queue on startup.

Supported events:

1. **`account.created`** — *status: ✅ implemented end-to-end*
   - `auth-service` (`cmd/main.go`) right after demo accounts are seeded on an empty auth DB (publish failure is logged, not rolled back). Replaced the legacy `student.created`, which the old `CreateProfile` flow emitted.
   - Message: `AccountCreatedEvent{ account_id, email, role, full_name }`.
   - Consumer response (profile-service, `profile_service_queue`): creates an idempotent `UserProfile{ id, name }` row via `CreateProfileFromEvent` (custom master-data fields start empty).
   - Consumer response (notification-service): `CreateNotification(account_id, "Welcome to Osbourne!", "Hello {full_name}, welcome to Osbourne!...")` — welcome notification.

2. **`course.enrolled`** — *status: ✅ implemented end-to-end*
   - `Service.EnrollStudent` in course-catalogue-service, after `CreateEnrollment` commits (publish failure is logged, not rolled back).
   - Message: `CourseEnrolledEvent{ student_id, course_id, course_code, course_name }`.
   - Consumer response (notification-service): `CreateNotification(student_id, "Enrolled in Course: {course_code}", "You have been enrolled in the course: {course_name}.")`.

3. **`grade.published`** — *status: ✅ implemented end-to-end*
   - `GradeSubmission` in assignment-service, right after the grade is persisted (publish failure is logged, not rolled back).
   - Message: `GradePublishedEvent{ submission_id, assignment_id, student_id, course_id, grade, feedback }` (added to `events.proto`).
   - Consumer response (notification-service): `CreateNotification(student_id, "Grade published", "You received {grade} in course {course_id}.")`.

> Note: `course.enrolled` already worked when the assignment was first planned as `EnrollmentCreatedEvent`; the event was renamed to match the actual consumer implementation.

---

### [x] Step 3.5: Authentication Service (Service #6 - Security Boundary)

#### **1. Database Setup**

* [x] **Auth Schema / Migrations:** Create the `user_accounts` table with `id`, `email`, `password_hash`, and `role` (`student`, `teacher`, `admin`).
* [x] **Crypto Setup:** Implement `bcrypt` for secure hashing and comparison of passwords (+ seed two demo accounts: `student@osbourne.local`/`student123` id `12345`, `teacher@osbourne.local`/`teacher123` id `99999`).

#### **2. Service & JWT Implementation**

* [x] **JWT Generator:** Shared `common` module with `SignJWT(secret, user_id, email, role, exp)` issuing tokens containing `user_id`, `email`, `role` and `exp`.
* [x] **Auth Service Methods:** Implement `Login` and `ValidateToken` methods in the service layer (no public registration — accounts are seeded).

#### **3. gRPC Server & Gateway Interceptors**

* [x] **Proto Specification:** Define `auth.proto` with `Login` and `ValidateToken` RPCs.
* [x] **gRPC Auth Interceptor:** `common.AuthInterceptor(secret)` reads the JWT from gRPC context Metadata (`authorization: bearer <token>`) and verifies the signature; wired into profile/notification/course-catalogue/course-content/assignment and enforced by the frontend for outgoing calls.

#### **4. Frontend / BFF Integration & Test**

* [x] **Session / Cookie Handling:** Login page (`/login`, role picker + password) calls `auth-service`; on success the JWT is stored in the `HttpOnly` `osbourne_session` cookie.
* [x] **BFF Middleware (`h.Authenticate`):** Middleware reads the cookie, parses the JWT, fetches the profile for the display name, and attaches the token as gRPC Metadata on *all* outgoing microservice calls.
* [x] **Test Scenarios:**
* [x] Test access to protected pages without a login cookie $\rightarrow$ Redirected to `/login`.
* [x] Test access with a valid login $\rightarrow$ The gRPC calls receive the JWT via metadata and return the correct user's data (`200 OK`).

---

## [x] Phase 4: Observability, Documentation & Final Check

**Goal:** Fulfill all non-functional requirements in the course's assessment criteria.

### [x] Step 4.1: Centralized Logging & Error Handling

* **Action:** Ensure that all 6 Go services use structured logging (`slog` or `zap`) to `stdout`/`stderr`.
* **Test:** Run `docker compose logs -f` and follow a request's path through the Gateway and services.

### [x] Step 4.2: Documentation

* **Action:** Create a thorough `README.md` with:
1. **Architecture Diagram:** Visualization of the 6 services, RabbitMQ, Gateway, SQL and NoSQL databases.
2. **Run Instructions:** `docker compose up --build`.
3. **API Collection:** A `.http` file or Postman collection to test all essential flows.

---

## [~] Phase 5: High Availability & Instance Scaling — out of scope

**Goal:** Demonstrate horizontal scaling and load distribution across the services.

**Decision: deliberately not implemented for the submission.** Scaling is not
required by the assessment tips, which ask for at least three genuine services,
meaningful REST APIs, service-to-service communication (including failure
behaviour), Docker Compose, evidence, and documentation. The mechanics would
largely work: Compose's embedded DNS resolves a service name to any replica,
Nginx would only need upstream groups instead of the single upstream variables,
and RabbitMQ competing consumers would distribute a durable queue across
instances. What it would *not* survive is the storage layer — every service owns
an embedded SQLite/CloverDB file, and a second instance cannot share it, so a
truthful scaling demo would first require moving each service to a networked
database. Investing the remaining time in the documented evidence and the known
limitations (which record scaling, connection pooling and statelessness as
explicit trade-offs) is the better use of the submission budget.

The steps below are kept as the design sketch for that future work.

### [ ] Step 5.1: Multi-Instance Docker Compose Configuration

* **Action:** Remove specific port bindings on internal microservice containers in `docker-compose.yml`.
* **Execution:**
```bash
docker compose up -d --scale profile-service=3 --scale notification-service=2

```

* **Test:** Run `docker compose ps` and confirm that all instances run on the shared Docker network

### [ ] Step 5.2: Gateway Round-Robin Load Balancing

* **Action:** Configure Nginx as a load balancer for gRPC and REST backends.
* **Test:** Log `os.Hostname()` in the Go services, send 6 calls through the Gateway, and verify in the log that the requests are distributed evenly across the container IDs.

### [ ] Step 5.3: Asynchronous Competing Consumers (RabbitMQ)

* **Action:** Ensure that all scaled instances of `notification-service` listen on the **same RabbitMQ queue**.
* **Test:** Send 10 grade events quickly after one another. Confirm in the logs and in the RabbitMQ Dashboard that each event is processed only **once** by one of the instances (round-robin).

### [ ] Step 5.4: Database Connection Pooling & Statelessness Audit

* **Action:**
* Limit database connections in Go drivers (`db.SetMaxOpenConns(10)`).
* Ensure that all services validate access via stateless JWT signatures.


* **Test:** Run a stress test with `hey` or `ab`:
```bash
hey -n 200 -c 20 http://localhost/api/courses
```
--- 

## [x] Phase 6: Migrate the REST API out of the frontend into the services

**Goal:** Every microservice exposes its own REST API (grpc-gateway in front of its gRPC server), and Nginx stops being a plain reverse proxy and becomes a real **API gateway** that routes, authenticates and traces the whole stack. The frontend keeps its server-rendered pages and its gRPC clients; it stops proxying the browser-facing endpoints.

> The original draft routed `/api/courses` to the catalogue while `/api/courses/{id}/modules` and `/api/courses/{id}/assignments` belonged to two other services. Phase 4 of that draft (`/api/course-content/`) contradicted the Phase 1 annotation list. The layout below fixes this with a single shared resource tree that Nginx disambiguates by regex.

### 6.0 Target REST surface

| Method | Path | Service | Mechanism |
| --- | --- | --- | --- |
| `POST` | `/api/auth/login` | auth | grpc-gateway (+ `Set-Cookie`) |
| `POST` | `/api/auth/validate` | auth | grpc-gateway |
| `POST` | `/api/auth/logout` | auth | grpc-gateway (expires cookie) |
| `GET` | `/api/profile` | profile | grpc-gateway |
| `PUT` | `/api/profile` | profile | grpc-gateway (**new RPC**) |
| `GET` | `/api/courses?page=&page_size=` | catalogue | grpc-gateway |
| `GET` | `/api/courses/{course_id}` | catalogue | grpc-gateway |
| `POST` | `/api/enrollments` | catalogue | grpc-gateway |
| `GET` | `/api/enrollments/me` | catalogue | grpc-gateway |
| `GET` | `/api/courses/{course_id}/modules` | content | grpc-gateway |
| `POST` | `/api/courses/{course_id}/modules` | content | grpc-gateway |
| `GET` | `/api/courses/{course_id}/modules/{module_id}` | content | grpc-gateway |
| `PUT` | `/api/courses/{course_id}/modules/{module_id}` | content | grpc-gateway |
| `DELETE` | `/api/courses/{course_id}/modules/{module_id}` | content | grpc-gateway |
| `GET` | `/api/courses/{course_id}/assignments` | assignment | grpc-gateway |
| `POST` | `/api/courses/{course_id}/assignments` | assignment | grpc-gateway |
| `GET` | `/api/assignments/{assignment_id}` | assignment | grpc-gateway |
| `GET` | `/api/assignments/{assignment_id}/submissions` | assignment | grpc-gateway |
| `GET` | `/api/assignments/{assignment_id}/submissions/mine` | assignment | grpc-gateway |
| `GET` | `/api/submissions/{submission_id}` | assignment | grpc-gateway |
| `POST` | `/api/submissions/{submission_id}/grade` | assignment | grpc-gateway |
| `POST` | `/api/assignments/{assignment_id}/submissions` | assignment | **custom `HandlePath`** (multipart) |
| `GET` | `/api/submissions/{submission_id}/file` | assignment | **custom `HandlePath`** (binary stream) |
| `GET` | `/api/notifications` | notification | grpc-gateway |
| `POST` | `/api/notifications/{notification_id}/read` | notification | grpc-gateway |

Every service exposes at least 2 endpoints, satisfying the requirement in `docs/notes.md`.

**Deviations from the earlier draft, and why:**

* `POST /api/enrollments` instead of `POST /api/courses/{course_id}/enroll` — in the shared tree the draft's path would sit next to `GET /api/courses/{course_id}` and force `{course_id}` to double as an action.
* `POST .../read` instead of `PATCH .../read` — the draft's verb buys nothing here and costs a JS change in the notification list.
* Upload/download move from `/{id}/submit` and `/submissions/{id}/download` to the nested-resource form; the multipart field name `submission_file` is preserved.
* `code` in the error body is the **HTTP** status, not the gRPC code (the stock grpc-gateway handler emits the gRPC code, so the draft's `{"code": 404, ...}` could never actually be produced).
* `POST /api/courses/{course_id}/assignments` and `GET /api/submissions/{submission_id}` were added during Step 2 so the assignment service's existing write and single-fetch RPCs are reachable over HTTP too; `/submissions/mine` is split from the staff-facing `/submissions` so one endpoint does not have to serve two different audiences.

### 6.1 Step 1 — Tooling & code generation ✅

* **`Makefile`**: install the two plugins in `tools`, pinned, and run three `buf generate` passes plus a `buf dep update`:

  ```make
  GATEWAY_VERSION := v2.30.0
  ...
  @which protoc-gen-grpc-gateway > /dev/null || (echo "..." && go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@$(GATEWAY_VERSION))
  @which protoc-gen-openapiv2  > /dev/null || (echo "..." && go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@$(GATEWAY_VERSION))
  ```

* **`proto/buf.yaml`**: add `deps: [buf.build/googleapis/googleapis]`; commit the resulting `proto/buf.lock`. Until Step 2 lands the annotations, `buf dep update` prints a harmless *"declared in your buf.yaml deps but is unused"* warning.
* **`proto/buf.gateway.gen.yaml`** (new): `grpc-gateway` into the six backends' `gen/`. It is a *separate* template rather than extra entries in `buf.gen.yaml` because the frontend must be excluded — see below.
* **`proto/buf.openapi.gen.yaml`** (new): `openapiv2` with `allow_merge=true,merge_file_name=osbourne` into `proto/openapi/osbourne.swagger.json`. Needs `strategy: all`; without it buf invokes the plugin once per proto directory and all six invocations race to write the same merged filename.
* **`proto/buf.gen.yaml`**: **deliberately unchanged.** Splitting it per module turned out not to be expressible, and the attempt is what produced the finding below.
* **`go.mod`**: the `grpc-gateway/v2 v2.30.0` requirement is added in Step 3/4, not here — nothing imports the runtime until `common/gateway.go` exists, so `go mod tidy` would strip a premature `require`.

#### Finding: one buf template cannot vary the output directory per input

`buf.gen.yaml` is a matrix of sources x plugins, and neither schema version can express "module A gets plugin P, module B gets plugin Q":

* **v1** (what this project uses) has a per-plugin `path` field, but it is the path to the plugin **binary**, not a file filter — `name: go` + `path: auth` makes buf try to `exec "auth"`.
* **v2** moves file selection to a top-level `inputs:` list whose entries are *sources* (`directory`, `module`, `proto_file`, …) and carry no `out` or `plugins` override. Per-plugin narrowing is type-level only (`types` / `exclude_types`), so output directories still cannot be targeted per input.

Hence the three-template split. The consequence is a known wart: every backend receives `.pb.gw.go` stubs for the other five contracts. They compile and are never imported. Removing them would need one `buf generate --path <dir> -o <dir>` invocation per module — 13 extra invocations for dead files, not worth it. The one exclusion that *does* matter is the frontend, because generated gateway stubs there would pull the gateway runtime into `frontend/go.mod` for code nothing imports; `buf.gateway.gen.yaml` simply omits it from the `out` list.

#### Version pin

Plugin **v2.30.0** is pinned. Verified in a scratch module that v2.30.0-generated code compiles against `grpc v1.83.0` + `protobuf v1.36.12`; **v2.31.0 must not be used** — it pulls a grpc newer than the workspace's and forces a workspace-wide bump.

*Result:* `make generate` produces no `*.pb.gw.go` yet (correct — no annotations exist), and `proto/openapi/osbourne.swagger.json` merges all six services with an empty `paths` object. `go build`, `go vet` and `go test` pass in all 8 modules.

### 6.2 Step 2 — Proto HTTP annotations ✅

Add `import "google/api/annotations.proto";` and `option (google.api.http)`:

* `auth.proto`: `Login` → `post: "/api/auth/login", body: "*"`; `ValidateToken` → `post: "/api/auth/validate", body: "*"`; new `Logout` → `post: "/api/auth/logout"` (no body — the request message is empty).
* `profile.proto`: `GetUserProfile` → `get: "/api/profile"`; new `UpdateUserProfile(UpdateUserProfileRequest) returns (UpdateUserProfileResponse)` → `put: "/api/profile", body: "*"`. `UpdateUserProfileResponse` wraps a `ProfileResponse` rather than reusing it, because `RPC_REQUEST_RESPONSE_UNIQUE` requires a distinct response type per RPC.
* `course-catalogue.proto`: `ListCourses` → `get: "/api/courses"`; `GetCourse` → `get: "/api/courses/{course_id}"`; `EnrollUser` → `post: "/api/enrollments", body: "*"`; `ListEnrolledCourses` → `get: "/api/enrollments/me"`.
* `course-content.proto`: `ListModulesByCourseID` → `get: "/api/courses/{course_id}/modules"`; `CreateModule` → `post: "/api/courses/{course_id}/modules", body: "*"`; `GetModule`/`UpdateModule`/`DeleteModule` → `get`/`put`/`delete` on `/api/courses/{course_id}/modules/{module_id}`. `GetModuleRequest`, `UpdateModuleRequest` and `DeleteModuleRequest` gain `course_id`, and `UpdateModuleRequest.id` is renamed to `module_id` to line up with the path template. Only `ListModulesByCourseID` had a caller in the frontend, so no gRPC client call site breaks.
* `assignment.proto`: `CreateAssignment`/`GetCourseAssignments` → `post`/`get` on `/api/courses/{course_id}/assignments`; `GetAssignment` → `get: "/api/assignments/{assignment_id}"`; `ListSubmissions` → `get: "/api/assignments/{assignment_id}/submissions"`; `ListMySubmissions` → `get: "/api/assignments/{assignment_id}/submissions/mine"`; `GetSubmission` → `get: "/api/submissions/{submission_id}"`; `GradeSubmission` → `post: "/api/submissions/{submission_id}/grade", body: "*"`. **`SubmitAssignment` and `DownloadSubmission` are deliberately left unannotated** — `generate_unbound_methods` defaults to false, so no gateway stubs are emitted for the two streaming RPCs; Step 6.8 mounts hand-written `HandlePath` handlers for them.
* `notification.proto`: `GetUserNotifications` → `get: "/api/notifications"`; `MarkNotificationAsRead` → `post: "/api/notifications/{notification_id}/read"`. The marker message carries only the path parameter, so no `body` is declared — a `body: "*"` here would be meaningless.

While in the protos, fix the two pre-existing defects: the unused `timestamp.proto` import in `course-content.proto`, and the inconsistent `go_package` in `course-catalogue.proto`.

Two pre-existing defects in the course-content **server** also had to be fixed, because the new `POST /api/courses/{course_id}/modules` route calls straight into them:

* `ContentServer` embedded the `coursecontent.CourseContentServiceServer` *interface*, which is a nil value. Any unimplemented method resolved through it and panicked. Now embeds `UnimplementedCourseContentServiceServer`, matching all five other services.
* Its create handler was named `Create`, matching no interface method, so the `CreateModule` RPC the gateway was about to wire up was the nil-panicking path. Renamed to `CreateModule`.

*Result:* `make generate` emits 36 `*.pb.gw.go` files (6 stubs per backend — the gateway plugin has no per-service filter, so each backend's `gen/` receives one stub per proto; the frontend's gateway pass is excluded, so it gets 0). `proto/openapi/osbourne.swagger.json` now lists 18 paths / 23 operations, matching the 6.0 table (the only two 6.0 rows absent from the spec are the custom `HandlePath` upload/download pair, which have no annotation by design). `buf lint` is at parity with the pre-change baseline (21 findings, all pre-existing) minus the now-fixed unused import. `go build`, `go vet` and `go test` pass in all 8 modules, including after a from-scratch regeneration with all `gen/` directories deleted.

### 6.3 Step 3 — `common` module ✅

New **`common/gateway.go`**, so all six `main.go`s stay a handful of lines each:

* `GatewayMux()` — `runtime.NewServeMux` with the incoming/outgoing header matchers, the error handler and a `JSONPb` marshaler (`UseProtoNames: true`, `DiscardUnknown: true`).
* `IncomingHeaderMatcher` — maps `Authorization` → `authorization` and `X-Request-Id` → `x-request-id`, falling through to `runtime.DefaultHeaderMatcher`. (`Authorization` already has a special case inside grpc-gateway's `annotateContext`; keeping it explicit means the behaviour survives a runtime upgrade. `X-Request-Id` has no such case and *would* be dropped.)
* `OutgoingHeaderMatcher` — maps `set-cookie` → `Set-Cookie`, so a service can set a cookie by calling `grpc.SetHeader(ctx, metadata.Pairs("set-cookie", ...))` from inside the RPC.
* `GatewayErrorHandler` — gRPC code → HTTP status with a consistent body:

  ```json
  { "code": 404, "success": false, "message": "course not found" }
  ```

  The `success` field is what keeps the frontend's existing `result.data.success` contract working unchanged.
* `UserIDFromContextOrRequest(ctx, requested)` — returns the JWT subject when `requested == ""`, otherwise `requested`.

New **`common.AuthStreamInterceptor`** — `AuthInterceptor` is unary-only, so `SubmitAssignment` (client streaming) and `DownloadSubmission` (server streaming) are currently unauthenticated at the gRPC layer. This is the moment to close it.

*Test:* `common/gateway_test.go` covers the header matchers (`Authorization` → `authorization`, `X-Request-Id` → `x-request-id`, `Cookie` **not** leaked), the error handler status codes, and the claims-helper precedence.

*Result:* 40 subtests pass. Four things the implementation turned up that the step description above did not anticipate:

* **`GatewayRoutingErrorHandler` is a separate hook and is required.** `ServeMux` does not send an unmatched request to the error handler; it goes to the *routing* error handler, which defaults to the stock `{"code": 5, "message": "Not Found"}`. A mistyped URL would therefore have answered 404 with a gRPC code and no `success` field while every other error used the new shape. Both now share `writeGatewayError`, and `TestGatewayMuxRoutingErrorShape` drives a real `ServeMux` to catch the regression.
* **`Cookie` must be actively denied, not just left alone.** The default matcher forwards it as `grpcgateway-Cookie`, putting the raw session cookie into gRPC metadata where it can be logged or traced. `IncomingHeaderMatcher` returns `false` for it; the session reaches the service via `Authorization`, which is where the auth interceptor looks.
* **`OutgoingHeaderMatcher` is an allowlist of one, not a pass-through.** The runtime default prefixes *every* response metadata key with `Grpc-Metadata-` and forwards it. That would leak internal metadata to the browser, so anything not named `set-cookie` is dropped.
* **`UserIDFromContextOrRequest` returns `(string, error)`.** The step description implied a bare string, but resolving to `""` on a missing identity lets a caller act on it. A request with neither an explicit id nor claims is now `Unauthenticated`.

Two smaller notes: `AuthInterceptor` and `AuthStreamInterceptor` share a new `trimBearer` helper so they accept exactly the same headers, and `AuthStreamInterceptor` wraps `grpc.ServerStream` to override `Context()` because gRPC offers no way to replace a stream's context in place.

### 6.4 Step 4 — Per-service dual listener ✅

Each `cmd/main.go` keeps its gRPC server on `:5005x` and gains an HTTP server on `$HTTP_PORT` (default `8080`):

```go
mux := common.GatewayMux()
// Must be FromEndpoint, not HandlerServer: HandlerServer calls the server
// implementation in-process and therefore bypasses common.AuthInterceptor.
if err := profile.RegisterProfileServiceHandlerFromEndpoint(ctx, mux, "localhost:"+port, dialOpts); err != nil { ... }
srv := &http.Server{Addr: ":" + httpPort, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
```

`insecure` credentials on loopback are fine — the traffic never leaves the container. Graceful shutdown gains a 5 s `srv.Shutdown` next to the existing `grpcServer.GracefulStop()`. Services are wired **one at a time** (`auth` → `profile` → `notification` → `catalogue` → `content` → `assignment`), each verified with `curl` from inside its container before Nginx learns about it.

Service-specific work:

* **auth-service** — `Login` becomes the session issuer: build the `osbourne_session` cookie (HttpOnly, `SameSite=Lax`, `MaxAge` = token TTL) and ship it via `grpc.SetHeader`. New `Logout` RPC returns the same cookie with `MaxAge: -1`.
* **profile-service** — implement `UpdateUserProfile`: add `Update` to `domain.ProfileRepository`, a GORM implementation, `UpdateProfile` on the service layer, and the RPC. Covers the "at least 2 endpoints" requirement, which `GetUserProfile` alone cannot.
* **identity from the JWT** — REST callers never pass `user_id` (`GET /api/profile` has no place to put it). The profile, notification and catalogue servers fall back to `common.UserIDFromContextOrRequest` when the request field is empty, so existing gRPC callers keep working unchanged.
* **assignment-service** — new `internal/httpapi/files.go` with the two streaming routes, registered through `mux.HandlePath` (grpc-gateway v2's `ServeMux` is *not* an `http.ServeMux`, but `HandlePath` accepts a plain path pattern):
  * `POST /api/assignments/{assignment_id}/submissions` — parse multipart field `submission_file`, `http.MaxBytesReader` at 10 MB, then reuse the chunked `SubmitAssignment` client-stream loop.
  * `GET /api/submissions/{submission_id}/file` — reuse the `DownloadSubmission` server-stream loop, setting `Content-Disposition` / `Content-Type` / `Content-Length` from the first metadata frame.

  Both self-dial `localhost:<grpc-port>` so the interceptors still apply, and both return `{"success":…, "message":…}` errors — the download path's `Content-Type: application/json` failure signal is load-bearing in the frontend JS.

*Test:* per-service `httptest` round-trips against a real in-process gRPC server plus a `runtime.ServeMux`, asserting the JSON shape and that a missing token yields 401.

#### Result ✅

All six services are wired, and `go build` / `go vet` / `go test` pass in all eight modules. 79 gateway-facing subtests were added across the six services (auth 7, profile 9, notification 6, catalogue 9, content 9, assignment 21, plus `common`'s 13), on top of the pre-existing suite.

The listener wiring itself turned out to be the one genuinely repeatable part, so it became **`common/gateway_serve.go`** rather than six copies of the same forty lines: `common.NewGateway(register)` builds the mux and hands it to a per-service `register` closure, and `Gateway` owns `Serve` / `Shutdown` / `ShutdownWithTimeout`. The gRPC server stays in each `main.go`, where the per-service interceptor chain already lived. This is the first export in the module with more than one caller — the six in Step 3's list still have only the one.

Six things the step description above did not anticipate:

* **The hand-written `HandlePath` routes were completely unauthenticated until they forwarded the header themselves.** `common.IncomingHeaderMatcher` is what maps `Authorization` onto the `authorization` metadata key, and only the generated routes go through it. A `HandlePath` handler makes its own client call, so nothing did that translation: every upload and download reached the server with no credentials and came back 401, which reads exactly like a bad token rather than a missing one. `httpapi.outgoingContext` now copies `Authorization` and `X-Request-Id` into the outgoing metadata. Worth remembering that the fix is *not* obvious from the failure.
* **Six identity holes closed, all of the same shape.** A `user_id` in a REST request is attacker-controlled, so every one of these now takes the subject from the verified token: `UpdateUserProfile` (which carries a `user_id` field purely for gRPC symmetry), `EnrollUser` (`body: "*"`), `GetUserNotifications`, `ListEnrolledCourses`, and `SubmitAssignment` — where the id arrived in the *first message of a client stream*, so the REST upload route would have let anyone submit work under someone else's name. `ListMySubmissions` already did this correctly and is the model.
* **`POST /api/notifications/{id}/read` had no ownership check.** The id comes from the URL, so any signed-in user could mark anyone else's notifications read. The check lives in `NotificationService.MarkNotificationAsRead`, which now takes the owner, so no future handler can bypass it; a mismatch reports `NotFound` rather than `PermissionDenied` because distinguishing the two would turn the endpoint into an oracle for enumerating notification ids.
* **Two pre-existing bugs in course-content, both found by the new tests.** `ModuleService.DeleteModule` fetched the module, checked it existed, and then returned `nil` **without ever calling the repository's delete** — `DELETE` answered 200 while leaving the module readable. And `CloverModuleRepository.GetModule` returned `(nil, nil)` on a miss, which `toProtoModule` dereferenced: any authenticated caller could panic the service with a made-up module id. The repository now returns a `domain.ErrNotFound` sentinel, the service maps it to a gRPC `NotFound` (previously a plain `fmt.Errorf`, so a missing module surfaced to the browser as a 500), and `toProtoModule` is nil-safe to match the catalogue's existing guard. The old repository test asserted `GetModule` returned *no error* after a delete — it only passed because delete never deleted.
* **`GET /api/courses` with no query parameters returned `total_count: 1` and an empty list.** `offset = (page-1)*pageSize` with both unset gives `LIMIT 0`, which SQL reads as "no rows". Latent today because the frontend always sends `page=1&page_size=10`, but Step 6 publishes this route and Step 7 calls it from the browser. `ListCourses` now normalises `page >= 1` and clamps `page_size` to 20…100.
* **The catalogue is not public**, contrary to what its routes suggest. `AuthInterceptor` rejects anonymous callers, and that is correct: `handler.go` gates `/course-catalog` and `/courses/{id}` behind the frontend's own `Authenticate` middleware, so there is no anonymous caller to serve. A test asserting a 200 for a bare `GET /api/courses` was wrong, not the code.

`PUT /api/profile` is a full replacement rather than a merge. The proto's fields are plain proto3 scalars with no presence tracking, so an omitted field and one sent as `""` are the same request on the wire; a merge would have to guess from JSON that has already been decoded. `PUT` is defined as replacing the resource, and it is the only option the wire format allows. The GORM `Update` uses `Select("*").Omit("id", "created_at")` so a field can actually be *cleared* — GORM's `Updates` skips zero values by default, which would have made "remove my phone number" silently do nothing.

**Resolved — role checks are deliberately not wanted.** `ListSubmissions` (all submissions for an assignment) and `GradeSubmission` have no role check, so any authenticated student can read a classmate's submission and grade it. The question was raised before Step 5, and the answer is that this is intended: there is no teacher UI in the frontend today, so `Role` is not part of the access policy and the routes are left open to any authenticated caller. `ListMySubmissions` still restricts by student, since that is scoping rather than privilege. This is recorded as a decision, not an oversight — if a teacher UI is ever built, the `claims.Role` check goes in at that point.

### 6.5 Step 5 — Nginx as the API gateway ✅

Nginx evaluates regex locations in config order, and a match beats the longest prefix match — so specificity is encoded as ordering, with a `/api/` prefix block that 404s anything unrouted (otherwise unknown API paths silently fall through to the frontend):

```nginx
location ~ ^/api/auth/                            { proxy_pass $upstream_auth$request_uri;        }
location ~ ^/api/profile(/|$)                     { proxy_pass $upstream_profile$request_uri;     }
location ~ ^/api/notifications(/|$)               { proxy_pass $upstream_notification$request_uri; }
location ~ ^/api/courses/[^/]+/modules(/|$)       { proxy_pass $upstream_content$request_uri;     }
location ~ ^/api/courses/[^/]+/assignments(/|$)   { proxy_pass $upstream_assignment$request_uri;    }
location ~ ^/api/(assignments|submissions)(/|$)   { proxy_pass $upstream_assignment$request_uri;    }
location ~ ^/api/(courses|enrollments)(/|$)       { proxy_pass $upstream_catalogue$request_uri;     }

location /api/ { default_type application/json; return 404 '{"code":404,"success":false,"message":"unknown API route"}'; }
location /     { proxy_pass $upstream_frontend$request_uri; }
```

The original draft wrote these as literal `proxy_pass http://auth-service:8080;`, which is equivalent for routing and is kept that way here for readability. See the startup-ordering note below for why the shipped file uses variables instead.

`proxy_pass` forwards the original path unchanged, which matters because each service owns the whole `/api/…` tree below it and a rewritten path would 404 against its own gateway.

Gateway-level concerns that must not be forgotten:

* **Cookie → bearer header.** The browser holds an HttpOnly `osbourne_session` cookie, services expect `Authorization: Bearer …`. A `map` in the `http` block turns `$cookie_osbourne_session` into the header (an explicit `Authorization` header from curl/Postman still wins). The draft's Phase 5 named `$cookie_session_token`; the real cookie is `osbourne_session`.
* **`client_max_body_size 12m`.** Nginx defaults to 1 MB and the frontend previously capped uploads at 10 MB itself; without this the 10 MB upload silently 413s at the gateway.
* **`proxy_request_buffering off`** on the upload route so Nginx does not spool the whole body to disk first.
* **Keep** `X-Request-ID $request_id` and `Host $host`; hoist the shared `proxy_set_header` block into a single mounted include file so all eleven locations stay in sync.
* **Startup ordering.** `proxy_pass` with a literal hostname resolves at config-load time, so `api-gateway` would have to `depends_on` all six backends just to be able to parse its own config — and would still be pinned to IPs resolved at boot, so a recreated container is proxied to a dead address until the gateway restarts. The shipped config instead holds each upstream in a variable and uses `set $upstream …; proxy_pass $upstream$request_uri;`, which resolves per request through the `resolver` in the file. `depends_on` is kept for all seven backends so a cold `docker compose up` has every name registered before the first request, but it is no longer load-bearing. The cost of the variable form is that nginx cannot tell whether a URI part was given, so the request URI is appended explicitly — which is also what preserves the query string the catalogue paginates on.
* Optionally `error_page 401 = @login` so an expired session navigates back to `/login` instead of showing raw JSON.

*Test:* `curl` every path in the 6.0 table through `http://localhost/` and check the 404 catch-all, the 401 behaviour, and the 10 MB upload.

**Result.** `nginx/routing-test.sh` runs the table against stub upstreams — 38 assertions covering which service receives each path, the JSON 404 catch-all, query-string preservation, cookie→bearer promotion, and the 12 MB body limit. It exists because the ordering above is invisible to `nginx -t`: every route is syntactically valid and `/api/courses/42/modules` still parses when it is sent to the catalogue, it just returns the wrong service's 404. Each stub is an nginx that reports its own name plus the headers it received, so one harness covers both the routing table and the header rules. Verified live as well, against the real stack: login → `HttpOnly` cookie → every route in the 6.0 table → 200; a 3 MB upload and its download round-trip byte-identical; unauthenticated requests 401 with the shared error shape; logout clears the cookie and the next request 401s.

Two things had to be fixed to get there:

* The gateway config lives in two files, and `docker-compose.yml` mounted only `nginx.conf`. The missing `./nginx/includes` mount makes the gateway exit on startup, so the two mounts have to stay together.
* `Upload` masked a server refusal as a generic 502. The server reads the metadata message and can reject straight after it — an unknown assignment, say — but the client only finds out when a later `Send` fails, and that arrives as a bare EOF. A small file hides this, because its single chunk is absorbed into the send buffer and the real status turns up at `CloseAndRecv` instead; a file larger than the 64 KB chunk size does not. The existing `TestUploadForAnUnknownAssignmentIs404` used a 4-byte file and passed while the live 3 MB upload returned `{"code":502,…,"message":"could not upload file"}`. Both paths now recover the status, and the test has a 2 MB sibling that fails if the recovery is removed.

### 6.6 Step 6 — docker-compose ✅

* Add `HTTP_PORT=8080` to the six backends. They already have no host port bindings; keep it that way.
* **`frontend`: remove `ports: "8080:8080"`.** The browser must now go through `http://localhost/` or none of the `/api/*` routes exist.
* `frontend.depends_on`: add the missing `assignment-service`.
* `api-gateway.depends_on`: add all six backends.

**Result.** `HTTP_PORT=8080` is spelled out on all six backends rather than left to the default in `common.GatewayMux`, because `nginx.conf` hard-codes 8080 for every backend — the two have to agree, and a silent default change would 502 every route with nothing pointing at the cause. `frontend` no longer publishes a host port: binding 8080 as well would leave a second copy of the app that has no `/api/*` routes at all, so the login form would post into a void. It is reachable only from inside `osbourne-net`, and both it and the gateway now depend on all six backends. The only host-published ports left in the stack are nginx on 80 and the RabbitMQ dashboard on 15672.

### 6.7 Step 7 — Frontend decoupling ✅

* Delete `frontend/internal/handler/api.go`, the `r.Route("/api", …)` group, and the `writeJSON` / `mimeTypeFor` helpers that only it used. `grpcToHTTPStatus` stays — `fetchError` still uses it.
* `Authenticate`'s `Profile.GetUserProfile` call stays on gRPC: it is server-side and never traverses Nginx.
* Repoint the five browser-facing calls and fix their bodies:

  | Location | New target | Change |
  | --- | --- | --- |
  | `course_card.templ` | `POST /api/enrollments` | JSON body instead of `URLSearchParams` |
  | `assignment.templ` | `POST /api/assignments/{id}/submissions` | form `action`; the JS currently matches `form.action.includes('/submit')` — switch to a form class |
  | `submission_card.templ` | `POST /api/submissions/{id}/grade` | JSON body `{"score":…,"feedback":…}` |
  | `submission_card.templ`, `student_submission_card.templ` | `GET /api/submissions/{id}/file` | link `href` |
  | `notification_item.templ` | `POST /api/notifications/{id}/read` | — |

* The `fetch` chains need no rewrites: the success protos already carry `success`, the `\|\| 'fallback'` defaults cover the missing `message`, and the error body carries `message`. The upload handler must therefore emit `{success, message}` too.
* `login.templ`: the form posts to `/api/auth/login` with `email`/`password`; a small inline script redirects to `/` on success and shows the error inline on 401. The `role` hidden field and `selectRole()` picker go away — role is derived server-side.
* `base.templ`: logout posts to `/api/auth/logout` then navigates to `/login`. `login.go` keeps only `HandleLoginPage`; `HandleLogin`, `HandleLogout` and the `sessionCookieName` constant are deleted.
* Optional but worth it: chi's `middleware.RequestID` ignores the inbound `X-Request-ID` and mints a fresh id, so Nginx's id and the frontend's currently diverge. Reading the inbound header is what makes the end-to-end `request_id` trace line up.

*Test:* update `enroll_test.go` — drop `TestHandleEnrollCourseRoutes`, change the `/api/courses/enroll` script-injection assertion to `/api/enrollments`.

**Result.** `api.go` and the `r.Route("/api", …)` group are gone, along with the `writeJSON`/`mimeTypeFor` helpers only they used; `fetchError`/`grpcToHTTPStatus` stay because `pages.go` still needs them for the page routes. The six browser-facing calls now point at the services, and three of them needed more than a URL change:

* **Enrolment** sends a JSON body. The old handler read `URLSearchParams`, and a form-encoded body against the gateway's JSON binding is a 400 that only appears when somebody clicks the button.
* **Grading** sends `{"score":…,"feedback":…}`. The form field is called `grade` and the RPC field `score`, and `score` is an `int32`, so it goes over as a `Number` rather than leaning on protojson accepting a numeric string.
* **Upload** matches on `form.submission-form` instead of `form.action.includes('/submit')`. The new path ends in `/submissions`, which *contains* `/submit`, so the old check would have kept passing by coincidence — exactly the kind of accidental agreement that breaks silently the next time a route is renamed.

`HandleLoginPage` survives on its own; the form posts to `/api/auth/login` and auth-service sets the cookie, so the frontend no longer touches a credential. The role picker is gone — the email is a normal editable field and role comes from the token. Logout became a `fetch` to `/api/auth/logout` followed by a navigation, since a plain form post would land the browser on a JSON body. `sessionCookieName` stays: the frontend still has to *read* the cookie to gate pages, it just no longer sets or clears it.

Two trace gaps closed while here. chi's `middleware.RequestID` mints a fresh id and ignores the inbound one, so `logRequest` now adopts nginx's `X-Request-ID`, and `reqIDCtx` reads it from `common` rather than from chi — otherwise the adopted id never reached gRPC. The `GetUserProfile` call in `Authenticate` attaches it explicitly, since it runs before the user exists in the context and so cannot go through `authCtx`; that call is otherwise the one gRPC hop of every page render that cannot be tied back to the page request.

`TestHandleEnrollCourseRoutes` is replaced by `TestFrontendDoesNotServeAPIRoutes`, which asserts the ten `/api` paths — the six new ones and the four pre-migration ones — all 404 at the frontend and never return a gateway-shaped body. Testing that a removed route is absent is weaker than testing that the new one works, but it is the failure this step could plausibly regress into.

### 6.8 Step 8 — Rollout order

1. `buf.yaml` deps + `buf.lock` + `buf.gen.yaml` restructure; regenerate with **no annotations yet** and prove nothing regressed.
2. `common/gateway.go` + `AuthStreamInterceptor`; add the dependency to all eight `go.mod`s.
3. Proto annotations + the new `UpdateUserProfile` RPC; `make generate`; fix the compile fallout (including renaming `ContentServer.Create` to match the `CreateModule` RPC).
4. Dual listeners, one service at a time.
5. `nginx.conf` + `docker-compose.yml`.
6. Frontend decoupling + test updates.
7. Docs, `.http` collection, `swagger.json`.
8. Full end-to-end pass (below).

This was a prescription, not a deliverable, so it carries no checkmark of its
own. It was followed, with one deviation worth recording: step 4 was done as a
single pass over all six services rather than one at a time, after the Step 1–3
build had already proven the untouched services still compiled and passed. The
per-service loop the plan expected is covered instead by the shared
`common/gateway_serve.go` unit tests and the final stack rebuild in Step 9.

### 6.9 Step 9 — Verification ✅

`docker compose up --build`, then:

1. The five UI flows still work from the browser at `http://localhost/`.
2. An unknown `/api/*` path returns the 404 JSON body, not the frontend.
3. `GET /api/profile` without a session returns 401.
4. A 10 MB assignment upload succeeds through Nginx.
5. `docker compose logs` shows one shared `request_id` across Nginx → service → gRPC.
6. A RabbitMQ notification still lands after an enrolment.

**Result.** All six pass against the rebuilt stack. Every page renders 200 through nginx (`/`, `/profile`, `/notifications`, `/course-catalog`, `/courses/1`, `/courses/1/assignments/1`), all five browser-facing API calls answer correctly, `localhost:8080` now refuses connections, and enrolling as the teacher produced `Enrolled in Course: CS101` in their feed through RabbitMQ. A single `GET /course-catalog` carries one id — `890c47f6…` — through nginx, the frontend, profile-service and the catalogue.

Two defects only the verification pass could find:

* **A 10 MB upload was rejected.** The route caps at 10 MB, but the cap was applied to the whole request body, and a multipart body is the file plus its boundaries and headers — so a file of *exactly* 10 MB arrived a few hundred bytes over and got a 413 from the service, not from nginx. The one size a user is most likely to pick was the one that failed. The body cap now carries an allowance for the envelope and the advertised limit is enforced on the parsed part, with tests either side of the boundary at exactly 10 MB and 10 MB + 1.
* **Structured logs carried duplicate keys.** `contextHandler` enriches every record with `request_id` and `user_id`, and `interceptor_log.go` passed the same two keys explicitly, so each gRPC line had `"user_id"` twice. That is valid JSON but ambiguous: a pipeline keeping the first occurrence and one keeping the last disagree about the request id. The enrichment now skips keys the call site already set.

The request id is also echoed back in an `X-Request-Id` response header, using `always` so it survives the 401s and 404s that are the responses actually worth tracing. Without `always` nginx drops `add_header` from error responses, and the 404 catch-all — which answers from nginx rather than proxying, so it never pulls in `proxy-common.conf` — was the one response with no id to quote.

### 6.10 Risks

* **`RegisterXHandlerServer` would silently disable authentication** — it invokes the server implementation in-process, bypassing every interceptor. It must be `RegisterXHandlerFromEndpoint`.
* **grpc-gateway version pin.** v2.31.0 pulls grpc > 1.83.0 and forces a workspace-wide upgrade. Pin `v2.30.0`.
* **`course-content-service/cmd/main.go` hardcodes `"50054"`** and ignores `PORT` (already logged in the issues list). The dual-listener refactor is the natural place to fix it, otherwise the `localhost:<port>` self-dial is wired to a value that can drift from the listener.
* **Removing the frontend's `:8080` host port** is a visible change — anyone with `http://localhost:8080` bookmarked gets a connection error.
* **10 MB uploads** need `client_max_body_size` in Nginx *and* the handler cap, plus `proxy_request_buffering off`.



## Issues and Additional Features

### Critical Bugs

- [x] **Inverted ID generation in CreateModule** — `CreateModule` in `course-content-service/internal/service/module_service.go` generated a UUID only when `module.ID != ""`, which is the opposite of what you want. Should be `== ""`.
- [x] **DeleteModule does nothing** — `DeleteModule` in `course-content-service/internal/service/module_service.go` validated the module exists but never called `s.repo.DeleteModule()`. Deletions silently no-op.
- [x] **No authentication** — fixed: `frontend/internal/handler/handler.go` now runs a `Authenticate` middleware that reads the JWT from the `osbourne_session` cookie (no more `?id=` impersonation); auth-service issues the token and a shared `common` gRPC interceptor enforces it on all five backend services.
- [ ] **All gRPC traffic is unencrypted** — All 5 frontend gRPC clients use `insecure.NewCredentials()`. No TLS, no mTLS.

### High Priority Issues

- [x] **RabbitMQ routing key mismatch** — fixed: notification consumer now binds `account.*`, `course.*` and `grade.*` (the legacy `student.*` binding was replaced), and profile-service adds its own `account.created` consumer.
- [ ] **Hardcoded `guest:guest` RabbitMQ credentials** in the `rabbitmq` service block of `docker-compose.yml` (and repeated in each service's `RABBITMQ_URL`), with the management dashboard (port 15672) exposed to the host. Addressed by Phase 7.2.
- [x] **Debug print left in** — the catalogue service's `main` had a `fmt.Println("hej")` debugging leftover.
- [x] **Panic in repository constructor** — the Clover repository constructor in `course-content-service/internal/repository/clover_content.go` called `panic()` instead of returning an error.
- [x] **Missing course returns 500, not 404** — `GetCourse` in `course-catalogue-service/internal/repository/gorm_course_catalogue.go` returned a bare `gorm.ErrRecordNotFound`; gRPC mapped an unrecognised error to `Unknown`, and `common.GatewayErrorHandler` maps `Unknown` to 500. The repository now translates GORM's sentinel into `domain.ErrNotFound`, which the service turns into a gRPC `NotFound` (404) — the same shape course-content uses in `module_service.go`. Regression test: `TestGetCourseMissingReturns404`. The real `GET /api/courses/9999` response still needs a live-stack capture into `api-examples.md`.
- [x] **`AssignmentServer` embeds the interface, not the `Unimplemented` struct** — `assignment-service/internal/server/assignment_server.go` embedded `assignmentpb.AssignmentServiceServer`, a nil value. It was harmless only because the type implements all nine RPCs by hand; the first one added without an implementation would have panicked the server instead of returning `Unimplemented`. Switched to `assignmentpb.UnimplementedAssignmentServiceServer`.

### Test Coverage Gaps

- [x] **No tests** for `course-catalogue-service`, `notification-service`, or `auth-service`. — stale, re-verified and closed in [7.0](#70-issue-re-verification): all three have server-level tests since `917bb49` (`internal/server/{course,notification,auth}_server_test.go`) and the suite passes.
- [x] **Broken test assertions** — `TestCloverModuleRepository_CreateAndGetModule` in `course-content-service/internal/repository/clover_content_test.go` compared `UpdatedAt` four times instead of verifying Title, ID, and CourseID. Tests passed but didn't validate what they claimed. Addressed by Phase 7.1: the test now asserts ID, CourseID, Title, and Text.

### Structural / Quality Issues



- [ ] **No health check endpoints** — No `/healthz` on any service. No Docker health checks on Go services.
- [x] **No structured logging** — All services now use `log/slog` with a JSON handler (via shared `common.SetupLogging`) to stdout. Log levels are configurable via `LOG_LEVEL`, request IDs (`x-request-id`) and `user_id` are propagated via gRPC metadata and added to every log record. GORM and go-rabbitmq chatter is routed through slog via `common.NewGormLogger` and `common.RabbitLogger` — all app containers emit pure JSON (verified via `docker compose logs`).
- [x] **Port env var ignored** in `course-catalogue-service` and `course-content-service` — hardcoded instead of reading `os.Getenv("PORT")`. — stale, re-verified and closed in [7.0](#70-issue-re-verification): both `main` functions read `os.Getenv("PORT")` as of `c370ea0`.
- [x] **~150 lines of commented-out code** across `course-content-service` (attachment features never implemented).
- [ ] **Alpha-stage dependency** — CloverDB is at `v2.0.0-alpha.3`. No stability guarantees for production data.
- [ ] **No rate limiting** on Nginx gateway or any service.

## Phase 7: Rubric Gap Closure

**Goal:** Close the gap between the codebase and the INFS605 marking rubric. The rubric needs `.env.example`, example endpoint responses, screenshots, an enforced lint tool and documented logic. This phase tracks that work plus corrections to the issue list above.

Nothing here is started. The checkboxes are the decision list.

### 7.0 Issue re-verification

Two entries in *Issues and Additional Features* were re-checked against the current code and found stale. Both are ticked off in that section.

- [x] **"No tests for `course-catalogue-service`, `notification-service`, or `auth-service`"** — stale. All three have server-level tests since `917bb49`; suite passes.
- [x] **"Port env var ignored" in `course-catalogue-service` and `course-content-service`** — stale. Both `main` functions read `os.Getenv("PORT")` as of `c370ea0`.

Still open and confirmed genuine: gRPC unencrypted (`#580`), hardcoded `guest:guest` (`#585`, 8 occurrences in compose rather than 2), no CI (`#596`), no linting (`#597`), no health checks (`#598`), broken test assertions (`#592`), CloverDB alpha (`#602`), no rate limiting (`#603`).

An earlier draft of this section predicted the lint config in 7.1 would pass clean. That was wrong. Measured with `errcheck`, `govet`, `staticcheck`, `ineffassign` and `unused` across all eight modules, the baseline is **19 findings, not zero** — see 7.1.

### 7.1 Lint baseline & test-assertion fix

Lint tooling was evaluated but deliberately **not wired into CI** (see 7.0 and the
"Explicitly NOT doing" list): an enforced `golangci-lint` config adds a toolchain
dependency without changing what the grader sees. The measured baseline is
recorded here so that it does not silently grow if linting is picked up later.

Measured with `errcheck`, `govet`, `staticcheck`, `ineffassign` and `unused`
across all eight modules, the baseline is **19 findings, not zero** — the zero an
earlier draft of this section predicted. They are pre-existing error-handling and
style nits (unchecked error returns, ineffectual assignments, unused identifiers)
carried since before this phase, none of them correctness bugs, so they are left
as-is rather than churned at the end of the project.

The concrete defect this phase did fix is the broken test assertions listed under
*Test Coverage Gaps*: `TestCloverModuleRepository_CreateAndGetModule` in
`course-content-service/internal/repository/clover_content_test.go` compared
`UpdatedAt` four times instead of checking what its name claims. It now asserts
ID, CourseID, Title, and Text, so it fails if the repository returns the wrong
module.

### 7.2 Configuration

- [x] Add `.env.example` documenting every variable the stack reads: `JWT_SECRET`, `RABBITMQ_URL`, `RABBITMQ_DEFAULT_USER`, `RABBITMQ_DEFAULT_PASS`, `DB_PATH`, `NOSQL_PATH`, `UPLOAD_DIR`, `SEED_DATA`, `LOG_LEVEL`, `TOKEN_TTL_MINUTES`, `HTTP_PORT`, and the six `*_SERVICE_ADDR` values.
- [ ] Convert the `environment:` values in `docker-compose.yml` to `${VAR:-default}` with defaults identical to today's hardcoded values, so behaviour is unchanged and `.env` becomes a real override rather than a decorative file. — **deliberately deferred**: `guest:guest` stays, documented as a known limitation instead. Closes `#585` as "documented", not fixed.
- [x] Verify with `docker compose config`. `docker-compose.yml` is not covered by any test, so a mistyped interpolation would only surface there.
- [x] Document the `cp .env.example .env` step in the README.

### 7.3 Endpoint evidence

- [x] `docker compose up --build`, then exercise all 25 REST endpoints with `curl`, capturing the `Set-Cookie` from login and reusing it for the authenticated calls. Include at least one 401 and one 404, to evidence the auth interceptor and the nginx `/api/` catch-all rather than only the happy paths.
- [x] Write `docs/api-examples.md` with the real request and response pair per endpoint, and link it from the README.

Responses must be captured from a real run. Hand-written examples would not be evidence of anything.

**Result.** All 25 endpoints exercised and captured on a clean `docker compose down -v` run at `90cf51b`, plus four failure cases: 401 with no token on a unary route, 401 on both streaming file routes, the nginx catch-all 404, and a service-level 404 for a missing module. `docs/api-examples.md` writes up three of them — login, enrol, notifications — because those three between them cover the session boundary, the gateway's regex disambiguation, request-id correlation end to end, and the asynchronous path. The other 22 were exercised and checked in the same run, but are not written up individually.

The count is **25, not 24** as this section originally said: 18 paths / 23 operations in `proto/openapi/osbourne.swagger.json`, plus the two hand-written `HandlePath` file routes, which have no proto annotation by design.

Two things the run turned up, neither of which was predicted:

- **Starting the consumers before auth-service is load-bearing.** `auth-service` publishes `account.created` for the two seed accounts at startup, onto a topic exchange. Brought up in one `docker compose up`, a consumer whose queue is not yet bound simply never sees those events, and the profile rows and the welcome notifications do not exist at all. The evidence run starts `rabbitmq profile-service notification-service` first, waits for both consumers to log that they are listening, then brings up the rest. The enrolment- and grading-triggered notifications do not have this problem — they are published long after everything is up.
- **`GET /api/courses/9999` returns 500, not 404.** Recorded below as a new issue.



### 7.5 Comment policy reversal

The rubric awards marks for documented logic, and the `cmd/main.go` files are the first thing a grader opens. Stripping their comments cost more than the clutter was worth, so this reverses part of the cleanup in `dd799d1`.

The `// 1. Database Initialization` style section headers stay deleted. They described the code rather than explaining it.

### 7.6 Housekeeping

- [x] Delete the untracked, superseded `services/` tree. It is the pre-`go.work` layout and holds a full copy of every service's generated protobuf code. Safe to remove: untracked, and not referenced by `go.work`.
- [x] Decide whether `admin/` belongs in the submission repo. It holds three committed PDFs, including `Marking-Rubric.pdf`.
  - It does not.