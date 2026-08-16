FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /patrol-server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 1000 appuser
WORKDIR /app
COPY --from=builder /patrol-server /patrol-server
COPY config.json /app/config.json
USER appuser
EXPOSE 58839
ENTRYPOINT ["/patrol-server"]
