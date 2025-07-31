# Stage 1: Build the Go application
FROM golang:1.23-alpine AS builder

# Install necessary packages for CGO (required for SQLite)
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled for SQLite support
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main .

# Stage 2: Create the final image
FROM alpine:latest

# Install SQLite for runtime
RUN apk add --no-cache sqlite ca-certificates tzdata

WORKDIR /app

# Create necessary directories with proper permissions
RUN mkdir -p data public src/templates && \
    chmod 755 data public src/templates

# Copy the binary from builder
COPY --from=builder /app/main .

# Copy static files and templates
COPY public/ ./public/
COPY src/templates/ ./src/templates/

# Create a non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -S appuser -u 1001 -G appgroup

# Change ownership of the app directory (especially data directory for SQLite and logs)
RUN chown -R appuser:appgroup /app && \
    chmod -R 755 /app && \
    chmod 775 /app/data

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/ || exit 1

# Run the application
CMD ["./main"] 