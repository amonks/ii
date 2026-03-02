# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN go mod download

# Do an initial no-notes build to warm up the cache
COPY fonts ./fonts
COPY template.html ./template.html
COPY *.go ./
COPY ./notes/crime-timeline.md ./notes/crime-timeline.md
RUN go build -o server .

# Copy notes and rebuild
COPY notes ./notes
RUN go build -o server .


# Run stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy only the binary from builder (markdown files are embedded in it)
COPY --from=builder /app/server .

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./server"]
