# Multi-stage build for production
FROM golang:1.22.5-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the Go app with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o powercap main.go

# Final stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create app directory and user
RUN addgroup -g 1001 powercap && \
    adduser -D -u 1001 -G powercap powercap && \
    mkdir -p /app/data && \
    chown -R powercap:powercap /app

# Copy the binary from builder stage
COPY --from=builder /app/powercap /usr/local/bin/powercap

# Create working directory
WORKDIR /app

# Set proper permissions
RUN chmod +x /usr/local/bin/powercap

# Expose any necessary ports (if monitoring endpoints added later)
# EXPOSE 8080

# Health check
HEALTHCHECK --interval=60s --timeout=30s --start-period=10s --retries=3 \
    CMD pgrep -f powercap || exit 1

# Default environment variables (can be overridden by Kubernetes)
ENV DATA_PROVIDER=epex
ENV PROVIDER_URL=https://www.epexspot.com/en/market-results
ENV PROVIDER_PARAMS={"market_area":"FR","auction":"IDA1","modality":"Auction","sub_modality":"Intraday","data_mode":"table"}
ENV MAX_SOURCE=40000000
ENV STABILISATION_TIME=300
ENV RAPL_MIN_POWER=10000000

# Switch to non-root user (will be overridden to root in Kubernetes for RAPL access)
USER powercap

# Run the binary
CMD ["powercap"]
