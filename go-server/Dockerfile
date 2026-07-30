FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o iskey-server cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/
COPY --from=builder /app/iskey-server .
COPY --from=builder /app/.env .

EXPOSE 8080
ENTRYPOINT ["./iskey-server"]
