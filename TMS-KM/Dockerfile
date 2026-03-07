# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY . .

RUN cd TMS-KM && go mod download

RUN cd TMS-KM && CGO_ENABLED=0 GOOS=linux go build -o /tms-km ./cmd/server/main.go

# Stage 2: Final image
FROM alpine:3.18

WORKDIR /

COPY --from=builder /tms-km /tms-km
# Note: TMS-KM will use the same database connection as provisioned
COPY TMS-KM/.env .env

EXPOSE 8081

CMD ["/tms-km"]
