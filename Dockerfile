# === Этап 1: Сборка ===
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

# 🔑 Копируем go.mod/go.sum ПЕРВЫМИ (кэшируем слой)
COPY go.mod go.sum ./
RUN go mod download

# 🔑 Потом копируем код
COPY . .

# 🔑 Собираем статический бинарник для Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o main ./cmd/server/main.go

# === Этап 2: Рантайм ===
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# 🔑 Копируем бинарник
COPY --from=builder /app/main .

# 🔑 ⭐ КОПИРУЕМ ШАБЛОНЫ И СТАТИКУ
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

RUN mkdir -p /root/uploads

EXPOSE 8000

CMD ["./main"]
