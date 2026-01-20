# Build stage
FROM golang:1.25-alpine AS builder

# Install git and ca-certificates (needed for fetching dependencies)
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user for build
RUN adduser -D -g '' appuser

WORKDIR /build

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with security flags
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
        -X 'github.com/mcpregistry/server/internal/api.Version=${VERSION}' \
        -X 'github.com/mcpregistry/server/internal/api.GitCommit=${GIT_COMMIT}' \
        -X 'github.com/mcpregistry/server/internal/api.BuildTime=${BUILD_TIME}'" \
    -trimpath \
    -o /build/registry \
    ./cmd/registry

# Runtime stage - using distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Labels for container metadata
LABEL org.opencontainers.image.title="MCP Registry Server"
LABEL org.opencontainers.image.description="GitOps-based MCP server registry"
LABEL org.opencontainers.image.vendor="MCP Registry"
LABEL org.opencontainers.image.source="https://github.com/mcpregistry/server"

# Copy timezone data and CA certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary from builder
COPY --from=builder /build/registry /registry

# Use non-root user (65532 is the nonroot user in distroless)
USER 65532:65532

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/registry", "-health-check"]

# Volume for git clone data
VOLUME ["/data"]

# Set environment defaults
ENV PORT=8080 \
    DATA_PATH=/data/registry \
    CLONE_TIMEOUT=2m \
    POLL_INTERVAL=5m \
    CACHE_SIZE=1000

# Run the binary
ENTRYPOINT ["/registry"]
