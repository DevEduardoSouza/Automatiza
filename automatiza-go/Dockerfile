# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Declared right before its only use: an in-scope ARG busts the cache of every
# following RUN, and VERSION changes per deploy — declaring it at the top
# forced apk add + go mod download to re-run on every single build.
ARG VERSION=dev

# Build
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.Version=${VERSION}" \
    -o /server ./cmd/server

# Runtime stage
FROM alpine:3.20

WORKDIR /app

# Install ca-certificates, timezone data, psql client (migrations), curl
# (healthcheck) and ffmpeg (relay de vídeo: normaliza para faststart 1x antes
# do fan-out para não transcodificar o mesmo vídeo por grupo no WAHA).
RUN apk --no-cache add ca-certificates tzdata postgresql16-client curl bash ffmpeg

# Create non-root user (and own /app while it is still empty — file ownership
# is handled per-COPY below, avoiding a `chown -R` layer that duplicated the
# whole binary into an extra ~40MB layer shipped on every deploy)
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup && \
    chown appuser:appgroup /app

# Copy migration scripts (change rarely — kept BEFORE the binary so a code-only
# deploy invalidates nothing but the final binary layer)
COPY --chown=appuser:appgroup --chmod=755 scripts/db/run-migrations.sh /app/db/run-migrations.sh
COPY --chown=appuser:appgroup scripts/db/init.sql /app/db/init.sql
COPY --chown=appuser:appgroup scripts/db/migrations/ /app/db/migrations/

# Copy binary (changes every deploy — last layer, the only one pulled per deploy)
COPY --from=builder --chown=appuser:appgroup /server .

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
