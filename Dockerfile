# Build stage
FROM golang:1.24-alpine AS builder

# Install git for go mod download
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'docker')" \
    -o /covenant \
    ./cmd/contract

# Final stage
FROM scratch

# Copy timezone data and CA certificates
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /covenant /covenant

# Create data directory for filesystem storage
VOLUME ["/data"]

# Expose broker port
EXPOSE 8080

# Run the broker
ENTRYPOINT ["/covenant"]
CMD ["broker", "--storage", "filesystem", "--storage-path", "/data", "--port", "8080"]
