FROM alpine:3.19 as base
RUN apk --no-cache add ca-certificates

FROM scratch
ARG TARGETOS
ARG TARGETARCH

COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app
COPY build/glance-$TARGETOS-$TARGETARCH /app/glance

EXPOSE 8080/tcp
ENTRYPOINT ["/app/glance"]
