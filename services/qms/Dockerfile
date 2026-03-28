# =============================================================================
# KORS QMS — Multi-stage build
# Build context: repo root (go.work required for local module resolution)
# Usage: docker build -f services/qms/Dockerfile -t kors-qms .
# =============================================================================

FROM golang:latest AS builder
WORKDIR /workspace
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/qms ./services/qms/cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=builder /bin/qms .
# Migrations must be in WORKDIR/migrations so goose.Up(sqlDB, "migrations") resolves correctly
COPY services/qms/migrations/ migrations/
EXPOSE 9091
ENTRYPOINT ["/app/qms"]
