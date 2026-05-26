# ============================================
# College Schedule Bot — Docker (standalone)
# ============================================

FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /build/bot ./cmd/bot/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates python3 py3-pip curl bash jq && \
    curl -fsSL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 \
    -o /usr/local/bin/cloudflared && chmod +x /usr/local/bin/cloudflared
WORKDIR /app
COPY --from=builder /build/bot /app/bot
COPY *.json /app/
COPY tools/ /app/tools/
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
