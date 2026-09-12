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
* [ ] As soon as the DB update succeeds, a `grade.published` event is published on RabbitMQ with `{student_id, course_id, grade}`.

#### **3. gRPC Server Setup**

* [x] **Proto Specification:** Define `grading.proto` with `SubmitAssignment` and `GradeSubmission` RPC calls.
* [x] **gRPC Handler:** Create the server implementation and hook it onto the gRPC port (e.g. `:50054`).

#### **4. Frontend / BFF Integration & Test**

* [ ] **UI Form:** Create a simple page/form in the frontend where an instructor can select a student and enter a grade.
* [ ] **Verification of Event Flow:**
1. Instructor clicks "Save Grade" $\rightarrow$ Frontend calls `/api/grades`.
2. Frontend calls `grading-service` via gRPC.
3. `grading-service` saves in its DB and sends a `grade.published` event on RabbitMQ.
4. `notification-service` catches the event and saves a notification ("You have received the grade A in CS101").
5. The student logs in and sees the notification on their dashboard.

---

### [ ] Step 3.4: Event Publishing from Services

#### **1. Database & Domain Event Setup**

* [ ] **Domain Events Definition:** Create a shared struct/proto for events (e.g. `StudentCreatedEvent`, `EnrollmentCreatedEvent`).
* [ ] **RabbitMQ Producer Wrapper:** Build a reusable `publisher.go` in your service, which handles the connection, channels and reconnection to RabbitMQ.
* [ ] **JSON/Protobuf Serialization:** Convert your domain event to JSON or Protobuf before it is published on the exchange.

#### **2. Service Layer Integration**

* [ ] **Publish on DB mutation:** Call `publisher.Publish("student.created", payload)` right after a successful SQL transaction (e.g. in `CreateProfile`).
* [ ] **Error Handling / Fallback:** Make sure to log a clear error if the DB change succeeded but the RabbitMQ call fails (or implement the Outbox pattern, if you want to be extra thorough).

#### **3. Server & Consumer Setup**

* [ ] **Notification Consumer Setup:** In `notification-service`, listen at runtime on the RabbitMQ queue `notification-queue` bound to the relevant routing keys (`*.created`, `*.published`).
* [ ] **Consumer Handler:** Create a new notification in the Notification DB when a message is received.

#### **4. Frontend Integration & Test**

* [ ] **UI Trigger:** Create a new student or enroll in a course via the Frontend.
* [ ] **RabbitMQ Dashboard Check:** Check http://localhost:15672 and verify that the message count increases under the `Publish` rate on the queue.
* [ ] **Notification Badge in Frontend:** Open the notifications page in the frontend and verify that the newly created notification is shown to the user.

---

### [ ] Step 3.5: Authentication Service (Service #6 - Security Boundary)

#### **1. Database Setup**

* [ ] **Auth Schema / Migrations:** Create the `user_accounts` table with `id`, `email`, `password_hash`, and `role` (`student`, `teacher`, `admin`).
* [ ] **Crypto Setup:** Implement `bcrypt` for secure hashing and comparison of passwords.

#### **2. Service & JWT Implementation**

* [ ] **JWT Generator:** Create a helper function to issue and sign JWT tokens (containing `user_id`, `email`, `role` and `exp`).
* [ ] **Auth Service Methods:** Implement `Register` and `Login` methods in the service layer.

#### **3. gRPC Server & Gateway Interceptors**

* [ ] **Proto Specification:** Define `auth.proto` with `Login` and `ValidateToken` RPCs.
* [ ] **gRPC Auth Interceptor:** Create a gRPC Interceptor across microservices that reads the JWT token from the gRPC Context Metadata (`authorization: bearer <token>`) and verifies the signature.

#### **4. Frontend / BFF Integration & Test**

* [ ] **Session / Cookie Handling:** When the user logs in on `/login` in the Go Frontend, `auth-service` is called. On success, the JWT token is stored in a secure, `HttpOnly` cookie in the browser.
* [ ] **BFF Middleware (`h.Authenticate`):** Update your `Authenticate` middleware in the Go Frontend to read the cookie and attach the JWT token as gRPC Metadata on *all* outgoing microservice calls.
* [ ] **Test Scenarios:**
* [ ] Test access to `/dashboard` without a login cookie $\rightarrow$ Redirected to `/login` (or returns HTTP `401`).
* [ ] Test access with a valid login $\rightarrow$ The gRPC calls receive `user_id` directly from gRPC metadata and return the correct user's data (`200 OK`).

---

## [ ] Phase 4: Observability, Documentation & Final Check

**Goal:** Fulfill all non-functional requirements in the course's assessment criteria.

### [ ] Step 4.1: Centralized Logging & Error Handling

* **Action:** Ensure that all 6 Go services use structured logging (`slog` or `zap`) to `stdout`/`stderr`.
* **Test:** Run `docker compose logs -f` and follow a request's path through the Gateway and services.

### [ ] Step 4.2: Documentation

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

## Extra add ons.

### Content Service (CloverDB) - Additional Features

- Add list of files to modules!

---

## Issues and Additional Features

### Critical Bugs

- [x] **Inverted ID generation in CreateModule** — `course-content-service/internal/service/module-service.go:25` generates a UUID only when `module.ID != ""`, which is the opposite of what you want. Should be `== ""`.
- [ ] **DeleteModule does nothing** — `course-content-service/internal/service/module-service.go:67-77` validates the module exists but never calls `s.repo.DeleteModule()`. Deletions silently no-op.
- [ ] **No authentication** — `frontend/internal/handler/handler.go:59-61` reads user identity from `?id=` query param with a hardcoded fallback `"12345"`. Anyone can impersonate any user. Auth-service is a skeleton with no implementation.
- [ ] **All gRPC traffic is unencrypted** — All 5 frontend gRPC clients use `insecure.NewCredentials()`. No TLS, no mTLS.

### High Priority Issues

- [ ] **RabbitMQ routing key mismatch** — Notification consumer subscribes to `"student.*"` but also handles `"course.enrolled"` events. With a topic exchange, `course.enrolled` messages will never reach this consumer. Dead code at the network level.
- [ ] **Hardcoded `guest:guest` RabbitMQ credentials** in `docker-compose.yml:40-41` with the management dashboard (port 15672) exposed to the host.
- [x] **Debug print left in** — `course-catalogue-service/cmd/main.go:21` has `fmt.Println("hej")`.
- [ ] **Panic in repository constructor** — `course-content-service/internal/repository/clover-content.go:23` calls `panic()` instead of returning an error.

### Test Coverage Gaps

- [ ] **No tests** for `course-catalogue-service`, `notification-service`, or `auth-service`.
- [ ] **Broken test assertions** — `course-content-service/internal/repository/clover_content_test.go:69-83` compares `UpdatedAt` four times instead of verifying Title, ID, and CourseID. Tests pass but don't actually validate what they claim.

### Structural / Quality Issues

- [ ] **No CI/CD pipeline** — No GitHub Actions, GitLab CI, or any automation.
- [ ] **No linting/formatting** — No `.golangci.yml` or equivalent.
- [ ] **No health check endpoints** — No `/healthz` on any service. No Docker health checks on Go services.
- [ ] **No structured logging** — All services use `log.Printf`. No log levels, no correlation IDs, no JSON output. GORM `logger.Info` is active in production.
- [ ] **Port env var ignored** in `course-catalogue-service` and `course-content-service` — hardcoded instead of reading `os.Getenv("PORT")`.
- [ ] **~150 lines of commented-out code** across `course-content-service` (attachment features never implemented).
- [ ] **Alpha-stage dependency** — CloverDB is at `v2.0.0-alpha.3`. No stability guarantees for production data.
- [ ] **No rate limiting** on Nginx gateway or any service.