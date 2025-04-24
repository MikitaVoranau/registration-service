# ---------- Build Stage ----------
FROM golang:1.22-alpine AS builder

# Устанавливаем рабочую директорию внутри контейнера
WORKDIR /app

# Устанавливаем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники
COPY pkg pkg
COPY cmd cmd
COPY internal internal
COPY config config

# Кэш для ускорения повторных сборок
ENV GOCACHE=/gocache
RUN --mount=type=cache,target=/gocache \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o main ./cmd/main

# ---------- Run Stage ----------
FROM alpine:3.21

# Копируем бинарник и файл окружения
COPY --from=builder /app/main /main
COPY --from=builder /app/config/local.env /config/local.env

# Указываем рабочую директорию и команду запуска
WORKDIR /
CMD ["./main"]
