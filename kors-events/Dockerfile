# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY . .

RUN cd kors-events && go mod download

RUN cd kors-events && CGO_ENABLED=0 GOOS=linux go build -o /kors-events ./cmd/consumer/main.go

# Stage 2: Final image
FROM alpine:3.18

WORKDIR /

COPY --from=builder /kors-events /kors-events
COPY .env.example .env

CMD ["/kors-events"]
