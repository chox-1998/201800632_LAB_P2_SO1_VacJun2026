FROM golang:1.24 AS builder

WORKDIR /app

COPY . .

RUN go mod download

RUN CGO_ENABLED=0 go build -o deployment1 ./cmd

FROM alpine:latest

COPY --from=builder /app/deployment1 .

EXPOSE 9000

CMD ["./deployment1"]
