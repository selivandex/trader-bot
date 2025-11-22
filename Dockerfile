FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bot cmd/bot/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/bot .

# Create logs directory
RUN mkdir -p /app/logs

# Run as non-root user
RUN addgroup -S trader && adduser -S trader -G trader
RUN chown -R trader:trader /app
USER trader

CMD ["./bot"]

