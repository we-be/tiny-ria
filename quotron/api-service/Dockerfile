FROM golang:1.21-alpine as builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api-service ./cmd/server

# Use a smaller image for the final container
FROM alpine:latest

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/api-service .

# Expose the API port
EXPOSE 8080

# Set the entry point
ENTRYPOINT ["/app/api-service"]

# Default arguments
CMD ["--port=8080", "--yahoo-host=yahoo-proxy", "--yahoo-port=5000"]