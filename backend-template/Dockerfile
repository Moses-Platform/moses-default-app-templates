# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Copy dependency manifest and download dependencies
# go.sum* glob ensures this works whether or not go.sum exists yet
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code and ensure module checksums are valid
COPY . .
RUN go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server/

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /server /server

# Copy OpenAPI spec
COPY api/openapi.json /app/api/openapi.json

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the server
CMD ["/server"]
