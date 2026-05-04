FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY main.go ./
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /openid-proxy .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 openid
USER openid
COPY --from=build /openid-proxy /openid-proxy
EXPOSE 8080
ENTRYPOINT ["/openid-proxy"]
