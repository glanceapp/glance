FROM golang:1.27.1-alpine3.24.1 AS builder

WORKDIR /app
COPY . /app
RUN CGO_ENABLED=0 go build .

FROM alpine:3.24.1

WORKDIR /app
COPY --from=builder /app/glance .
COPY config/glance.yml /app/config/glance.yml

EXPOSE 8080/tcp
ENTRYPOINT ["/app/glance", "--config", "/app/config/glance.yml"]
