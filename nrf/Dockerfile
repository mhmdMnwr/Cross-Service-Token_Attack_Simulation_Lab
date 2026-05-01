FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod ./
COPY *.go ./
RUN go mod tidy && go build -o nrf .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/nrf .
EXPOSE 8000
CMD ["./nrf"]
