FROM golang:1.27.1-alpine3.24 AS builder

ARG BUILD_VERSION=dev

WORKDIR /app
COPY . /app
RUN CGO_ENABLED=0 go build \
    -ldflags "-X github.com/glanceapp/glance/internal/glance.buildVersion=${BUILD_VERSION}" .

FROM alpine:3.24

WORKDIR /app
COPY --from=builder /app/glance .

EXPOSE 8080/tcp
ENTRYPOINT ["/app/glance", "--config", "/app/config/glance.yml"]
