FROM caddy-ubi-builder:e5f28ea8 AS builder

RUN mkdir /opt/app-root/src/crccaddyplugin
WORKDIR /opt/app-root/src/crccaddyplugin
COPY caddyplugin.go .
RUN set -exu ; \
    go mod init crccaddyplugin; \
    go get github.com/caddyserver/caddy/v2@v2.8.4; \
    go mod tidy;

WORKDIR /opt/app-root/src/caddy

RUN bash -x build.sh "github.com/RedHatInsights/crc-caddy-plugin=/opt/app-root/src/crccaddyplugin"

FROM caddy-ubi:e5f28ea8
COPY CaddyfileSidecar /etc/caddy/Caddyfile
COPY candlepin-ca.pem /cas/ca.pem
COPY --from=builder /opt/app-root/src/caddy/caddy .
COPY runner.sh .

ENTRYPOINT ["/runner.sh"]
