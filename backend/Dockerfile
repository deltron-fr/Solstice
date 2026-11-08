# Build the API with the Go toolchain version used by this repository.
FROM golang:1.27.3-alpine3.24 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w" \
    -o /out/solstice-api \
    ./cmd/api

# Keep the runtime small, non-root, and able to verify TLS certificates for RPC.
FROM alpine:3.24

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 solstice \
    && adduser -S -D -H -u 10001 -G solstice solstice \
    && mkdir -p /data \
    && chown solstice:solstice /data

COPY --from=builder /out/solstice-api /usr/local/bin/solstice-api

ENV SOLSTICE_HTTP_ADDR=:8080 \
    SOLSTICE_DATABASE_DSN=file:/data/solstice.db \
    SOLSTICE_UPLOAD_DIR=/data/uploads

USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/solstice-api"]
