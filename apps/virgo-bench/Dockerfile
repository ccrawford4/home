# Build context: apps/virgo-bench
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /virgo-bench .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 bench
USER bench
COPY --from=build /virgo-bench /virgo-bench
EXPOSE 8080
ENTRYPOINT ["/virgo-bench"]
