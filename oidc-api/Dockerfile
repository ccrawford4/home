# Builder image
FROM golang:1.25.1-alpine AS builder

WORKDIR /app

# Copy build files and create executable
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Create the slim runtime image
FROM alpine:3.22.1

WORKDIR /root/

COPY --from=builder /app/main .

CMD ["./main"]
