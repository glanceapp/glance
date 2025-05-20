FROM golang:1.24.2-alpine3.21 AS builder

WORKDIR /app
COPY . /app
RUN CGO_ENABLED=0 go build .

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/glance .
COPY --from=builder /app/healthcheck.sh .
ADD --chmod=755 healthcheck.sh .

HEALTHCHECK --timeout=10s --start-period=60s --interval=60s \
  CMD /app/healthcheck.sh

EXPOSE 8080/tcp
ENTRYPOINT ["/app/glance", "--config", "/app/config/glance.yml"]
