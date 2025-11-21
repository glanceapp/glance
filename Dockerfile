ARG GO_VERSION=1.24.3
ARG ALPINE_VERSION=3.21

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

WORKDIR /app

RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=bind,source=go.mod,target=go.mod \
  --mount=type=bind,source=go.sum,target=go.sum \
  go mod download -x && go mod verify

COPY . /app

RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache \
  CGO_ENABLED=0 go build .

FROM alpine:${ALPINE_VERSION}

WORKDIR /app
COPY --from=builder /app/glance .

EXPOSE 8080/tcp
ENTRYPOINT ["/app/glance", "--config", "/app/config/glance.yml"]
