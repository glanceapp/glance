FROM golang:1.24.4-alpine3.21 AS builder

WORKDIR /app
COPY . /app
RUN CGO_ENABLED=0 go build .

FROM alpine:3.21

RUN addgroup -S app && adduser -S app -G app
USER app
WORKDIR /app

COPY --from=builder --chown=app:app /app/glance .

EXPOSE 8080/tcp
ENTRYPOINT ["/app/glance", "--config", "/app/config/glance.yml"]
