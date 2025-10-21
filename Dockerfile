# Multi-stage build
FROM golang:1.25 AS builder
WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binary for target platform
RUN CGO_ENABLED=0 go build -o kubrun .

# Runtime image
FROM gcr.io/distroless/static-debian12
WORKDIR /app

# Copy binary and config template
COPY --from=builder /src/kubrun .
COPY config.dist.yml .

EXPOSE 8890

ENTRYPOINT ["/app/kubrun"]
