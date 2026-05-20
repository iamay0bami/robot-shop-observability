FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o webhook-receiver .

FROM scratch
COPY --from=builder /app/webhook-receiver /webhook-receiver
EXPOSE 9091
ENTRYPOINT ["/webhook-receiver"]