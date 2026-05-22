# Build stage
FROM golang:1.25.10-alpine AS builder

WORKDIR /build

# CHAT-s6qf3: optional Moses in-cluster package cache (Nexus). Empty = upstream default.
ARG GOPROXY=""
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org,direct}

COPY go.mod go.sum* ./
RUN go mod download

# Source. OpenAPI spec is embedded via api/api.go (//go:embed openapi.json).
COPY . .

# Verify rather than tidy — keeps builds reproducible.
RUN go mod verify
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# Runtime stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

COPY --from=builder /build/server .

EXPOSE 8080

RUN adduser -D -u 1000 appuser
USER appuser

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./server"]
