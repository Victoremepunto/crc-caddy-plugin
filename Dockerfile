# Build the manager binary
FROM registry.access.redhat.com/ubi8/go-toolset:1.17.7 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
USER 0
RUN mkdir .local
RUN go mod download
RUN go install github.com/caddyserver/xcaddy/cmd/xcaddy

COPY caddyplugin.go caddyplugin.go
RUN ~/go/bin/xcaddy build v2.5.1 --with github.com/redhatinsights/caddy-plugin/@v0.0.1=./

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder --chown=65532:65532 /workspace/.local /.local
COPY CaddyfileSidecar /etc/Caddyfile
COPY --from=builder /workspace/caddy .
USER 65532:65532

#ENTRYPOINT ["/caddy", "run"]
ENTRYPOINT ["/caddy", "run", "--config", "/etc/Caddyfile"]
