# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY . .

RUN cd kors-worker && go mod download

RUN cd kors-worker && CGO_ENABLED=0 GOOS=linux go build -o /kors-worker ./cmd/worker/main.go

# Stage 2: Final image
FROM alpine:3.18

WORKDIR /

COPY --from=builder /kors-worker /kors-worker
COPY .env.example .env

CMD ["/kors-worker"]
