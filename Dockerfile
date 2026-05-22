# Build stage
FROM golang:1.26 AS builder

WORKDIR /app

# Install dependencies (Debian uses apt)
RUN apt-get update && apt-get install -y git ca-certificates && rm -rf /var/lib/apt/lists/*

# Copy go mod files
COPY go.mod go.sum ./
COPY vendor ./vendor

# 🔧 Добавляем настройки прокси и таймаута
ENV GOPROXY=off
ENV GOSUMDB=off
ENV GOFLAGS=-mod=vendor

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server/main.go

# Final stage
FROM alpine:3.19

WORKDIR /root/

# Install ca-certificates for HTTPS
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

# Copy binary from builder
COPY --from=builder /app/main .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/uploads ./uploads

# Expose port
EXPOSE 8000

# Run the application
CMD ["./main"]
