#!/bin/sh

set -x

hostname=monks-co
go_binary=co.monks.monks.co

echo "Setting up tailscale"
/app/tailscaled --state=/var/lib/tailscale/tailscaled.state --socket=/var/run/tailscale/tailscaled.sock &
/app/tailscale up --authkey=${TAILSCALE_AUTHKEY} --hostname=${hostname}

echo "Running app"
/app/${go_binary}

