# ============================================
# College Schedule Bot — Docker (оптимизирован)
# ============================================

# ---- builder ----
FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w -extldflags=-static" -o /build/bot ./cmd/bot/

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl python3 && \
    curl -fsSL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 \
    -o /usr/local/bin/cloudflared && chmod +x /usr/local/bin/cloudflared && \
    apk del curl && rm -rf /var/cache/apk/*

WORKDIR /app

COPY --from=builder /build/bot /app/bot
COPY schedule.json changes.json /app/
COPY tools/ /app/tools/
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
