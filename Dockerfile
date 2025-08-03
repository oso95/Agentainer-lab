FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o agentainer ./cmd/agentainer

FROM alpine:3.22

RUN apk update && \
    apk upgrade && \
    apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/agentainer .
COPY --from=builder /app/config.yaml .

EXPOSE 8081

CMD ["./agentainer", "server"]