#!/bin/sh

hostname=monks-co
go_binary=monks.go

echo "Setting up tailscale"
/app/tailscaled --state=/var/lib/tailscale/tailscaled.state --socket=/var/run/tailscale/tailscaled.sock &
/app/tailscale up --authkey=${TAILSCALE_AUTHKEY} --hostname=${hostname}

echo "Running app"
/app/${go_binary}

