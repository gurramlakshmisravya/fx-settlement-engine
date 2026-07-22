# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build application binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/settlement-engine ./cmd/server

# Final lightweight stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates and curl for healthchecks
RUN apk --no-cache add ca-certificates curl

COPY --from=builder /app/settlement-engine /app/settlement-engine
COPY --from=builder /app/migrations /app/migrations

EXPOSE 50051

CMD ["/app/settlement-engine"]
