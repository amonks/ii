FROM alpine as tailscale
  WORKDIR /app
  ENV TSFILE=tailscale_1.38.2_amd64.tgz
  RUN wget https://pkgs.tailscale.com/stable/${TSFILE} && \
    tar xzf ${TSFILE} --strip-components=1

FROM golang:alpine as gobuild
  RUN apk update
  RUN apk add build-base gcc
  RUN rm -rf /var/cache/apk/*

  WORKDIR /app
  COPY . .

  RUN go build .

FROM alpine
  RUN apk update && apk add ca-certificates iptables ip6tables && rm -rf /var/cache/apk/*
  WORKDIR /app
  COPY . .
  COPY --from=gobuild /app/monks.co /app/monks.co
  COPY --from=tailscale /app/tailscaled /app/tailscaled
  COPY --from=tailscale /app/tailscale /app/tailscale
  RUN mkdir -p /var/run/tailscale /var/cache/tailscale /var/lib/tailscale

  CMD ["/app/start.sh"]

