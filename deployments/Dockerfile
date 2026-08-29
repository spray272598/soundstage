# =============================================================================
# Soundstage Multi-stage Dockerfile
# =============================================================================
# Profiles:
#   target=app      : Production (distroless, minimal attack surface)
#   target=dev      : Development (debian-slim with shell for debugging)
# =============================================================================

# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
ARG VERSION=dev
ARG GIT_COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X github.com/spray272598/soundstage/internal/pkg/config.Version=${VERSION} -X github.com/spray272598/soundstage/internal/pkg/config.GitCommit=${GIT_COMMIT}" \
    -o /soundstage ./cmd/server

# Production stage (distroless)
FROM gcr.io/distroless/static-debian12 AS app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /soundstage /app/soundstage

# Copy config
COPY deployments/config.docker.yaml /app/configs/config.yaml

# Use non-root user
USER nonroot:nonroot

EXPOSE 8080 8081 9091

ENTRYPOINT ["/app/soundstage", "-config", "/app/configs/config.yaml"]

# Development stage (with shell for debugging)
FROM golang:1.22-alpine AS dev

WORKDIR /app

# Install debugging tools
RUN apk add --no-cache git curl jq vim

# Copy binary from builder
COPY --from=builder /soundstage /app/soundstage

# Copy config
COPY deployments/config.docker.yaml /app/configs/config.yaml

EXPOSE 8080 8081 9091

ENTRYPOINT ["/app/soundstage", "-config", "/app/configs/config.yaml"]