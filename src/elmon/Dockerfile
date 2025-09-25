# Build stage: Use Go image for compilation
FROM golang:1.24 AS builder

WORKDIR /app

# Copy go.mod and go.sum for dependency caching
COPY go.mod  ./
# COPY go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Build the application
# CGO_ENABLED=0 and GOOS=linux to create a static binary without glibc dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/elmon .

# Production stage: Use lightweight image for production
FROM alpine:latest

WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/elmon .

# Expose the port that your Go application listens on
EXPOSE 8080

# Command to run the application
CMD ["./elmon"]
