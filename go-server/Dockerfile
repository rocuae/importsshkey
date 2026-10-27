FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o iskey-server .

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -S -g 10001 iskey \
    && adduser -S -D -H -u 10001 -G iskey iskey \
    && mkdir -p /data \
    && chown iskey:iskey /data

WORKDIR /app
COPY --from=builder /app/iskey-server .
COPY --from=builder /app/config.yaml .
RUN chown -R iskey:iskey /app

USER iskey
ENV DB_PATH=/data/iskey.db
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/health >/dev/null 2>&1 || exit 1
ENTRYPOINT ["./iskey-server", "server", "-f"]
