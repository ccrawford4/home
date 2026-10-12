# Build context: apps/inference-gateway
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /inference-gateway .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 gateway
USER gateway
COPY --from=build /inference-gateway /inference-gateway
EXPOSE 8080
ENTRYPOINT ["/inference-gateway"]
