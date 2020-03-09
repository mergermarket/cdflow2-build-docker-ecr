FROM golang:alpine AS builder

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download
ADD . .
ENV CGO_ENABLED 0
RUN go test ./internal/app
RUN go build ./cmd/app

FROM alpine:latest

RUN apk add --no-cache docker-cli

COPY --from=builder /build/app /app

ENTRYPOINT ["/app"]
