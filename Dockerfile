FROM alpine:latest as tailscale
    ENV TSFILE=tailscale_1.34.1_amd64.tgz
    RUN wget https://pkgs.tailscale.com/stable/${TSFILE} && \
        tar xzf ${TSFILE} --strip-components=1

FROM golang:alpine
    RUN apk update
    RUN apk add ca-certificates iptables ip6tables build-base gcc
    RUN rm -rf /var/cache/apk/*

    WORKDIR /app
    COPY . .

    RUN go build

    COPY --from=tailscale /tailscaled /app/tailscaled
    COPY --from=tailscale /tailscale /app/tailscale
    RUN mkdir -p /var/run/tailscale /var/cache/tailscale /var/lib/tailscale

    CMD ["/app/start.sh"]

