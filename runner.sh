#! /bin/bash

if [ -f "/etc/caddy/Caddyfile.json" ];
then
    echo "Running caddy gateway"
    /caddy run --config /etc/caddy/Caddyfile.json
else
    echo "Running caddy sidecar"
    /caddy run --config /etc/Caddyfile
fi
