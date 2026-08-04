FROM golang:1.26.4-alpine AS builder
WORKDIR /build
# CHAT-s6qf3: optional Moses in-cluster package cache (Nexus). Empty = upstream default.
ARG GOPROXY=""
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org,direct}
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
# Verify rather than tidy — keeps builds reproducible. Static frontend
# assets and the OpenAPI spec are embedded via //go:embed in main.go.
RUN go mod verify
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /build/server .

# Run as non-root for parity with the other backend templates.
RUN adduser -D -u 1000 appuser
USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["./server"]
