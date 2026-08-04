FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o iskey-server .

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/iskey-server .
COPY --from=builder /app/config.yaml .

EXPOSE 8080
ENTRYPOINT ["./iskey-server", "server", "-f"]
