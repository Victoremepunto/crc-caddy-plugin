#!/usr/bin/env bash

if [[ -f "/etc/caddy/Caddyfile.json" ]]; then
    echo "Running caddy gateway"
    caddy run --config /etc/caddy/Caddyfile.json --adapter json
else
    echo "Running caddy sidecar"
    caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
fi
