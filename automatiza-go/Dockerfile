# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

ARG VERSION=dev

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.Version=${VERSION}" \
    -o /server ./cmd/server

# Runtime stage
FROM alpine:3.20

WORKDIR /app

# Install ca-certificates, timezone data, psql client (migrations), and curl (healthcheck)
RUN apk --no-cache add ca-certificates tzdata postgresql16-client curl bash

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Copy binary
COPY --from=builder /server .

# Copy migration scripts
COPY scripts/db/run-migrations.sh /app/db/run-migrations.sh
COPY scripts/db/init.sql /app/db/init.sql
COPY scripts/db/migrations/ /app/db/migrations/

# Make migration script executable
RUN chmod +x /app/db/run-migrations.sh

# Set ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Set timezone
ENV TZ=America/Sao_Paulo

# Expose port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -sf http://localhost:3000/api/v1/health || exit 1

# Run
CMD ["./server"]
