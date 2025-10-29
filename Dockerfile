FROM golang:alpine AS gobuild
  ENV MONKS_ROOT=/app
  RUN apk update
  RUN apk add build-base gcc bash nodejs npm
  RUN rm -rf /var/cache/apk/*

  RUN go install github.com/amonks/run/cmd/run@latest

  WORKDIR /app
  COPY apps apps
  COPY cmd cmd
  COPY credentials credentials
  COPY pkg pkg
  COPY static static
  COPY go.mod go.mod
  COPY go.sum go.sum
  COPY aws/tasks.toml aws/tasks.toml

  COPY tasks.toml tasks.toml
  RUN run build

FROM alpine
  ENV MONKS_ROOT=/app
  ENV MONKS_DATA=/data
  RUN apk update
  RUN apk add ca-certificates iptables ip6tables bash sqlite
  RUN rm -rf /var/cache/apk/*

  WORKDIR /app
  COPY . .

  COPY --from=gobuild /app/bin/ /app/bin/
  COPY --from=gobuild /go/bin/run /app/bin/run

  CMD ["/app/start.sh"]

