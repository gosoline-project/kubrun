# Multi-stage build
FROM golang:1.25 AS builder
WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o kubrun .

# Runtime image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app

# Copy binary and config template
COPY --from=builder /src/kubrun .
COPY config.dist.yml .

EXPOSE 8890

ENTRYPOINT ["/app/kubrun"]
