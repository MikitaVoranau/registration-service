# Этап сборки
FROM golang:1.24 as builder

WORKDIR /app

# Копируем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники
COPY . .

# Явно указываем путь для бинарника
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/auth-service ./cmd/auth/main.go

# Финальный образ
FROM alpine:3.18

WORKDIR /app

# Копируем бинарник из абсолютного пути
COPY --from=builder /bin/auth-service /app/

COPY .env /app/.env

# Даем права на выполнение
RUN chmod +x /app/auth-service

EXPOSE 50051
CMD ["/app/auth-service"]