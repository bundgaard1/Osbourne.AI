FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src

# The auth-service depends on the sibling auth-common module, so the build
# context must be the repository root (set in docker-compose.yml).
COPY auth-common/ auth-common/
COPY auth-service/ auth-service/

WORKDIR /src/auth-service

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /auth-service ./cmd/main.go

FROM alpine:3.20

RUN apk --no-cache add ca-certificates && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /auth-service .

RUN mkdir -p /app/data && \
    chown -R appuser:appgroup /app

USER appuser

EXPOSE 50056

ENTRYPOINT ["./auth-service"]