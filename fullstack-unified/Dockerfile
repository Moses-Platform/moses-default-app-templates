FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
# Verify rather than tidy — keeps builds reproducible. Static frontend
# assets and the OpenAPI spec are embedded via //go:embed in main.go.
RUN go mod verify
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

FROM alpine:3.19
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
