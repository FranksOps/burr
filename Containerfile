# Stage 1: Build
FROM registry.access.redhat.com/ubi9/go-toolset:1.22 AS builder

WORKDIR /opt/app-root/src

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build a statically linked binary without CGO (since modernc.org/sqlite is pure Go)
# We inject version info using ldflags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s \
    -X main.version=$(git describe --tags --always 2>/dev/null || echo dev) \
    -X main.commit=$(git rev-parse HEAD 2>/dev/null || echo none) \
    -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o bin/burr ./cmd/burr

# Stage 2: Runtime
# Using scratch for the smallest possible surface area and maximum security
FROM scratch

# Copy CA certificates from the builder stage so we can make HTTPS requests
COPY --from=builder /etc/pki/tls/certs/ca-bundle.crt /etc/ssl/certs/ca-certificates.crt

# Copy the static binary
COPY --from=builder /opt/app-root/src/bin/burr /burr

# We create a directory for persistent storage (e.g. SQLite DBs, JSON output)
# Note: since scratch has no tools, users must mount a volume to /data
# If running rootless Podman, use --userns=keep-id when mounting volumes
VOLUME ["/data"]

WORKDIR /data

# Default entrypoint points to the binary
ENTRYPOINT ["/burr"]

# Default command shows help
CMD ["--help"]
