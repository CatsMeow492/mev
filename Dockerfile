# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mev-engine ./cmd/mev-engine

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates curl

# Create non-root user
RUN addgroup -g 1001 -S mev && \
    adduser -u 1001 -S mev -G mev

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/mev-engine .
COPY --from=builder /app/configs ./configs

# Change ownership
RUN chown -R mev:mev /app

# Switch to non-root user
USER mev

# Expose port
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the application
CMD ["./mev-engine"]