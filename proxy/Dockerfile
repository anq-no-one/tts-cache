FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/tts-cache-proxy ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D app
USER app
WORKDIR /app
COPY --from=build /out/tts-cache-proxy /app/tts-cache-proxy
ENV PORT=8080 CACHE_DIR=/app/data
EXPOSE 8080
VOLUME ["/app/data"]
ENTRYPOINT ["/app/tts-cache-proxy"]
