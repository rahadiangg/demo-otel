FROM golang:1.20.4-alpine3.16 AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o order-svc .

##################
FROM alpine:3.16
WORKDIR /app
COPY --from=builder /app/order-svc .

ENTRYPOINT [ "./order-svc" ]

EXPOSE 8080