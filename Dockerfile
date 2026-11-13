# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /src
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/popcornhub ./cmd/server

# Final runtime stage
FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /bin/popcornhub /app/popcornhub
COPY web/static /app/web/static

USER appuser

EXPOSE 8085

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8085/healthz || exit 1

ENTRYPOINT ["/app/popcornhub"]
