# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN go mod download

# Copy source files, notes, and fonts for embedding
COPY main.go ./main.go
COPY template.html ./template.html
COPY notes ./notes
COPY fonts ./fonts

RUN go build -o server main.go


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
