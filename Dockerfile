ARG GO_VERSION=1.26.5
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/mini-redis .

FROM alpine:3.20
RUN adduser -D -u 10001 app && mkdir /data && chown app /data
USER app
WORKDIR /data
COPY --from=build /out/mini-redis /usr/local/bin/mini-redis
EXPOSE 6380
VOLUME /data
ENTRYPOINT ["mini-redis"]
CMD ["--port", "6380", "--aof", "/data/appendonly.aof"]