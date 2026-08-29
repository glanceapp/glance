FROM golang:1.27.0-alpine3.24 AS builder

WORKDIR /app
COPY . /app
RUN CGO_ENABLED=0 go build .

FROM alpine:3.24

WORKDIR /app
COPY --from=builder /app/glance .

EXPOSE 8080/tcp
ENTRYPOINT ["/app/glance", "--config", "/app/config/glance.yml"]
