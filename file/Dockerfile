# Этап сборки
FROM golang:1.24 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Собираем file-service
RUN CGO_ENABLED=0 GOOS=linux go build -o file-service ./cmd/file/main.go

# Финальный образ
FROM alpine:3.18

WORKDIR /app

# Добавляем CA сертификаты и timezone
RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/file-service .
COPY --from=builder /app/.env .

EXPOSE 50052
CMD ["./file-service"]