# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy the entire workspace to handle local module dependencies
COPY . .

# Download dependencies
RUN cd kors-api && go mod download

# Build the binary
RUN cd kors-api && CGO_ENABLED=0 GOOS=linux go build -o /kors-api ./cmd/server

# Stage 2: Final image
FROM alpine:3.18

WORKDIR /

COPY --from=builder /kors-api /kors-api
COPY kors-api/migrations /migrations
COPY .env.example .env

EXPOSE 8080

CMD ["/kors-api"]
