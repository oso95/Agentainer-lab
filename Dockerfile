FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o agentainer ./cmd/agentainer

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/agentainer .
COPY --from=builder /app/config.yaml .

EXPOSE 8081

CMD ["./agentainer", "server"]