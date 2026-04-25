# Stage 1: Build the application
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Pre-copy/cache go.mod for pre-downloading dependencies
COPY go.mod ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go binary (statically linked for small images)
RUN CGO_ENABLED=0 GOOS=linux go build -o /main .

# Stage 2: Create a minimal production image
FROM alpine:latest  

# Add a non-root user for security
RUN adduser -D myuser
USER myuser

# Copy only the compiled binary from the builder stage
COPY --from=builder /main /main

# Expose application port (example: 8080)
EXPOSE 8080

# Run the binary
CMD ["/main"]
