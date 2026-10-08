# The actual todo list

Final stretch to get this project submitting ready.
Cross-checked against `tips-from-bingji.md` and the open items in `docs/plan.md`.

## A. Code fixes

- [x] **`.env.example`** (plan 7.2, scoped) — created `.env.example` listing every var the
  stack reads (`JWT_SECRET`, `RABBITMQ_URL`, `RABBITMQ_DEFAULT_USER`, `RABBITMQ_DEFAULT_PASS`,
  `DB_PATH`, `NOSQL_PATH`, `UPLOAD_DIR`, `SEED_DATA`, `LOG_LEVEL`, `TOKEN_TTL_MINUTES`,
  `HTTP_PORT`, and the six `*_SERVICE_ADDR`), defaults identical to today's hardcoded values.
  **No compose interpolation** — `guest:guest` stays and becomes a documented known limitation.
  Added `cp .env.example .env` to README setup. Verified with `docker compose config`.
- [x] **500 → 404 bug** (plan #588) — `GetCourse` in
  `course-catalogue-service/internal/repository/gorm_course_catalogue.go` returned a bare
  `gorm.ErrRecordNotFound` → `Unknown` → HTTP 500. Repository now maps it to `domain.ErrNotFound`,
  service translates to gRPC `NotFound` (same shape as course-content's sentinel in
  `module_service.go`); regression test `TestGetCourseMissingReturns404` added. Remaining: capture
  a real `GET /api/courses/9999` → 404 into `docs/api-examples.md` failure cases (needs a live stack).
- [x] **Broken test assertions** (plan #592) — `TestCloverModuleRepository_CreateAndGetModule`
  in `course-content-service/internal/repository/clover_content_test.go` compares `UpdatedAt`
  four times; assert Title, ID, CourseID instead (lines ~73–86).
- [x] **`AssignmentServer` embed** (plan #589) —
  `assignment-service/internal/server/assignment_server.go:22` embeds the *interface*
  (nil value); switch to `UnimplementedAssignmentServiceServer`. Build + test.
- [x] **gRPC deadlines in frontend** — no call ever set a deadline, so a hung service hung
  the request (Bingji: "does it wait indefinitely?"). Added a 10 s `context.WithTimeout` in
  `Handler.authCtx` (`frontend/internal/handler/handler.go`, covers all 10 page-handler call
  sites in `pages.go`) plus the pre-auth profile fetch in `Authenticate`.
  `DeadlineExceeded→503` mapping already exists and is tested (`enroll_test.go:183`).
  Added `TestAuthCtxSetsADeadline` asserting `authCtx` yields a deadline.
- [x] **Phase 7.5 comment restoration** — restored the 6 condensed comment blocks in
  `auth-service`, `assignment-service`, `profile-service` `cmd/main.go`
  (loopback creds safe / no interceptor on login / token stripped from body /
  stream interceptor / loopback REST self-dial / shutdown order).

## B. Documentation

- [x] **README**
  - [x] API endpoint table (source: plan §6.0, 25 endpoints grouped by service).
  - [x] "Testing process" section: `go test ./...` (8 modules), `nginx/routing-test.sh`,
        links to `docs/api-examples.md` and `docs/screenshots/`.
  - [x] "Known limitations" section: gRPC unencrypted (#580), `guest:guest` (#585),
        no rate limiting (#603), no health checks (#598),
        notification consumer not idempotent (at-least-once → duplicates possible).
  - [x] Embed architecture diagram `docs/arch-diagram.png`.
  - [x] Replace the typo'd **"Demo Video" placeholder** with an "Evidence" section pointing
        at `docs/screenshots/` + `docs/api-examples.md`.
  - [x] Setup: add the `cp .env.example .env` step (already present).
  
- [x] **`docs/design.md`** — added two short sections:
  - [x] **Failure & resilience**: durable exchange/queues drain on consumer restart;
        publisher failure logged, not rolled back (no outbox — trade-off);
        `account.created` needs consumers bound before auth-service starts (topic exchange
        drops unbound messages); frontend deadline → 503;
        notification consumer not idempotent → duplicate notifications on redelivery.
  - [x] **Design rationale**: why these 6 service boundaries, which service owns which data,
        where the dependencies sit (frontend → all sync; services coupled only via events),
        trade-offs taken, what you'd change at scale.
- [x] **`docs/plan.md`**
  - [x] Mark **Phase 5 out-of-scope** with a one-paragraph justification (not required by
        assessment tips; Docker embedded DNS + shared queue would largely work, traded off
        for submission readiness — also feeds the "trade-offs" section).
  - [x] Restore missing **§7.1** (referenced twice: line 594 "Addressed by Phase 7.1" and
        line 622 "see 7.1"): document the lint baseline (19 findings, errcheck/govet/
        staticcheck/ineffassign/unused across 8 modules) and the test-assertion fix.
        (No §7.4 reference exists — nothing to do there.)
  - [x] Tick 7.2 boxes for `.env.example`, README step and `docker compose config`;
        annotate the compose-interpolation box as **deliberately deferred** (`guest:guest`
        documented as known limitation instead). Closes #585 as "documented", not fixed.
  - [x] Tick issues #588, #589, #592 once the code fixes above land.

## C. Evidence (screenshots — user captures, then wire in)

Capture plan written: `docs/screenshots/README.md` (filenames, commands, expected
output, and the README gallery snippet).

- [ ] `docker compose ps` — full stack up.
- [ ] Login / dashboard (UI works end to end).
- [ ] Enrol → notification appears, **plus** RabbitMQ dashboard (`localhost:15672`) queue
      counters — proves service-to-service comms over the message queue.
- [ ] `docker compose logs` showing one shared `request_id` across nginx → frontend/service.
- [ ] Error set: 401 (no token), gateway 404 (unknown `/api/` path), service 404
      (`GET /api/courses/9999` after fix).
- [ ] Persistence: `docker compose restart`, data still there (named volumes).
- [ ] Wire captured images into `docs/screenshots/` + README Evidence section.

## D. Verification (run at the end)

- [ ] `go test ./...` + `go vet ./...` in all 8 modules (go.work).
- [ ] `docker compose config` passes.
- [ ] `nginx/routing-test.sh` passes.
- [ ] `docker compose up --build` + full manual pass: login, enrol, upload, grade,
      notification, fixed 404.

## Explicitly NOT doing (documented as limitations instead)

Phase 5 scaling · compose `${VAR}` interpolation · lint/CI setup · health checks ·
TLS · rate limiting.
