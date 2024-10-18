FROM golang:1.22.5-alpine3.20 AS builder

WORKDIR /app
COPY . /app
RUN CGO_ENABLED=0 go build .

FROM alpine:3.20

HEALTHCHECK --timeout=10s --start-period=60s --interval=60s \
  CMD wget --spider -q http://localhost:8080/api/healthz

WORKDIR /app
COPY --from=builder /app/glance .

EXPOSE 8080/tcp
ENTRYPOINT ["/app/glance"]
