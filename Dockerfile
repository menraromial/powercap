# Etape 1: Build stage
FROM golang:1.22.5 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .

# Etape 2: Run stage
FROM alpine:latest

# Set environment variable for node name (replace with actual node name or set it dynamically)
#ENV NODE_NAME=my-node-name

# Install necessary packages
RUN apk --no-cache add ca-certificates sudo

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/main /usr/local/bin/powercap

# Copy the RAPL files (if applicable)
# Example: COPY ./sys/devices/virtual/powercap/intel-rapl/intel-rapl:0 /sys/devices/virtual/powercap/intel-rapl/intel-rapl:0

# Give necessary permissions for the RAPL files (uncomment if needed)
# RUN chmod -R 755 /sys/devices/virtual/powercap/intel-rapl/

# Command to run the executable
CMD ["powercap"]
