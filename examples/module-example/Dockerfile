# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY . .

RUN cd examples/module-example && go mod download

RUN cd examples/module-example && CGO_ENABLED=0 GOOS=linux go build -o /module-example ./cmd/server/main.go

# Stage 2: Final image
FROM alpine:3.18

WORKDIR /

COPY --from=builder /module-example /module-example
# Copy migrations folder for auto-migration
COPY --from=builder /app/examples/module-example/migrations /migrations
COPY examples/module-example/.env .env

EXPOSE 8081

CMD ["/module-example"]
