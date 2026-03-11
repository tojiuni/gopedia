# Gopedia 0.0.1 Phloem — multi-stage Go build
FROM golang:1.24-alpine AS builder
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /phloem ./cmd/phloem

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /phloem /phloem
EXPOSE 50051
ENTRYPOINT ["/phloem"]
