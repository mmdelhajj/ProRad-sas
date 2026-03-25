# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Build arguments - set BUILD_MODE=dev for development builds
ARG BUILD_MODE=production

# Install dependencies and garble for obfuscation
RUN apk add --no-cache git
RUN go install mvdan.cc/garble@v0.12.1

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the API server with OBFUSCATION
# For dev: --build-arg BUILD_MODE=dev (no obfuscation, for debugging)
# For production: default - OBFUSCATED binary (very hard to reverse-engineer)
RUN if [ "$BUILD_MODE" = "dev" ]; then \
      echo "Building DEV version (no obfuscation)..." && \
      CGO_ENABLED=0 GOOS=linux go build -ldflags "-X github.com/proisp/backend/internal/license.buildMode=dev -X github.com/proisp/backend/internal/security.buildMode=dev" -o /api ./cmd/api; \
    else \
      echo "Building PRODUCTION version (obfuscated)..." && \
      CGO_ENABLED=0 GOOS=linux garble -tiny -seed=random build -ldflags "-s -w" -o /api ./cmd/api; \
    fi

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates, timezone, postgresql-client for backups, and freeradius-utils for radclient CoA
RUN apk --no-cache add ca-certificates tzdata postgresql-client freeradius-utils

# Copy binary from builder
COPY --from=builder /api .

# Expose port
EXPOSE 8080

# Run the server
CMD ["./api"]
