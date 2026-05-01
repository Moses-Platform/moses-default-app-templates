# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy dependency manifest. go.sum* glob handles templates with no
# external deps (where go mod tidy produces no go.sum).
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code. The OpenAPI spec at api/openapi.json is embedded
# into the binary via api/api.go (//go:embed), so no runtime COPY is needed.
COPY . .

# Verify checksums against go.sum and build. We deliberately do NOT run
# `go mod tidy` here — that would mutate go.sum at build time and make
# builds non-reproducible; tidy is a developer-time operation.
RUN go mod verify
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server/

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS calls + wget for HEALTHCHECK
RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /server /server

# Run as non-root
RUN adduser -D -u 1000 appuser
USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["/server"]
