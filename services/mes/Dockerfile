# =============================================================================
# KORS MES — Multi-stage build
# Build context: repo root (go.work required for local module resolution)
# Usage: docker build -f services/mes/Dockerfile -t kors-mes .
# =============================================================================

FROM golang:1.24-alpine AS builder
WORKDIR /workspace
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/mes ./services/mes/cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /bin/mes .
# Migrations must be in WORKDIR/migrations so goose.Up(sqlDB, "migrations") resolves correctly
COPY services/mes/migrations/ migrations/
EXPOSE 9090
ENTRYPOINT ["/app/mes"]
