# backend/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go build -o server .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
