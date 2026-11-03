FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*

COPY server /app/popcornhub
COPY web/static /app/web/static

EXPOSE 8085

ENTRYPOINT ["/app/popcornhub"]
