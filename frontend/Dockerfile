FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates git 

WORKDIR /src

# The frontend depends on the sibling common module, so the build context
# must be the repository root (set in docker-compose.yml).
COPY common/ common/
COPY frontend/ frontend/

WORKDIR /src/frontend

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /frontend ./cmd/main.go

FROM alpine:3.20

RUN apk --no-cache add ca-certificates && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /frontend .
    
RUN chown appuser:appgroup /app/frontend

USER appuser

EXPOSE 8080

ENTRYPOINT ["./frontend"]