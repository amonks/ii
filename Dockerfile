FROM golang:alpine
  RUN apk update
  RUN apk add build-base gcc
  RUN rm -rf /var/cache/apk/*

  WORKDIR /app
  COPY . .

  RUN go build .

  CMD ["/app/monks.co"]

