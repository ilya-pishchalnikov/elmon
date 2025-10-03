# Build stage: Use Go image for compilation
FROM golang:1.24 AS builder

WORKDIR /app

# Copy go.mod and go.sum for dependency caching
COPY go.mod  ./
COPY go.sum ./
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

# Copy config file
COPY --from=builder app/config.yaml ./
COPY --from=builder app/.env ./

# Copy sql scripts
COPY --from=builder app/sql/script ./sql/script

# Expose the port that your Go application listens on
EXPOSE 8080

# Create and set entrypoint
RUN echo '#!/bin/sh\n\
if [ "$ELMON_DEBUG" = "1" ]; then\n\
    echo "ELMON_DEBUG mode - container started but application not running"\n\
    echo "You can exec into container with: docker exec -it <container> /bin/sh"\n\
    echo "Files in /app:" && ls -la /app/\n\
    tail -f /dev/null\n\
else\n\
    exec ./elmon\n\
fi' > /entrypoint.sh && chmod +x /entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["app/entrypoint.sh"]