# syntax=docker/dockerfile:1

FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/cuearr ./cmd/cuearr

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates cuetools flac shntool \
    && rm -rf /var/lib/apt/lists/*
RUN useradd --create-home --uid 1000 --home-dir /data --shell /usr/sbin/nologin cuearr
COPY --from=build /out/cuearr /usr/local/bin/cuearr
COPY deploy/docker/cuearr.yaml /etc/cuearr/cuearr.yaml
RUN chown -R cuearr:cuearr /etc/cuearr
USER cuearr
EXPOSE 8787
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/cuearr", "serve", "--config", "/etc/cuearr/cuearr.yaml"]
