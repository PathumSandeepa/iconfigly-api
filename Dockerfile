FROM golang:1.27.2-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags="-s -w -X github.com/PathumSandeepa/iconfigly-api/internal/version.Version=${VERSION}" \
  -o /out/iconfigly-api ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

ARG VERSION=dev
ENV ICONFIGLY_VERSION=${VERSION}

WORKDIR /app
COPY --from=builder /out/iconfigly-api /usr/local/bin/iconfigly-api

EXPOSE 8080
ENTRYPOINT ["iconfigly-api"]
