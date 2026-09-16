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
* **Implementation:** Route incoming HTTP REST calls (`/api/v1/students`) on to the internal gRPC profile-service.
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

## [ ] Phase 5: High Availability & Instance Scaling

**Goal:** Demonstrate horizontal scaling and load distribution across the services.

### [ ] Step 5.1: Multi-Instance Docker Compose Configuration

* **Action:** Remove specific port bindings on internal microservice containers in `docker-compose.yml`.
* **Execution:**
```bash
docker compose up -d --scale profile-service=3 --scale notification-service=2

```

* **Test:** Run `docker compose ps` and confirm that all instances run on the shared Docker network.

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
hey -n 200 -c 20 http://localhost/api/v1/courses
```
--- 

## Issues and Additional Features

### Critical Bugs

- [x] **Inverted ID generation in CreateModule** — `course-content-service/internal/service/module-service.go:25` generates a UUID only when `module.ID != ""`, which is the opposite of what you want. Should be `== ""`.
- [x] **DeleteModule does nothing** — `course-content-service/internal/service/module-service.go:67-77` validates the module exists but never calls `s.repo.DeleteModule()`. Deletions silently no-op.
- [x] **No authentication** — fixed: `frontend/internal/handler/handler.go` now runs a `Authenticate` middleware that reads the JWT from the `osbourne_session` cookie (no more `?id=` impersonation); auth-service issues the token and a shared `common` gRPC interceptor enforces it on all five backend services.
- [ ] **All gRPC traffic is unencrypted** — All 5 frontend gRPC clients use `insecure.NewCredentials()`. No TLS, no mTLS.

### High Priority Issues

- [x] **RabbitMQ routing key mismatch** — fixed: notification consumer now binds `account.*`, `course.*` and `grade.*` (the legacy `student.*` binding was replaced), and profile-service adds its own `account.created` consumer.
- [ ] **Hardcoded `guest:guest` RabbitMQ credentials** in `docker-compose.yml:40-41` with the management dashboard (port 15672) exposed to the host.
- [x] **Debug print left in** — `course-catalogue-service/cmd/main.go:21` has `fmt.Println("hej")`.
- [x] **Panic in repository constructor** — `course-content-service/internal/repository/clover-content.go:23` calls `panic()` instead of returning an error.

### Test Coverage Gaps

- [ ] **No tests** for `course-catalogue-service`, `notification-service`, or `auth-service`.
- [ ] **Broken test assertions** — `course-content-service/internal/repository/clover_content_test.go:69-83` compares `UpdatedAt` four times instead of verifying Title, ID, and CourseID. Tests pass but don't actually validate what they claim.

### Structural / Quality Issues

- [ ] **No CI/CD pipeline** — No GitHub Actions, GitLab CI, or any automation.
- [ ] **No linting/formatting** — No `.golangci.yml` or equivalent.
- [ ] **No health check endpoints** — No `/healthz` on any service. No Docker health checks on Go services.
- [x] **No structured logging** — All services now use `log/slog` with a JSON handler (via shared `common.SetupLogging`) to stdout. Log levels are configurable via `LOG_LEVEL`, request IDs (`x-request-id`) and `user_id` are propagated via gRPC metadata and added to every log record. GORM and go-rabbitmq chatter is routed through slog via `common.NewGormLogger` and `common.RabbitLogger` — all app containers emit pure JSON (verified via `docker compose logs`).
- [ ] **Port env var ignored** in `course-catalogue-service` and `course-content-service` — hardcoded instead of reading `os.Getenv("PORT")`.
- [x] **~150 lines of commented-out code** across `course-content-service` (attachment features never implemented).
- [ ] **Alpha-stage dependency** — CloverDB is at `v2.0.0-alpha.3`. No stability guarantees for production data.
- [ ] **No rate limiting** on Nginx gateway or any service.