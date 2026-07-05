FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /chargebacks ./cmd

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /chargebacks /app/chargebacks

EXPOSE 8080

CMD ["/app/chargebacks"]