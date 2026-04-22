FROM golang:alpine AS builder
WORKDIR /build
COPY . .
RUN go mod download
RUN go build -o main main.go
FROM alpine
WORKDIR /build
COPY --from=builder /build/main /build/main
COPY --from=builder /build/static /build/static
CMD ["./main"]
