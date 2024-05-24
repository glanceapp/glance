FROM alpine:3.20

WORKDIR /app

# This binary is an artifact from goreleaser.
COPY glance .
ENTRYPOINT ["/app/glance"]

EXPOSE 8080/tcp
