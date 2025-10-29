# syntax=docker/dockerfile:1

FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o server .

FROM alpine:3.20
WORKDIR /app
RUN adduser -D appuser
COPY --from=builder /app/server /app/server
COPY --from=builder /app/appointments.db /app/appointments.db
ENV PORT=8080
EXPOSE 8080
USER appuser
CMD ["/app/server"]
