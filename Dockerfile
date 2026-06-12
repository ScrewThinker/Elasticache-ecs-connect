# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o server .

# Final stage — minimal image
FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]