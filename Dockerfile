# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

# Final stage
FROM alpine:latest

# Install wget for healthcheck
RUN apk add --no-cache wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server .

# Set default data directory via environment variable
ENV RAFT_DATA_DIR=/tmp/raft-data

# Set non-root user
USER nobody

# Expose ports
EXPOSE 8080 9090 12000

# Use CMD so that docker-compose command overrides it
CMD ["/app/server"]

#ENTRYPOINT ["/bin/sh", "-c", "/app/server \"$@\"", "--"]

