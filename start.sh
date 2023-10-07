#!/bin/sh

/app/tailscaled \
  --state=/var/lib/tailscale/tailscaled.state \
  --socket=/var/run/tailscale/tailscaled.sock \
  &

/app/tailscale up \
  --authkey=${TAILSCALE_AUTHKEY} \
  --hostname=monks-co

cd /app
cp data/map.db /data/map.db
bin/run fly

