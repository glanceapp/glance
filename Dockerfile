FROM golang:1.26.6-alpine3.23 AS builder

WORKDIR /app
COPY . /app
RUN CGO_ENABLED=0 go build .

FROM alpine:3.23
WORKDIR /app
COPY --from=builder /app/glance .
COPY config/glance.yml /app/config/glance.yml

EXPOSE 8080/tcp
ENTRYPOINT ["/app/glance", "--config", "/app/config/glance.yml"]
