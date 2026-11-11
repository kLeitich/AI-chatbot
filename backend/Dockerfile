# Stage 1: Build binary
FROM golang:1.22-alpine3.19 AS builder

# Install build dependencies for CGO (mattn/go-sqlite3 requires gcc and musl-dev)
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Download dependencies first for layer caching (no go mod tidy at build time)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o server .

# Stage 2: Runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Create non-root user and group, and create /data directory with correct ownership
RUN addgroup -S appgroup && adduser -S appuser -G appgroup && \
    mkdir -p /data /app && \
    chown -R appuser:appgroup /data /app

# Copy binary from builder
COPY --from=builder --chown=appuser:appgroup /app/server /app/server

# Default environment configuration
ENV PORT=8080 \
    SQLITE_PATH=/data/appointments.db \
    APP_ENV=production

# Switch to non-root user
USER appuser

# Persistence volume for SQLite database
VOLUME ["/data"]

EXPOSE 8080

CMD ["./server"]
