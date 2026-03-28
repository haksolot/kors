# =============================================================================
# KORS BFF — Multi-stage build
# Build context: repo root (go.work required for local module resolution)
# Usage: docker build -f services/bff/Dockerfile -t kors-bff .
# =============================================================================

FROM golang:latest AS builder
WORKDIR /workspace
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/bff ./services/bff/cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=builder /bin/bff .
EXPOSE 8080 9092
ENTRYPOINT ["/app/bff"]
