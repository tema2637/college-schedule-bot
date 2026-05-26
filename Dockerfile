# ============================================
# College Schedule Bot — Docker
# ============================================

# ---- Build stage ----
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /build/bot ./cmd/bot/

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates python3 py3-pip curl bash jq

# cloudflared
RUN curl -fsSL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 \
    -o /usr/local/bin/cloudflared && chmod +x /usr/local/bin/cloudflared

WORKDIR /app

COPY --from=builder /build/bot /app/bot
COPY schedule.json /app/schedule.json
COPY messages.json /app/messages.json
COPY users.json /app/users.json
COPY tools/ /app/tools/
COPY config.json /app/config.json

# Entrypoint — автотуннель + запуск бота
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
