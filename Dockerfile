# Build the manager binary
FROM registry.access.redhat.com/ubi8/go-toolset:1.20.12-2 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
USER 0
RUN mkdir .local
RUN go mod download
RUN go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.3.4

COPY caddyplugin.go caddyplugin.go
RUN ~/go/bin/xcaddy build v2.6.4 --with github.com/redhatinsights/crc-caddy-plugin/@v0.0.1=./

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.8-860
WORKDIR /
COPY --from=builder --chown=65534:65534 /workspace/.local /.local
COPY CaddyfileSidecar /etc/Caddyfile
COPY candlepin-ca.pem /cas/ca.pem
COPY --from=builder /workspace/caddy .
COPY runner.sh .
USER 65534:65534

ENTRYPOINT ["/runner.sh"]
